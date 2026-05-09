package network

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"wirety/internal/domain/auth"
	"wirety/internal/domain/ipam"
	"wirety/internal/domain/network"
	"wirety/internal/infrastructure/validation"
	"wirety/pkg/wireguard"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// WebSocketNotifier is an interface for notifying peers about config updates
type WebSocketNotifier interface {
	NotifyNetworkPeers(networkID string)
}

// WebSocketConnectionChecker is an interface for checking if a peer has an active WebSocket connection
type WebSocketConnectionChecker interface {
	IsConnected(networkID, peerID string) bool
}

// PolicyService interface for generating iptables rules
type PolicyService interface {
	GenerateIPTablesRules(ctx context.Context, networkID, jumpPeerID string) ([]string, error)
}

// Service implements the business logic for network management
type Service struct {
	repo                FullRepository
	authRepo            auth.Repository
	groupRepo           network.GroupRepository
	routeRepo           network.RouteRepository
	dnsRepo             network.DNSRepository
	policyRepo          network.PolicyRepository
	policyService       PolicyService
	wsNotifier          WebSocketNotifier
	wsConnectionChecker WebSocketConnectionChecker
}

// SetWebSocketNotifier sets the WebSocket notifier for the service
func (s *Service) SetWebSocketNotifier(notifier WebSocketNotifier) {
	s.wsNotifier = notifier
}

// SetWebSocketConnectionChecker sets the WebSocket connection checker for the service
func (s *Service) SetWebSocketConnectionChecker(checker WebSocketConnectionChecker) {
	s.wsConnectionChecker = checker
}

// ResolveAgentToken returns networkID, peer for a given enrollment token.
func (s *Service) ResolveAgentToken(ctx context.Context, token string) (string, *network.Peer, error) {
	return s.repo.GetPeerByToken(ctx, token)
}

// NewService creates a new network service
func NewService(networkRepo network.Repository, ipamRepo ipam.Repository, authRepo auth.Repository, groupRepo network.GroupRepository, routeRepo network.RouteRepository, dnsRepo network.DNSRepository, policyRepo network.PolicyRepository) *Service {
	return &Service{
		repo:       NewCombinedRepository(networkRepo, ipamRepo),
		authRepo:   authRepo,
		groupRepo:  groupRepo,
		routeRepo:  routeRepo,
		dnsRepo:    dnsRepo,
		policyRepo: policyRepo,
	}
}

// SetPolicyService sets the policy service for iptables rule generation
func (s *Service) SetPolicyService(policyService PolicyService) {
	s.policyService = policyService
}

// CreateNetwork creates a new WireGuard network
func (s *Service) CreateNetwork(ctx context.Context, req *network.NetworkCreateRequest) (*network.Network, error) {
	// Validate network name follows DNS hostname convention (dots allowed for subdomains)
	if err := validation.ValidateDNSHostname(req.Name); err != nil {
		return nil, fmt.Errorf("invalid network name: %w", err)
	}

	// Set default domain suffix if not provided
	domainSuffix := req.DomainSuffix
	if domainSuffix == "" {
		domainSuffix = "internal"
	}

	// Validate domain suffix (dots allowed, e.g. "corp.example.com")
	if err := validation.ValidateDNSHostname(domainSuffix); err != nil {
		return nil, fmt.Errorf("invalid domain suffix: %w", err)
	}

	now := time.Now()

	net := &network.Network{
		ID:              uuid.New().String(),
		Name:            req.Name,
		CIDR:            req.CIDR,
		Peers:           make(map[string]*network.Peer),
		DomainSuffix:    domainSuffix,
		DefaultGroupIDs: []string{}, // Initialize empty default groups
		CreatedAt:       now,
		UpdatedAt:       now,
		DNS:             req.DNS,
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
	// Validate network name if provided (dots allowed for subdomains)
	if req.Name != "" {
		if err := validation.ValidateDNSHostname(req.Name); err != nil {
			return nil, fmt.Errorf("invalid network name: %w", err)
		}
	}

	// Validate domain suffix if provided (dots allowed, e.g. "corp.example.com")
	if req.DomainSuffix != "" {
		if err := validation.ValidateDNSHostname(req.DomainSuffix); err != nil {
			return nil, fmt.Errorf("invalid domain suffix: %w", err)
		}
	}

	net, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	oldCIDR := net.CIDR
	cidrChanged := false
	dnsChanged := false

	if req.Name != "" {
		net.Name = req.Name
	}
	if req.DomainSuffix != "" {
		net.DomainSuffix = req.DomainSuffix
	}
	if req.CIDR != "" && req.CIDR != oldCIDR {
		net.CIDR = req.CIDR
		cidrChanged = true
	}
	if req.DNS != nil {
		if len(req.DNS) != len(net.DNS) {
			dnsChanged = true
		} else {
			for _, dns := range req.DNS {
				match := 0
				for _, existing := range net.DNS {
					if dns == existing {
						match++
						break
					}
				}
				if match != len(net.DNS) {
					dnsChanged = true
					break
				}
			}
		}
		net.DNS = req.DNS
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
				log.Warn().Err(err).Str("ip", peer.Address).Str("cidr", oldCIDR).Msg("failed to release old IP during CIDR migration")
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

	if cidrChanged || dnsChanged {
		if s.wsNotifier != nil {
			s.wsNotifier.NotifyNetworkPeers(networkID)
		}
	}

	return net, nil
}

// AddPeer adds a new peer to the network
func (s *Service) AddPeer(ctx context.Context, networkID string, req *network.PeerCreateRequest, ownerID string) (*network.Peer, error) {
	// Validate peer name follows DNS naming convention
	if err := validation.ValidateDNSName(req.Name); err != nil {
		return nil, fmt.Errorf("invalid peer name: %w", err)
	}

	// Ownership rule (v2):
	//   - Agent-managed peers (jump peers, server agents on dedicated hosts) MAY
	//     be ownerless — they are infrastructure, not user devices, and they
	//     don't go through the captive portal.
	//   - Every other peer (a user device) MUST have an owner so that the
	//     captive portal can match the authenticated user against the peer.
	if !req.IsJump && !req.UseAgent && ownerID == "" {
		return nil, fmt.Errorf("a non-agent peer must have an owner — assign owner_id, or set use_agent=true if this peer runs the wirety agent on a dedicated host")
	}

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

	// Ensure AdditionalAllowedIPs is never nil
	additionalIPs := req.AdditionalAllowedIPs
	if additionalIPs == nil {
		additionalIPs = []string{}
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
		UseAgent:             req.UseAgent,  // Track if peer uses agent or static config
		AdditionalAllowedIPs: additionalIPs, // Ensure never nil to avoid DB constraint violation
		OwnerID:              ownerID,       // Set the owner of the peer
		GroupIDs:             []string{},    // Initialize empty group list
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

	// Check if user is admin or non-admin and handle default groups
	if ownerID != "" && s.authRepo != nil && s.groupRepo != nil {
		user, err := s.authRepo.GetUser(ownerID)
		if err == nil && user != nil {
			// For non-admin users, automatically add peer to network's default groups
			if !user.IsAdministrator() && len(net.DefaultGroupIDs) > 0 {
				for _, groupID := range net.DefaultGroupIDs {
					// Add peer to each default group
					if err := s.groupRepo.AddPeerToGroup(ctx, networkID, groupID, peer.ID); err != nil {
						// Log error but don't fail peer creation
						log.Warn().
							Err(err).
							Str("peer_id", peer.ID).
							Str("group_id", groupID).
							Msg("failed to add peer to default group")
					}
				}
			}
		}
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
	// Validate peer name if provided
	if req.Name != "" {
		if err := validation.ValidateDNSName(req.Name); err != nil {
			return nil, fmt.Errorf("invalid peer name: %w", err)
		}
	}

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
	if req.AdditionalAllowedIPs != nil {
		peer.AdditionalAllowedIPs = req.AdditionalAllowedIPs
	}
	// Ensure AdditionalAllowedIPs is never nil
	if peer.AdditionalAllowedIPs == nil {
		peer.AdditionalAllowedIPs = []string{}
	}
	// Allow owner change (admin only, checked in handler)
	if req.OwnerID != "" {
		peer.OwnerID = req.OwnerID
	}
	peer.UpdatedAt = time.Now()
	// Preserve token (do not allow overwrite via update)

	// Re-check the ownership rule: a non-agent peer must keep an owner.
	if !peer.IsJump && !peer.UseAgent && peer.OwnerID == "" {
		return nil, fmt.Errorf("a non-agent peer must have an owner; cannot leave owner_id empty")
	}

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

	// Get routes for this peer based on group membership
	var peerRoutes []*network.Route
	if s.routeRepo != nil && s.groupRepo != nil {
		// Get all groups this peer belongs to
		groups, err := s.groupRepo.GetPeerGroups(ctx, networkID, peerID)
		if err == nil {
			// Collect all routes from all groups
			routeMap := make(map[string]*network.Route) // Use map to deduplicate routes
			for _, group := range groups {
				routes, err := s.groupRepo.GetGroupRoutes(ctx, networkID, group.ID)
				if err == nil {
					for _, route := range routes {
						routeMap[route.ID] = route
					}
				}
			}
			// Convert map to slice
			for _, route := range routeMap {
				peerRoutes = append(peerRoutes, route)
			}
		}
	}

	config := wireguard.GenerateConfig(peer, allowedPeers, net, presharedKeys, peerRoutes)

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
	IP              string    `json:"ip"`
	Domain          string    `json:"domain"`
	Peers           []DNSPeer `json:"peers"`
	UpstreamServers []string  `json:"upstream_servers"` // Upstream DNS servers for forwarding
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

// JumpPolicy contains policy data for jump agent filtering
type JumpPolicy struct {
	IP            string   `json:"ip"`
	IPTablesRules []string `json:"iptables_rules"` // Generated iptables rules from policies
	Peers         []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		IP       string `json:"ip"`
		UseAgent bool   `json:"use_agent"`
	} `json:"peers"`
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

	// Get routes for this peer based on group membership
	var peerRoutes []*network.Route
	if s.routeRepo != nil && s.groupRepo != nil {
		// Get all groups this peer belongs to
		groups, err := s.groupRepo.GetPeerGroups(ctx, networkID, peerID)
		if err == nil {
			// Collect all routes from all groups
			routeMap := make(map[string]*network.Route) // Use map to deduplicate routes
			for _, group := range groups {
				routes, err := s.groupRepo.GetGroupRoutes(ctx, networkID, group.ID)
				if err == nil {
					for _, route := range routes {
						routeMap[route.ID] = route
					}
				}
			}
			// Convert map to slice
			for _, route := range routeMap {
				peerRoutes = append(peerRoutes, route)
			}
		}
	}

	config := wireguard.GenerateConfig(peer, allowedPeers, net, presharedKeys, peerRoutes)
	var dnsConfig *PeerDNSConfig
	var policy *JumpPolicy
	if peer.IsJump {
		peerList := make([]DNSPeer, 0, len(net.Peers))
		policy = &JumpPolicy{
			IP: peer.Address,
		}

		// Generate iptables rules from policies attached to groups
		if s.policyService != nil {
			iptablesRules, err := s.policyService.GenerateIPTablesRules(ctx, networkID, peerID)
			if err != nil {
				// Log error but don't fail - jump peer can still function without policy rules
				log.Warn().
					Err(err).
					Str("network_id", networkID).
					Str("peer_id", peerID).
					Msg("failed to generate iptables rules for jump peer")
			} else {
				policy.IPTablesRules = iptablesRules
			}
		}

		// Add peer DNS records
		for _, p := range net.Peers {
			peerList = append(peerList, DNSPeer{Name: sanitizeDNSLabel(p.Name), IP: p.Address})
			policy.Peers = append(policy.Peers, struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				IP       string `json:"ip"`
				UseAgent bool   `json:"use_agent"`
			}{
				ID:       p.ID,
				Name:     p.Name,
				IP:       p.Address,
				UseAgent: p.UseAgent,
			})
		}

		// Add route DNS records
		if s.dnsRepo != nil && s.routeRepo != nil {
			routeMappings, err := s.dnsRepo.GetNetworkDNSMappings(ctx, networkID)
			if err == nil {
				// For each DNS mapping, get the route to build FQDN
				for _, mapping := range routeMappings {
					route, err := s.routeRepo.GetRoute(ctx, networkID, mapping.RouteID)
					if err != nil {
						// Skip if route not found
						continue
					}

					// Build FQDN using route's domain suffix
					routeDomainSuffix := route.DomainSuffix
					if routeDomainSuffix == "" {
						routeDomainSuffix = "internal"
					}

					// Format: name.route_name.domain_suffix
					fqdn := fmt.Sprintf("%s.%s.%s", sanitizeDNSLabel(mapping.Name), sanitizeDNSLabel(route.Name), routeDomainSuffix)

					// Add to peer list with the FQDN as the name (without the domain suffix since it's already in the FQDN)
					// The DNS server will use the full FQDN
					peerList = append(peerList, DNSPeer{
						Name: fqdn,
						IP:   mapping.IPAddress,
					})
				}
			}
		}

		// Use network's custom domain suffix
		domainSuffix := net.DomainSuffix
		if domainSuffix == "" {
			domainSuffix = "internal"
		}

		dnsConfig = &PeerDNSConfig{
			IP:              peer.Address,
			Domain:          fmt.Sprintf("%s.%s", net.Name, domainSuffix),
			Peers:           peerList,
			UpstreamServers: net.DNS, // Use network's configured DNS servers for forwarding
		}
	} else {
		// For non-jump peers using agent, send an empty policy to trigger firewall initialization
		// This ensures firewall rules are applied even for non-jump peers
		if peer.UseAgent {
			policy = &JumpPolicy{
				IP: peer.Address,
				Peers: []struct {
					ID       string `json:"id"`
					Name     string `json:"name"`
					IP       string `json:"ip"`
					UseAgent bool   `json:"use_agent"`
				}{},
			}
		}
	}
	return config, dnsConfig, policy, nil
}

// UpdateACL is deprecated - use policy-based access control instead
// Kept for backward compatibility during migration
func (s *Service) UpdateACL(ctx context.Context, networkID string, acl interface{}) error {
	return fmt.Errorf("ACL system has been removed - use policy-based access control instead")
}

// GetACL is deprecated - use policy-based access control instead
// Kept for backward compatibility during migration
func (s *Service) GetACL(ctx context.Context, networkID string) (interface{}, error) {
	return nil, fmt.Errorf("ACL system has been removed - use policy-based access control instead")
}

// DeleteNetwork deletes a network and releases its CIDR from IPAM
func (s *Service) DeleteNetwork(ctx context.Context, networkID string) error {
	// Get the network to retrieve its CIDR before deletion
	net, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to get network for deletion: %w", err)
	}

	// Delete the network first
	if err := s.repo.DeleteNetwork(ctx, networkID); err != nil {
		return fmt.Errorf("failed to delete network: %w", err)
	}

	// Release the CIDR from IPAM to allow reuse
	if err := s.repo.DeletePrefix(ctx, net.CIDR); err != nil {
		// Log the error but don't fail the network deletion
		// The network is already deleted, so we don't want to rollback
		log.Warn().
			Err(err).
			Str("network_id", networkID).
			Str("cidr", net.CIDR).
			Msg("Failed to release CIDR from IPAM after network deletion")
	} else {
		log.Info().
			Str("network_id", networkID).
			Str("cidr", net.CIDR).
			Msg("Successfully released CIDR from IPAM after network deletion")
	}

	return nil
}

// ProcessAgentHeartbeat updates the agent session's last_seen timestamp and, if
// the heartbeat is from a jump peer, prunes captive portal whitelist entries for
// peers no longer reporting an endpoint.
//
// Security-incident detection (session conflict, shared config, port change,
// suspicious activity) was removed in v2 — the captive portal now performs an
// endpoint check on every authenticated connection, which provides a stronger
// guarantee than after-the-fact heartbeat analysis.
func (s *Service) ProcessAgentHeartbeat(ctx context.Context, networkID, peerID string, heartbeat *network.AgentHeartbeat) error {
	now := time.Now()

	// Preserve FirstSeen / SessionID across heartbeats so the session is treated
	// as continuous.  GetSession returns the most recent session for the peer.
	existing, _ := s.repo.GetSession(ctx, networkID, peerID)

	session := &network.AgentSession{
		PeerID:          peerID,
		Hostname:        heartbeat.Hostname,
		SystemUptime:    heartbeat.SystemUptime,
		WireGuardUptime: heartbeat.WireGuardUptime,
		LastSeen:        now,
	}
	if existing != nil {
		session.FirstSeen = existing.FirstSeen
		session.SessionID = existing.SessionID
		session.ReportedEndpoint = existing.ReportedEndpoint
	} else {
		session.FirstSeen = now
		session.SessionID = uuid.NewString()
	}

	if err := s.repo.CreateOrUpdateSession(ctx, networkID, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	// Jump peers also report which other peers are currently connected via
	// WireGuard.  We use those endpoint reports for two things:
	//   1. Update last_seen on each reported peer's session — so the dashboard's
	//      "connected" status badge works for peers that don't run the agent.
	//   2. Prune captive portal whitelist entries for non-agent peers that have
	//      disappeared, so disconnect → re-auth.
	peer, err := s.repo.GetPeer(ctx, networkID, peerID)
	if err == nil && peer.IsJump {
		peers, err := s.repo.ListPeers(ctx, networkID)
		if err == nil {
			activePeerIPs := make(map[string]bool)
			for _, p := range peers {
				endpoint, seen := heartbeat.PeerEndpoints[p.PublicKey]
				if !seen {
					continue
				}
				// Update each reported peer's session to mark them seen now.
				// Skip the heartbeat sender itself (already updated above) and
				// skip agent peers — they send their own heartbeat directly,
				// and overwriting their session here would lose hostname/uptime.
				if p.ID == peerID || p.UseAgent {
					if !p.UseAgent {
						activePeerIPs[p.Address] = true
					}
					continue
				}
				existingP, _ := s.repo.GetSession(ctx, networkID, p.ID)
				ps := &network.AgentSession{
					PeerID:           p.ID,
					ReportedEndpoint: endpoint,
					LastSeen:         now,
				}
				if existingP != nil {
					ps.Hostname = existingP.Hostname
					ps.SystemUptime = existingP.SystemUptime
					ps.WireGuardUptime = existingP.WireGuardUptime
					ps.FirstSeen = existingP.FirstSeen
					ps.SessionID = existingP.SessionID
				} else {
					ps.FirstSeen = now
					ps.SessionID = uuid.NewString()
				}
				_ = s.repo.CreateOrUpdateSession(ctx, networkID, ps)
				activePeerIPs[p.Address] = true
			}
			if err := s.CleanupWhitelistForDisconnectedPeers(ctx, networkID, peerID, activePeerIPs); err != nil {
				log.Error().Err(err).Msg("failed to cleanup whitelist for disconnected peers")
			}
		}
	}

	return nil
}

// PeerConnectivityThreshold is the inactivity window beyond which a peer is
// considered disconnected.  Heartbeats fire every 30 s, so 3 min ≈ 6 missed
// heartbeats — close to WireGuard's own 180 s activity threshold.
const PeerConnectivityThreshold = 3 * time.Minute

// GetPeerConnectivityStatus reports whether a peer is currently considered
// connected, based on its WebSocket presence (for agent peers) and the
// freshness of its last heartbeat.
func (s *Service) GetPeerConnectivityStatus(ctx context.Context, networkID, peerID string) (*network.PeerConnectivityStatus, error) {
	now := time.Now()
	status := &network.PeerConnectivityStatus{
		PeerID:      peerID,
		LastChecked: now,
	}

	// Live WebSocket presence is the strongest signal that a peer's agent is up.
	if s.wsConnectionChecker != nil {
		status.HasActiveAgent = s.wsConnectionChecker.IsConnected(networkID, peerID)
	}

	// Even without an active WS, a recent heartbeat counts as "connected".
	session, err := s.repo.GetSession(ctx, networkID, peerID)
	if err == nil && session != nil {
		status.CurrentSession = session
		if !status.HasActiveAgent && now.Sub(session.LastSeen) <= PeerConnectivityThreshold {
			status.HasActiveAgent = true
		}
	}

	return status, nil
}

// ListSessions returns all sessions in a network
func (s *Service) ListSessions(ctx context.Context, networkID string) ([]*network.AgentSession, error) {
	return s.repo.ListSessions(ctx, networkID)
}

// CreateCaptivePortalToken creates a short-lived token for the captive portal flow.
// Called by the jump peer agent when a new peer connects and needs authentication.
// peerEndpoint is the peer's current full public endpoint ("ip:port", strict);
// it is stored in the token so that AddCaptivePortalWhitelist can bind the
// whitelist entry to a specific source IP+port — any change (different network,
// NAT port rebinding, tunnel restart) forces re-authentication.  May be empty
// for legacy agents.
func (s *Service) CreateCaptivePortalToken(ctx context.Context, networkID, jumpPeerID, peerIP, peerEndpoint string) (*network.CaptivePortalToken, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	now := time.Now()
	token := &network.CaptivePortalToken{
		Token:          "cpt_" + base64.RawURLEncoding.EncodeToString(tokenBytes),
		NetworkID:      networkID,
		JumpPeerID:     jumpPeerID,
		PeerIP:         peerIP,
		PeerEndpoint: peerEndpoint,
		CreatedAt:      now,
		ExpiresAt:      now.Add(10 * time.Minute),
	}

	if err := s.repo.CreateCaptivePortalToken(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to create captive portal token: %w", err)
	}

	return token, nil
}

// AuthenticateCaptivePortal validates a captive portal token + session hash, then whitelists the peer.
// Called by the frontend captive portal page after the user authenticates via OIDC/password.
func (s *Service) AuthenticateCaptivePortal(ctx context.Context, captiveToken, sessionHash string) (*network.CaptivePortalToken, error) {
	cpt, err := s.repo.GetCaptivePortalToken(ctx, captiveToken)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	if !cpt.IsValid() {
		_ = s.repo.DeleteCaptivePortalToken(ctx, captiveToken)
		return nil, fmt.Errorf("token expired")
	}

	// Validate session (either OIDC or simple-auth session)
	session, err := s.authRepo.GetSession(sessionHash)
	if err != nil || session == nil || !session.IsValid() {
		return nil, fmt.Errorf("invalid session")
	}

	// Verify that the authenticated user is the owner of the peer they are trying to access.
	// Rules:
	//   - Captive portal requires OIDC (simple-auth callers are rejected at the handler level).
	//   - Ownerless peers (admin-created) cannot be authenticated via captive portal.
	//   - Administrators are subject to the same ownership rules as regular users.
	authUser, err := s.authRepo.GetUser(session.UserID)
	if err != nil || authUser == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Find the peer in this network whose VPN address matches the captive token's peer IP
	peers, err := s.repo.ListPeers(ctx, cpt.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up peer: %w", err)
	}
	var matchedPeer *network.Peer
	for _, p := range peers {
		addr := p.Address
		if idx := strings.Index(addr, "/"); idx != -1 {
			addr = addr[:idx]
		}
		if addr == cpt.PeerIP {
			matchedPeer = p
			break
		}
	}
	if matchedPeer == nil {
		return nil, fmt.Errorf("peer not found in network")
	}
	// Ownerless peers cannot use the captive portal
	if matchedPeer.OwnerID == "" {
		return nil, fmt.Errorf("access denied: this peer has no owner and cannot be authenticated via captive portal")
	}
	// The authenticated user must be the peer's owner (admins are not exempt)
	if matchedPeer.OwnerID != authUser.ID {
		return nil, fmt.Errorf("access denied: this peer belongs to another user")
	}

	// Whitelist the peer — also triggers WebSocket notification to jump peer.
	// AddCaptivePortalWhitelist is idempotent (ON CONFLICT DO NOTHING), so repeated
	// calls for the same peer are safe.
	if err := s.AddCaptivePortalWhitelist(ctx, cpt.NetworkID, cpt.JumpPeerID, cpt.PeerIP, cpt.PeerEndpoint); err != nil {
		return nil, fmt.Errorf("failed to whitelist peer: %w", err)
	}

	// Do NOT delete the token here. The redirect server caches the token for up to
	// 9 minutes (tokenTTL) to avoid creating a new DB token on every intercepted
	// HTTP request. If the agent has not yet synced iptables by the time the browser
	// follows the post-authentication redirect, the next HTTP request from the peer
	// will be DNAT'd again and the captive portal page will attempt to authenticate
	// with the same cached token. Keeping the token alive (it expires after 10 min)
	// makes that second attempt succeed gracefully via the idempotent whitelist upsert.

	return cpt, nil
}

// AddCaptivePortalWhitelist adds a peer IP to the captive portal whitelist.
// peerEndpoint is the peer's full public endpoint ("ip:port", strict) recorded
// at authentication time; the jump peer uses it to verify that the peer is
// still connecting from the exact same source IP+port.  Pass an empty string
// to store a legacy (endpoint-unchecked) entry.
func (s *Service) AddCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP, peerEndpoint string) error {
	if err := s.repo.AddCaptivePortalWhitelist(ctx, networkID, jumpPeerID, peerIP, peerEndpoint); err != nil {
		return err
	}

	// Notify jump peer to update firewall rules
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// RemoveCaptivePortalWhitelist removes a peer IP from the captive portal whitelist
func (s *Service) RemoveCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	if err := s.repo.RemoveCaptivePortalWhitelist(ctx, networkID, jumpPeerID, peerIP); err != nil {
		return err
	}

	// Notify jump peer to update firewall rules
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// GetCaptivePortalWhitelist retrieves the whitelist for a jump peer
func (s *Service) GetCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) ([]string, error) {
	return s.repo.GetCaptivePortalWhitelist(ctx, networkID, jumpPeerID)
}

// CleanupWhitelistForDisconnectedPeers removes peers from whitelist when their connection is down
func (s *Service) CleanupWhitelistForDisconnectedPeers(ctx context.Context, networkID string, jumpPeerID string, activePeerIPs map[string]bool) error {
	// Get current whitelist
	whitelist, err := s.repo.GetCaptivePortalWhitelist(ctx, networkID, jumpPeerID)
	if err != nil {
		return fmt.Errorf("failed to get whitelist: %w", err)
	}

	// Remove peers that are no longer active.
	// Whitelist entries may be "wgIP@endpointIP" — extract the wgIP part.
	for _, entry := range whitelist {
		wgIP := entry
		if idx := strings.IndexByte(entry, '@'); idx != -1 {
			wgIP = entry[:idx]
		}
		if !activePeerIPs[wgIP] {
			log.Info().
				Str("network_id", networkID).
				Str("jump_peer_id", jumpPeerID).
				Str("peer_ip", wgIP).
				Msg("removing disconnected peer from whitelist")

			if err := s.repo.RemoveCaptivePortalWhitelist(ctx, networkID, jumpPeerID, wgIP); err != nil {
				log.Error().Err(err).Str("peer_ip", wgIP).Msg("failed to remove peer from whitelist")
			}
		}
	}

	// Notify jump peer to update firewall rules
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}


