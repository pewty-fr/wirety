package network

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"wirety/internal/domain/network"
	"wirety/pkg/wireguard"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// WebSocketNotifier is an interface for notifying peers about config updates
type WebSocketNotifier interface {
	NotifyNetworkPeers(networkID string)
}

// Service implements the business logic for network management
type Service struct {
	repo       network.Repository
	wsNotifier WebSocketNotifier
}

// SetWebSocketNotifier sets the WebSocket notifier for the service
func (s *Service) SetWebSocketNotifier(notifier WebSocketNotifier) {
	s.wsNotifier = notifier
}

// ResolveAgentToken returns networkID, peer for a given enrollment token.
func (s *Service) ResolveAgentToken(ctx context.Context, token string) (string, *network.Peer, error) {
	networks, err := s.repo.ListNetworks(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("list networks: %w", err)
	}
	for _, n := range networks {
		for _, p := range n.Peers {
			if p.Token == token {
				return n.ID, p, nil
			}
		}
	}
	return "", nil, fmt.Errorf("token not found")
}

// NewService creates a new network service
func NewService(repo network.Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// CreateNetwork creates a new WireGuard network
func (s *Service) CreateNetwork(ctx context.Context, req *network.NetworkCreateRequest) (*network.Network, error) {
	now := time.Now()

	net := &network.Network{
		ID:        uuid.New().String(),
		Name:      req.Name,
		CIDR:      req.CIDR,
		Domain:    req.Domain,
		Peers:     make(map[string]*network.Peer),
		ACL:       &network.ACL{Enabled: false, BlockedPeers: make(map[string]bool)},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.CreateNetwork(ctx, net); err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	// Ensure IPAM root prefix exists for this network CIDR so future IP allocations succeed.
	if _, err := s.repo.EnsureRootPrefix(ctx, net.CIDR); err != nil {
		return nil, fmt.Errorf("failed to ensure root prefix: %w", err)
	}

	return net, nil
}

// GetNetwork retrieves a network by ID
func (s *Service) GetNetwork(ctx context.Context, networkID string) (*network.Network, error) {
	return s.repo.GetNetwork(ctx, networkID)
}

// ListNetworks retrieves all networks
func (s *Service) ListNetworks(ctx context.Context) ([]*network.Network, error) {
	return s.repo.ListNetworks(ctx)
}

// UpdateNetwork updates a network's configuration
func (s *Service) UpdateNetwork(ctx context.Context, networkID string, req *network.NetworkUpdateRequest) (*network.Network, error) {
	net, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	oldCIDR := net.CIDR
	cidrChanged := false

	if req.Name != "" {
		net.Name = req.Name
	}
	if req.Domain != "" {
		net.Domain = req.Domain
	}
	if req.CIDR != "" && req.CIDR != oldCIDR {
		net.CIDR = req.CIDR
		cidrChanged = true
	}
	net.UpdatedAt = time.Now()

	// If CIDR changed, reallocate all peer IPs
	if cidrChanged {
		// Get all peers to check for static peers
		peers, err := s.repo.ListPeers(ctx, networkID)
		if err != nil {
			return nil, fmt.Errorf("failed to list peers: %w", err)
		}

		// Check if any regular peers are using static config (not using agent)
		for _, peer := range peers {
			if !peer.IsJump && !peer.UseAgent {
				return nil, fmt.Errorf("cannot change CIDR: network contains static regular peer '%s' which would require manual reconfiguration", peer.Name)
			}
		}

		// Ensure new root prefix exists
		if _, err := s.repo.EnsureRootPrefix(ctx, net.CIDR); err != nil {
			return nil, fmt.Errorf("failed to ensure new root prefix: %w", err)
		}

		// Release old IPs and allocate new ones
		for _, peer := range peers {
			// Release old IP from old CIDR
			if err := s.repo.ReleaseIP(ctx, oldCIDR, peer.Address); err != nil {
				// Log but don't fail - old CIDR may not exist in IPAM
				fmt.Printf("Warning: failed to release old IP %s from %s: %v\n", peer.Address, oldCIDR, err)
			}

			// Allocate new IP from new CIDR
			newAddress, err := s.repo.AcquireIP(ctx, net.CIDR)
			if err != nil {
				return nil, fmt.Errorf("failed to allocate new IP for peer %s: %w", peer.ID, err)
			}

			peer.Address = newAddress
			peer.UpdatedAt = time.Now()

			if err := s.repo.UpdatePeer(ctx, networkID, peer); err != nil {
				return nil, fmt.Errorf("failed to update peer %s with new IP: %w", peer.ID, err)
			}
		}
	}

	if err := s.repo.UpdateNetwork(ctx, net); err != nil {
		return nil, fmt.Errorf("failed to update network: %w", err)
	}

	return net, nil
}

// AddPeer adds a new peer to the network
func (s *Service) AddPeer(ctx context.Context, networkID string, req *network.PeerCreateRequest, ownerID string) (*network.Peer, error) {
	net, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	// Allocate IP address for the peer using IPAM repository (hexagonal compliant)
	address, err := s.repo.AcquireIP(ctx, net.CIDR)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire IP from IPAM: %w", err)
	}

	// Generate WireGuard keys for the peer
	privateKey, publicKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	now := time.Now()
	peer := &network.Peer{
		ID:                   uuid.New().String(),
		Name:                 req.Name,
		PublicKey:            publicKey,
		PrivateKey:           privateKey,
		Address:              address,
		Endpoint:             req.Endpoint,
		ListenPort:           req.ListenPort,
		IsJump:               req.IsJump,
		JumpNatInterface:     req.JumpNatInterface,
		IsIsolated:           req.IsIsolated,
		FullEncapsulation:    req.FullEncapsulation,
		UseAgent:             req.UseAgent,             // Track if peer uses agent or static config
		AdditionalAllowedIPs: req.AdditionalAllowedIPs, // default full network access (used for jump peers)
		OwnerID:              ownerID,                  // Set the owner of the peer
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	// Generate enrollment token
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	peer.Token = base64.RawURLEncoding.EncodeToString(raw)

	// Default listen port for jump peers if not provided
	if peer.IsJump && peer.ListenPort == 0 {
		peer.ListenPort = 51820
	}

	// Jump peers always use agent
	if peer.IsJump {
		peer.UseAgent = true
	}

	if err := s.repo.CreatePeer(ctx, networkID, peer); err != nil {
		return nil, fmt.Errorf("failed to create peer: %w", err)
	}

	// Create preshared key connections with all existing peers
	existingPeers, err := s.repo.ListPeers(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing peers: %w", err)
	}

	for _, existingPeer := range existingPeers {
		if existingPeer.ID == peer.ID {
			continue // skip self
		}

		presharedKey, err := wireguard.GeneratePresharedKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate preshared key: %w", err)
		}

		conn := &network.PeerConnection{
			Peer1ID:      peer.ID,
			Peer2ID:      existingPeer.ID,
			PresharedKey: presharedKey,
			CreatedAt:    now,
		}

		if err := s.repo.CreateConnection(ctx, networkID, conn); err != nil {
			return nil, fmt.Errorf("failed to create connection: %w", err)
		}
	}

	return peer, nil
}

// GetPeer retrieves a peer by ID
func (s *Service) GetPeer(ctx context.Context, networkID, peerID string) (*network.Peer, error) {
	return s.repo.GetPeer(ctx, networkID, peerID)
}

// ListPeers retrieves all peers in a network
func (s *Service) ListPeers(ctx context.Context, networkID string) ([]*network.Peer, error) {
	return s.repo.ListPeers(ctx, networkID)
}

// UpdatePeer updates a peer's configuration
func (s *Service) UpdatePeer(ctx context.Context, networkID, peerID string, req *network.PeerUpdateRequest) (*network.Peer, error) {
	peer, err := s.repo.GetPeer(ctx, networkID, peerID)
	if err != nil {
		return nil, fmt.Errorf("peer not found: %w", err)
	}

	if req.ListenPort != 0 {
		peer.ListenPort = req.ListenPort
	}
	if req.Name != "" {
		peer.Name = req.Name
	}
	if req.Endpoint != "" {
		peer.Endpoint = req.Endpoint
	}
	peer.IsIsolated = req.IsIsolated
	peer.FullEncapsulation = req.FullEncapsulation
	if req.AdditionalAllowedIPs != nil {
		peer.AdditionalAllowedIPs = req.AdditionalAllowedIPs
	}
	peer.UpdatedAt = time.Now()
	// Preserve token (do not allow overwrite via update)

	if err := s.repo.UpdatePeer(ctx, networkID, peer); err != nil {
		return nil, fmt.Errorf("failed to update peer: %w", err)
	}

	return peer, nil
}

// DeletePeer removes a peer from the network
func (s *Service) DeletePeer(ctx context.Context, networkID, peerID string) error {
	// Retrieve network and peer to release IP before deletion
	net, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("network not found: %w", err)
	}
	peer, err := s.repo.GetPeer(ctx, networkID, peerID)
	if err != nil {
		return fmt.Errorf("peer not found: %w", err)
	}

	// Prevent deletion of last jump server
	if peer.IsJump {
		jumpCount := 0
		allPeers, err := s.repo.ListPeers(ctx, networkID)
		if err != nil {
			return fmt.Errorf("failed to list peers: %w", err)
		}
		for _, p := range allPeers {
			if p.IsJump {
				jumpCount++
			}
		}
		if jumpCount <= 1 {
			return fmt.Errorf("cannot delete last jump server; network must have at least one jump server")
		}
	}

	// Delete all connections involving this peer
	allPeers, err := s.repo.ListPeers(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to list peers: %w", err)
	}

	for _, otherPeer := range allPeers {
		if otherPeer.ID == peerID {
			continue
		}
		// Ignore errors if connection doesn't exist
		_ = s.repo.DeleteConnection(ctx, networkID, peerID, otherPeer.ID)
	}

	// Release IP back to IPAM
	if err := s.repo.ReleaseIP(ctx, net.CIDR, peer.Address); err != nil {
		return fmt.Errorf("failed to release IP: %w", err)
	}

	return s.repo.DeletePeer(ctx, networkID, peerID)
}

// GeneratePeerConfig generates WireGuard configuration for a specific peer
func (s *Service) GeneratePeerConfig(ctx context.Context, networkID, peerID string) (string, error) {
	net, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("network not found: %w", err)
	}

	peer, exists := net.GetPeer(peerID)
	if !exists {
		return "", fmt.Errorf("peer not found")
	}

	allowedPeers := net.GetAllowedPeersFor(peerID)

	// Build a map of preshared keys for allowed peers
	presharedKeys := make(map[string]string)
	for _, allowedPeer := range allowedPeers {
		conn, err := s.repo.GetConnection(ctx, networkID, peerID, allowedPeer.ID)
		if err == nil && conn != nil {
			presharedKeys[allowedPeer.ID] = conn.PresharedKey
		}
	}

	config := wireguard.GenerateConfig(peer, allowedPeers, net, presharedKeys)

	return config, nil
}

// PeerDNSConfig is sent to jump agents for DNS server startup
// Peer struct reused from domain/network/peer.go

// DNSPeer provides minimal peer info for jump DNS distribution
type DNSPeer struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type PeerDNSConfig struct {
	IP     string    `json:"ip"`
	Domain string    `json:"domain"`
	Peers  []DNSPeer `json:"peers"`
}

// sanitizeDNSLabel converts a peer name into a DNS-safe lowercase label.
func sanitizeDNSLabel(s string) string {
	// Simple sanitation: lowercase and replace invalid chars with '-'
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r+'a'-'A')
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-':
			out = append(out, r)
		case r == '_' || r == '.' || r == ' ':
			out = append(out, '-')
		default:
			out = append(out, '-')
		}
	}
	if len(out) == 0 {
		return "peer"
	}
	return string(out)
}

// JumpPolicy contains isolation & ACL data for jump agent filtering
type JumpPolicy struct {
	IP    string `json:"ip"`
	Peers []struct {
		ID       string `json:"id"`
		IP       string `json:"ip"`
		Isolated bool   `json:"isolated"`
	} `json:"peers"`
	ACLBlocked []string `json:"acl_blocked"`
}

// GeneratePeerConfigWithDNS returns WireGuard config, DNS config & jump policy (for jump peers)
func (s *Service) GeneratePeerConfigWithDNS(ctx context.Context, networkID, peerID string) (string, *PeerDNSConfig, *JumpPolicy, error) {
	net, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("network not found: %w", err)
	}
	peer, exists := net.GetPeer(peerID)
	if !exists {
		return "", nil, nil, fmt.Errorf("peer not found")
	}
	allowedPeers := net.GetAllowedPeersFor(peerID)
	presharedKeys := make(map[string]string)
	for _, allowedPeer := range allowedPeers {
		conn, err := s.repo.GetConnection(ctx, networkID, peerID, allowedPeer.ID)
		if err == nil && conn != nil {
			presharedKeys[allowedPeer.ID] = conn.PresharedKey
		}
	}
	config := wireguard.GenerateConfig(peer, allowedPeers, net, presharedKeys)
	var dnsConfig *PeerDNSConfig
	var policy *JumpPolicy
	if peer.IsJump {
		peerList := make([]DNSPeer, 0, len(net.Peers))
		policy = &JumpPolicy{
			IP: peer.Address,
		}
		for _, p := range net.Peers {
			peerList = append(peerList, DNSPeer{Name: sanitizeDNSLabel(p.Name), IP: p.Address})
			policy.Peers = append(policy.Peers, struct {
				ID       string `json:"id"`
				IP       string `json:"ip"`
				Isolated bool   `json:"isolated"`
			}{ID: p.ID, IP: p.Address, Isolated: p.IsIsolated})
		}
		dnsConfig = &PeerDNSConfig{IP: peer.Address, Domain: net.Domain, Peers: peerList}
		if net.ACL != nil && net.ACL.Enabled {
			for blockedID := range net.ACL.BlockedPeers {
				policy.ACLBlocked = append(policy.ACLBlocked, blockedID)
			}
		}
	}
	return config, dnsConfig, policy, nil
}

// UpdateACL updates the ACL configuration for a network
func (s *Service) UpdateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	_, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("network not found: %w", err)
	}
	return s.repo.UpdateACL(ctx, networkID, acl)
}

// GetACL retrieves the ACL configuration for a network
func (s *Service) GetACL(ctx context.Context, networkID string) (*network.ACL, error) {
	return s.repo.GetACL(ctx, networkID)
}

// DeleteNetwork deletes a network
func (s *Service) DeleteNetwork(ctx context.Context, networkID string) error {
	return s.repo.DeleteNetwork(ctx, networkID)
}

// blockPeerInACL adds a peer to the ACL blocked list to prevent all communication
func (s *Service) blockPeerInACL(ctx context.Context, networkID, peerID string, reason string) error {
	// Get current ACL
	acl, err := s.repo.GetACL(ctx, networkID)
	if err != nil {
		// Create new ACL if it doesn't exist
		acl = &network.ACL{
			ID:           uuid.New().String(),
			Name:         "Default ACL",
			Enabled:      true,
			BlockedPeers: make(map[string]bool),
			Rules:        []network.ACLRule{},
		}
	}

	// Enable ACL if not already enabled
	if !acl.Enabled {
		acl.Enabled = true
	}

	// Add peer to blocked list
	if acl.BlockedPeers == nil {
		acl.BlockedPeers = make(map[string]bool)
	}
	acl.BlockedPeers[peerID] = true

	// Update ACL in repository
	if err := s.repo.UpdateACL(ctx, networkID, acl); err != nil {
		return fmt.Errorf("failed to update ACL: %w", err)
	}

	// Notify all connected peers about the ACL change
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	peer, _ := s.repo.GetPeer(ctx, networkID, peerID)
	peerName := peerID
	if peer != nil {
		peerName = peer.Name
	}

	fmt.Printf("SECURITY: Blocked peer %s in ACL due to %s\n", peerName, reason)
	return nil
}

// ProcessAgentHeartbeat processes a heartbeat message from an agent
func (s *Service) ProcessAgentHeartbeat(ctx context.Context, networkID, peerID string, heartbeat *network.AgentHeartbeat) error {
	// Generate or retrieve session ID (based on hostname + peer ID for now)
	// sessionID := fmt.Sprintf("%s-%s", peerID, heartbeat.Hostname)

	// Check for existing sessions for this peer
	existingSessions, err := s.repo.GetActiveSessionsForPeer(ctx, networkID, peerID)
	if err != nil {
		return fmt.Errorf("failed to get existing sessions: %w", err)
	}

	// Check for session conflicts (multiple agents with different hostnames)
	now := time.Now()
	activeSessionThreshold := now.Add(-network.SessionConflictThreshold)

	var currentSession *network.AgentSession
	for _, session := range existingSessions {
		// Consider sessions active if seen within threshold
		if session.LastSeen.After(activeSessionThreshold) {
			if session.Hostname != heartbeat.Hostname {
				peer, err := s.repo.GetPeer(ctx, networkID, peerID)
				if err == nil {
					net, err := s.repo.GetNetwork(ctx, networkID)
					if err == nil {

						resolved := false
						incidents, err := s.repo.ListSecurityIncidentsByNetwork(ctx, networkID, &resolved)
						if err == nil {
							exists := false
							for _, incident := range incidents {
								if incident.IncidentType == network.IncidentTypeSessionConflict && incident.PeerID == peerID {
									// nothing to do, incident already exists
									exists = true
									break
								}
							}
							if exists {
								continue
							}
							incident := &network.SecurityIncident{
								ID:           fmt.Sprintf("incident-conflict-%s-%d", peerID, now.Unix()),
								PeerID:       peerID,
								PeerName:     peer.Name,
								NetworkID:    networkID,
								NetworkName:  net.Name,
								IncidentType: network.IncidentTypeSessionConflict,
								DetectedAt:   now,
								PublicKey:    peer.PublicKey,
								Endpoints:    []string{session.ReportedEndpoint},
								Details:      fmt.Sprintf("Session conflict: peer %s has multiple active agents (hostnames: %s, %s)", peer.Name, session.Hostname, heartbeat.Hostname),
								Resolved:     false,
							}
							if err := s.repo.CreateSecurityIncident(ctx, incident); err != nil {
								fmt.Printf("failed to create security incident for session conflict: %v\n", err)
							}

							// Block the peer in ACL to prevent all communication
							if err := s.blockPeerInACL(ctx, networkID, peerID, "session conflict detection"); err != nil {
								fmt.Printf("failed to block peer in ACL: %v\n", err)
							}
						}
					}
				}
			} else {
				currentSession = session
			}
		}
	} // Create or update the session
	session := &network.AgentSession{
		PeerID:          peerID,
		Hostname:        heartbeat.Hostname,
		SystemUptime:    heartbeat.SystemUptime,
		WireGuardUptime: heartbeat.WireGuardUptime,
		LastSeen:        now,
		SessionID:       uuid.NewString(),
	}

	// Get all session from peersendpoints
	peerEndpoints := map[string]string{}
	peers, err := s.repo.ListPeers(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to list peers: %w", err)
	}

	for _, peer := range peers {
		if endpoint, ok := heartbeat.PeerEndpoints[peer.PublicKey]; ok {
			peerEndpoints[peer.ID] = endpoint
		}
	}

	if currentSession == nil {
		session.FirstSeen = now
	} else {
		session.FirstSeen = currentSession.FirstSeen
		session.ReportedEndpoint = currentSession.ReportedEndpoint
		session.SessionID = currentSession.SessionID
	}

	for id, endpoint := range peerEndpoints {
		existingSess, err := s.repo.GetActiveSessionsForPeer(ctx, networkID, id)
		if err != nil {
			return fmt.Errorf("failed to get existing sessions: %w", err)
		}
		now := time.Now()
		activeSessionThreshold := now.Add(-network.SessionConflictThreshold)

		var currentSess *network.AgentSession
		for _, sess := range existingSess {
			// Consider sessions active if seen within threshold
			if sess.LastSeen.After(activeSessionThreshold) {
				currentSess = sess
				break
			}
		}
		if currentSess == nil {
			continue
		}
		if currentSess.ReportedEndpoint == "" {
			currentSess.ReportedEndpoint = endpoint
			_ = s.repo.CreateOrUpdateSession(ctx, networkID, currentSess)
			continue
		}
		if currentSess.ReportedEndpoint == endpoint {
			continue
		}
		change := &network.EndpointChange{
			PeerID:      currentSess.PeerID,
			OldEndpoint: currentSess.ReportedEndpoint,
			NewEndpoint: endpoint,
			ChangedAt:   now,
			Source:      "agent",
		}
		if err := s.repo.RecordEndpointChange(ctx, networkID, change); err != nil {
			// Log but don't fail on endpoint change recording error
			fmt.Printf("failed to record endpoint change: %v\n", err)
		}
	}

	err = s.detectAndHandleSharedConfigs(ctx, networkID)
	if err != nil {
		log.Error().Err(err).Msg("failed to detect and handle shared configs")
	}
	err = s.detectAndHandleSuspicousActivity(ctx, networkID)
	if err != nil {
		log.Error().Err(err).Msg("failed to detect and handle suspicious activity")
	}

	return s.repo.CreateOrUpdateSession(ctx, networkID, session)
}

// GetPeerSessionStatus returns the security status of a peer's sessions
func (s *Service) GetPeerSessionStatus(ctx context.Context, networkID, peerID string) (*network.PeerSessionStatus, error) {
	sessions, err := s.repo.GetActiveSessionsForPeer(ctx, networkID, peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions: %w", err)
	}

	now := time.Now()
	activeThreshold := now.Add(-network.SessionConflictThreshold)

	status := &network.PeerSessionStatus{
		PeerID:                peerID,
		HasActiveAgent:        false,
		ConflictingSessions:   []network.AgentSession{},
		RecentEndpointChanges: []network.EndpointChange{},
		SuspiciousActivity:    false,
		LastChecked:           now,
	}

	// Find active sessions
	var activeSessions []*network.AgentSession
	for _, session := range sessions {
		if session.LastSeen.After(activeThreshold) {
			activeSessions = append(activeSessions, session)
		}
	}

	if len(activeSessions) > 0 {
		status.HasActiveAgent = true
		status.CurrentSession = activeSessions[0]
	}

	// Check for conflicts (multiple active sessions with different hostnames)
	if len(activeSessions) > 1 {
		for _, session := range activeSessions {
			status.ConflictingSessions = append(status.ConflictingSessions, *session)
		}
	}

	// Get recent endpoint changes (last 24 hours)
	since := now.Add(-24 * time.Hour)
	changes, err := s.repo.GetEndpointChanges(ctx, networkID, peerID, since)
	if err == nil && changes != nil {
		for _, change := range changes {
			status.RecentEndpointChanges = append(status.RecentEndpointChanges, *change)
		}

		// Check for suspicious activity (too many endpoint changes)
		if len(changes) >= network.MaxEndpointChangesPerDay {
			status.SuspiciousActivity = true
		}

		// Check for rapid endpoint changes
		for i := 1; i < len(changes); i++ {
			timeDiff := changes[i].ChangedAt.Sub(changes[i-1].ChangedAt)
			if timeDiff < network.EndpointChangeThreshold {
				status.SuspiciousActivity = true
				break
			}
		}
	}

	return status, nil
}

// ListSessions returns all sessions in a network
func (s *Service) ListSessions(ctx context.Context, networkID string) ([]*network.AgentSession, error) {
	return s.repo.ListSessions(ctx, networkID)
}

// Security incident operations

// ListSecurityIncidents lists all security incidents
func (s *Service) ListSecurityIncidents(ctx context.Context, resolved *bool) ([]*network.SecurityIncident, error) {
	return s.repo.ListSecurityIncidents(ctx, resolved)
}

// ListSecurityIncidentsByNetwork lists security incidents for a specific network
func (s *Service) ListSecurityIncidentsByNetwork(ctx context.Context, networkID string, resolved *bool) ([]*network.SecurityIncident, error) {
	return s.repo.ListSecurityIncidentsByNetwork(ctx, networkID, resolved)
}

// GetSecurityIncident retrieves a security incident by ID
func (s *Service) GetSecurityIncident(ctx context.Context, incidentID string) (*network.SecurityIncident, error) {
	return s.repo.GetSecurityIncident(ctx, incidentID)
}

// ResolveSecurityIncident marks a security incident as resolved
func (s *Service) ResolveSecurityIncident(ctx context.Context, incidentID, resolvedBy string) error {
	return s.repo.ResolveSecurityIncident(ctx, incidentID, resolvedBy)
}

// ReconnectPeer removes a peer from the ACL blocked list to re-enable communication
func (s *Service) ReconnectPeer(ctx context.Context, networkID, peerID string) error {
	peer, err := s.repo.GetPeer(ctx, networkID, peerID)
	if err != nil {
		return fmt.Errorf("peer not found: %w", err)
	}

	// Get current ACL
	acl, err := s.repo.GetACL(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to get ACL: %w", err)
	}

	// Check if peer is blocked
	if acl.BlockedPeers == nil || !acl.BlockedPeers[peerID] {
		return fmt.Errorf("peer %s is not blocked in ACL", peer.Name)
	}

	// Remove peer from blocked list
	delete(acl.BlockedPeers, peerID)

	// Update ACL in repository
	if err := s.repo.UpdateACL(ctx, networkID, acl); err != nil {
		return fmt.Errorf("failed to update ACL: %w", err)
	}

	// Notify all connected peers about the ACL change
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	fmt.Printf("SECURITY: Unblocked peer %s in ACL - peer reconnected\n", peer.Name)
	return nil
}

// trackPeerEndpointChanges tracks endpoint changes for peers based on their public keys
// as observed by the jump server from WireGuard handshakes
// func (s *Service) trackPeerEndpointChanges(ctx context.Context, networkID, jumpServerID string, peerEndpoints map[string]string) error {
// 	// Get the network to access peer information
// 	net, err := s.repo.GetNetwork(ctx, networkID)
// 	if err != nil {
// 		return fmt.Errorf("failed to get network: %w", err)
// 	}

// 	now := time.Now()

// 	// For each public key -> endpoint mapping seen by the jump server
// 	for publicKey, newEndpoint := range peerEndpoints {
// 		// Find which peer has this public key
// 		var targetPeer *network.Peer
// 		for _, peer := range net.Peers {
// 			if peer.PublicKey == publicKey {
// 				targetPeer = peer
// 				break
// 			}
// 		}

// 		if targetPeer == nil {
// 			// Public key not found in network peers - might be the jump server's own peers
// 			continue
// 		}

// 		// Don't track endpoint changes for the jump server itself
// 		if targetPeer.ID == jumpServerID {
// 			continue
// 		}

// 		// Get the last recorded endpoint for this peer
// 		recentChanges, err := s.repo.GetEndpointChanges(ctx, networkID, targetPeer.ID, now.Add(-24*time.Hour))
// 		if err != nil {
// 			// Continue even if we can't get history
// 			continue
// 		}

// 		var lastEndpoint string
// 		if len(recentChanges) > 0 {
// 			// Get the most recent endpoint
// 			lastEndpoint = recentChanges[len(recentChanges)-1].NewEndpoint
// 		}

// 		// If endpoint changed, record it
// 		if lastEndpoint != "" && lastEndpoint != newEndpoint {
// 			change := &network.EndpointChange{
// 				PeerID:      targetPeer.ID,
// 				OldEndpoint: lastEndpoint,
// 				NewEndpoint: newEndpoint,
// 				ChangedAt:   now,
// 				Source:      "wireguard", // From WireGuard handshakes, not agent-reported
// 			}

// 			if err := s.repo.RecordEndpointChange(ctx, networkID, change); err != nil {
// 				fmt.Printf("failed to record endpoint change for peer %s: %v\n", targetPeer.ID, err)
// 			} else {
// 				fmt.Printf("Recorded endpoint change for peer %s (%s): %s -> %s\n",
// 					targetPeer.Name, targetPeer.ID, lastEndpoint, newEndpoint)
// 			}
// 		} else if lastEndpoint == "" {
// 			// First time seeing this peer's endpoint, record it as initial endpoint
// 			change := &network.EndpointChange{
// 				PeerID:      targetPeer.ID,
// 				OldEndpoint: "",
// 				NewEndpoint: newEndpoint,
// 				ChangedAt:   now,
// 				Source:      "wireguard",
// 			}

// 			if err := s.repo.RecordEndpointChange(ctx, networkID, change); err != nil {
// 				fmt.Printf("failed to record initial endpoint for peer %s: %v\n", targetPeer.ID, err)
// 			}
// 		}
// 	}

// 	return nil
// }

// detectAndHandleSharedConfigs detects if the same peer's endpoint changes multiple times
// within a short time period (5 minutes), indicating shared WireGuard configurations
func (s *Service) detectAndHandleSharedConfigs(ctx context.Context, networkID string) error {
	// Get all peers in the network
	peers, err := s.repo.ListPeers(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to list peers: %w", err)
	}

	now := time.Now()
	checkWindow := now.Add(-network.EndpointChangeThreshold) // 30 minutes ago

	sharedConfigPeers := make(map[string]bool)
	peerEndpoints := make(map[string][]string) // peerID -> list of endpoints

	// Check each peer for rapid endpoint changes
	for _, peer := range peers {
		// Get recent endpoint changes for this peer (last 5 minutes)
		changes, err := s.repo.GetEndpointChanges(ctx, networkID, peer.ID, checkWindow)
		if err != nil {
			// Continue checking other peers even if one fails
			continue
		}

		if len(changes) < 2 {
			// Need at least 2 changes to detect shared config
			continue
		}

		// Collect unique endpoints seen in this window
		endpointSet := make(map[string]bool)
		var endpoints []string
		for _, change := range changes {
			if change.NewEndpoint != "" && !endpointSet[change.NewEndpoint] {
				endpointSet[change.NewEndpoint] = true
				endpoints = append(endpoints, change.NewEndpoint)
			}
		}

		// If we see 2+ different endpoints within 5 minutes, it's a shared config
		if len(endpoints) >= 2 {
			fmt.Printf("WARNING: Shared config detected! Peer %s (%s) seen at %d different endpoints within 30 minutes:\n",
				peer.Name, peer.ID, len(endpoints))
			for _, ep := range endpoints {
				fmt.Printf("  - %s\n", ep)
			}

			sharedConfigPeers[peer.ID] = true
			peerEndpoints[peer.ID] = endpoints

			// Create security incident for this peer
			net, err := s.repo.GetNetwork(ctx, networkID)
			if err == nil {
				incident := &network.SecurityIncident{
					ID:           fmt.Sprintf("incident-%s-%d", peer.ID, now.Unix()),
					PeerID:       peer.ID,
					PeerName:     peer.Name,
					NetworkID:    networkID,
					NetworkName:  net.Name,
					IncidentType: network.IncidentTypeSharedConfig,
					DetectedAt:   now,
					PublicKey:    peer.PublicKey,
					Endpoints:    endpoints,
					Details:      fmt.Sprintf("Peer %s detected at %d different endpoints within 30 minutes: %v", peer.Name, len(endpoints), endpoints),
					Resolved:     false,
				}

				if err := s.repo.CreateSecurityIncident(ctx, incident); err != nil {
					fmt.Printf("failed to create security incident: %v\n", err)
				}
			}
		}
	}

	// Block affected peers using ACL to prevent all communication
	if len(sharedConfigPeers) > 0 {
		for affectedPeerID := range sharedConfigPeers {
			if err := s.blockPeerInACL(ctx, networkID, affectedPeerID, "shared configuration detection"); err != nil {
				fmt.Printf("failed to block peer %s in ACL: %v\n", affectedPeerID, err)
			}
		}
	}

	return nil
}

func (s *Service) detectAndHandleSuspicousActivity(ctx context.Context, networkID string) error {
	// Get all peers in the network
	peers, err := s.repo.ListPeers(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to list peers: %w", err)
	}

	now := time.Now()
	checkWindow := now.Add(-time.Hour * 24) // 24 hours ago

	suspiciousActivityPeers := make(map[string]bool)

	// Check each peer for rapid endpoint changes
	for _, peer := range peers {
		// Get recent endpoint changes for this peer (last 5 minutes)
		changes, err := s.repo.GetEndpointChanges(ctx, networkID, peer.ID, checkWindow)
		if err != nil {
			// Continue checking other peers even if one fails
			continue
		}

		if len(changes) >= network.MaxEndpointChangesPerDay {
			// Collect unique endpoints seen in this window
			endpointSet := make(map[string]bool)
			var endpoints []string
			for _, change := range changes {
				if change.NewEndpoint != "" && !endpointSet[change.NewEndpoint] {
					endpointSet[change.NewEndpoint] = true
					endpoints = append(endpoints, change.NewEndpoint)
				}
			}

			suspiciousActivityPeers[peer.ID] = true
			net, err := s.repo.GetNetwork(ctx, networkID)
			if err == nil {
				incident := &network.SecurityIncident{
					ID:           fmt.Sprintf("incident-%s-%d", peer.ID, now.Unix()),
					PeerID:       peer.ID,
					PeerName:     peer.Name,
					NetworkID:    networkID,
					NetworkName:  net.Name,
					IncidentType: network.IncidentTypeSuspiciousActivity,
					DetectedAt:   now,
					PublicKey:    peer.PublicKey,
					Endpoints:    endpoints,
					Details:      fmt.Sprintf("Peer %s has %d endpoint changes within 24 hours", peer.Name, len(changes)),
					Resolved:     false,
				}

				if err := s.repo.CreateSecurityIncident(ctx, incident); err != nil {
					fmt.Printf("failed to create security incident: %v\n", err)
				}
			}
		}
	}

	// Block affected peers using ACL to prevent all communication
	if len(suspiciousActivityPeers) > 0 {
		for affectedPeerID := range suspiciousActivityPeers {
			if err := s.blockPeerInACL(ctx, networkID, affectedPeerID, "suspicious activity detection"); err != nil {
				fmt.Printf("failed to block peer %s in ACL: %v\n", affectedPeerID, err)
			}
		}
	}

	return nil
}

// OLD IMPLEMENTATION (kept for debugging)
// detectAndHandleSharedConfigs detects if the same public key appears with different endpoints
// within a short time period, indicating shared WireGuard configurations
// func (s *Service) detectAndHandleSharedConfigs(ctx context.Context, networkID, reportingPeerID string) error {
// 	if len(peerEndpoints) == 0 {
// 		return nil
// 	}

// 	// First, track endpoint changes for each peer based on their public key
// 	if err := s.trackPeerEndpointChanges(ctx, networkID, reportingPeerID, peerEndpoints); err != nil {
// 		fmt.Printf("failed to track peer endpoint changes: %v\n", err)
// 	}

// 	// Get all active sessions in this network
// 	allSessions, err := s.repo.ListSessions(ctx, networkID)
// 	if err != nil {
// 		return fmt.Errorf("failed to list sessions: %w", err)
// 	}

// 	now := time.Now()
// 	activeThreshold := now.Add(-network.SessionConflictThreshold) // 5 minutes

// 	// Build a map of publicKey -> {endpoint, peerID, lastSeen} from all active sessions
// 	type endpointInfo struct {
// 		endpoint string
// 		peerID   string
// 		lastSeen time.Time
// 	}
// 	publicKeyToInfo := make(map[string][]endpointInfo)

// 	// Collect from all active sessions
// 	for _, session := range allSessions {
// 		// Only consider active sessions (within last 5 minutes)
// 		if !session.LastSeen.After(activeThreshold) {
// 			continue
// 		}

// 		for publicKey, endpoint := range session.PeerEndpoints {
// 			publicKeyToInfo[publicKey] = append(publicKeyToInfo[publicKey], endpointInfo{
// 				endpoint: endpoint,
// 				peerID:   session.PeerID,
// 				lastSeen: session.LastSeen,
// 			})
// 		}
// 	}

// 	// Add current reporting peer's endpoints
// 	for publicKey, endpoint := range peerEndpoints {
// 		publicKeyToInfo[publicKey] = append(publicKeyToInfo[publicKey], endpointInfo{
// 			endpoint: endpoint,
// 			peerID:   reportingPeerID,
// 			lastSeen: now,
// 		})
// 	}

// 	// Detect shared configs: same public key with different endpoints in active window
// 	sharedConfigPeers := make(map[string]bool)
// 	var sharedConfigDetails []string

// 	for publicKey, infos := range publicKeyToInfo {
// 		if len(infos) < 2 {
// 			continue
// 		}

// 		// Check if this public key has different endpoints
// 		endpointSet := make(map[string]bool)
// 		var endpoints []string
// 		for _, info := range infos {
// 			endpointSet[info.endpoint] = true
// 			endpoints = append(endpoints, info.endpoint)
// 		}

// 		// If same public key appears with multiple different endpoints within 5 min window
// 		if len(endpointSet) > 1 {
// 			fmt.Printf("WARNING: Shared config detected! Public key %s seen with multiple endpoints within 5 minutes:\n", publicKey)
// 			details := fmt.Sprintf("Public key %s detected at %d different endpoints", publicKey[:16]+"...", len(endpointSet))
// 			sharedConfigDetails = append(sharedConfigDetails, details)

// 			for _, info := range infos {
// 				fmt.Printf("  - Endpoint: %s (Peer: %s, Last seen: %s)\n", info.endpoint, info.peerID, info.lastSeen.Format(time.RFC3339))
// 				sharedConfigPeers[info.peerID] = true
// 			}

// 			// Create security incident for each affected peer
// 			net, err := s.repo.GetNetwork(ctx, networkID)
// 			if err == nil {
// 				for _, info := range infos {
// 					peer, err := s.repo.GetPeer(ctx, networkID, info.peerID)
// 					if err != nil {
// 						continue
// 					}

// 					incident := &network.SecurityIncident{
// 						ID:           fmt.Sprintf("incident-%s-%d", info.peerID, now.Unix()),
// 						PeerID:       info.peerID,
// 						PeerName:     peer.Name,
// 						NetworkID:    networkID,
// 						NetworkName:  net.Name,
// 						IncidentType: network.IncidentTypeSharedConfig,
// 						DetectedAt:   now,
// 						PublicKey:    publicKey,
// 						Endpoints:    endpoints,
// 						Details:      fmt.Sprintf("Public key %s detected at multiple endpoints: %v", publicKey[:16]+"...", endpoints),
// 						Resolved:     false,
// 					}

// 					if err := s.repo.CreateSecurityIncident(ctx, incident); err != nil {
// 						fmt.Printf("failed to create security incident: %v\n", err)
// 					}
// 				}
// 			}
// 		}
// 	}

// 	// Remove affected peers from all jump servers in the network
// 	if len(sharedConfigPeers) > 0 {
// 		net, err := s.repo.GetNetwork(ctx, networkID)
// 		if err != nil {
// 			return fmt.Errorf("failed to get network: %w", err)
// 		}

// 		// Find all jump servers
// 		for _, peer := range net.Peers {
// 			if !peer.IsJump {
// 				continue
// 			}

// 			// Remove each affected peer from this jump server
// 			for affectedPeerID := range sharedConfigPeers {
// 				if affectedPeerID == peer.ID {
// 					// Don't remove the jump server itself
// 					continue
// 				}

// 				// Check if connection exists
// 				_, err := s.repo.GetConnection(ctx, networkID, peer.ID, affectedPeerID)
// 				if err != nil {
// 					// Connection doesn't exist, skip
// 					continue
// 				}

// 				// Delete the connection to shut down access
// 				if err := s.repo.DeleteConnection(ctx, networkID, peer.ID, affectedPeerID); err != nil {
// 					fmt.Printf("failed to remove peer %s from jump server %s: %v\n", affectedPeerID, peer.ID, err)
// 					continue
// 				}

// 				fmt.Printf("SECURITY: Removed peer %s from jump server %s due to shared configuration detection\n", affectedPeerID, peer.ID)
// 			}
// 		}
// 	}

// 	return nil
// }

// Helper function to get used IPs in the network
// (previous getUsedIPs helper removed; IPAM now tracks used IPs internally)
// (No further helpers.)
