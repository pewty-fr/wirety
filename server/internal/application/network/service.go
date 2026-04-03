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

	// Filter out quarantined peers from allowed peers (for jump servers)
	// This ensures quarantined peers are completely disconnected from the network
	if peer.IsJump {
		allowedPeers = s.filterQuarantinedPeers(ctx, networkID, allowedPeers)
	}

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

	// Filter out quarantined peers from allowed peers (for jump servers)
	// This ensures quarantined peers are completely disconnected from the network
	if peer.IsJump {
		allowedPeers = s.filterQuarantinedPeers(ctx, networkID, allowedPeers)
	}

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

// blockPeerInACL is deprecated - ACL system has been removed
// Security incidents should now be handled through policy-based access control
func (s *Service) blockPeerInACL(ctx context.Context, networkID, peerID string, reason string) error {
	peer, _ := s.repo.GetPeer(ctx, networkID, peerID)
	peerName := peerID
	if peer != nil {
		peerName = peer.Name
	}

	log.Warn().
		Str("peer_id", peerID).
		Str("peer_name", peerName).
		Str("reason", reason).
		Msg("SECURITY: Peer security incident detected - implementing policy-based blocking")

	// Implement policy-based blocking using groups and policies
	if s.groupRepo == nil || s.policyService == nil {
		log.Error().Msg("cannot block peer: group repository or policy service not available")
		return fmt.Errorf("policy-based blocking not available")
	}

	// 1. Ensure quarantine group exists
	quarantineGroupID, err := s.ensureQuarantineGroup(ctx, networkID)
	if err != nil {
		log.Error().Err(err).Msg("failed to ensure quarantine group")
		return fmt.Errorf("failed to create quarantine group: %w", err)
	}

	// 2. Add peer to quarantine group
	err = s.groupRepo.AddPeerToGroup(ctx, networkID, quarantineGroupID, peerID)
	if err != nil {
		log.Error().Err(err).Msg("failed to add peer to quarantine group")
		return fmt.Errorf("failed to quarantine peer: %w", err)
	}

	log.Info().
		Str("peer_id", peerID).
		Str("peer_name", peerName).
		Str("group_id", quarantineGroupID).
		Str("reason", reason).
		Msg("SECURITY: Peer quarantined successfully")

	// 3. Revoke captive portal whitelist so quarantined peer loses iptables ACCEPT immediately
	if peer != nil && peer.Address != "" {
		// Strip CIDR suffix if present (e.g. "10.0.0.2/22" → "10.0.0.2")
		peerIP := peer.Address
		if idx := strings.Index(peerIP, "/"); idx != -1 {
			peerIP = peerIP[:idx]
		}
		if err := s.repo.RemoveCaptivePortalWhitelistByPeerIP(ctx, networkID, peerIP); err != nil {
			log.Warn().Err(err).Str("peer_ip", peerIP).Msg("SECURITY: failed to revoke captive portal whitelist for quarantined peer")
		} else {
			log.Info().Str("peer_ip", peerIP).Msg("SECURITY: captive portal whitelist revoked for quarantined peer")
		}
	}

	// 4. Notify all peers to regenerate configs
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// ensureQuarantineGroup ensures a quarantine group with deny-all policy exists
func (s *Service) ensureQuarantineGroup(ctx context.Context, networkID string) (string, error) {
	// Check if quarantine group already exists
	groups, err := s.groupRepo.ListGroups(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to list groups: %w", err)
	}

	for _, group := range groups {
		if group.Name == "quarantine" {
			return group.ID, nil
		}
	}

	// Resolve or create the deny-all policy for the quarantine group.
	// We check for an existing policy first so that ensureQuarantineGroup is fully
	// idempotent: a previous partial failure (policy created, group creation failed)
	// would otherwise cause a unique-constraint error on the second attempt.
	var denyAllPolicyID string
	if s.policyRepo != nil {
		existingPolicies, listErr := s.policyRepo.ListPolicies(ctx, networkID)
		if listErr == nil {
			for _, p := range existingPolicies {
				if p.Name == "quarantine-deny-all" {
					denyAllPolicyID = p.ID
					break
				}
			}
		}

		if denyAllPolicyID == "" {
			// Policy does not exist yet — create it.
			denyAllPolicyID = uuid.New().String()
			denyAllPolicy := &network.Policy{
				ID:          denyAllPolicyID,
				NetworkID:   networkID,
				Name:        "quarantine-deny-all",
				Description: "automatically created policy to deny all traffic for quarantined peers",
				Rules: []network.PolicyRule{
					{
						ID:          uuid.New().String(),
						Direction:   "output",
						Action:      "deny",
						Target:      "0.0.0.0/0",
						TargetType:  "cidr",
						Description: "deny all outbound traffic",
					},
					{
						ID:          uuid.New().String(),
						Direction:   "input",
						Action:      "deny",
						Target:      "0.0.0.0/0",
						TargetType:  "cidr",
						Description: "deny all inbound traffic",
					},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err = s.policyRepo.CreatePolicy(ctx, networkID, denyAllPolicy); err != nil {
				return "", fmt.Errorf("failed to create quarantine deny-all policy: %w", err)
			}
		}
	} else {
		log.Warn().Msg("Policy repository not available - quarantine group created without deny-all policy")
	}

	// Create quarantine group with priority 0 (highest priority)
	var policyIDs []string
	if denyAllPolicyID != "" {
		policyIDs = []string{denyAllPolicyID} // Attach the deny-all policy if created
	}

	quarantineGroup := &network.Group{
		ID:          uuid.New().String(),
		NetworkID:   networkID,
		Name:        "quarantine",
		Description: "automatically created group for blocking compromised peers",
		Priority:    0, // Highest priority - quarantine policies apply first
		PeerIDs:     []string{},
		PolicyIDs:   policyIDs,
		RouteIDs:    []string{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = s.groupRepo.CreateGroup(ctx, networkID, quarantineGroup)
	if err != nil {
		return "", fmt.Errorf("failed to create quarantine group: %w", err)
	}

	// Log that quarantine group was created
	if denyAllPolicyID != "" {
		log.Warn().
			Str("network_id", networkID).
			Str("group_id", quarantineGroup.ID).
			Str("policy_id", denyAllPolicyID).
			Msg("SECURITY: Quarantine group created with deny-all policy to block quarantined peers")
	} else {
		log.Warn().
			Str("network_id", networkID).
			Str("group_id", quarantineGroup.ID).
			Msg("SECURITY: Quarantine group created - admin should manually attach deny-all policy to block quarantined peers")
	}

	return quarantineGroup.ID, nil
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
	securityConfig := s.getSecurityConfigWithDefaults(ctx, networkID)
	activeSessionThreshold := now.Add(-securityConfig.SessionConflictThreshold)

	var currentSession *network.AgentSession
	for _, session := range existingSessions {
		// Consider sessions active if seen within threshold
		if session.LastSeen.After(activeSessionThreshold) {
			if session.Hostname != heartbeat.Hostname && session.ReportedEndpoint != "" && session.SystemUptime != -1 && session.WireGuardUptime != -1 {
				peer, err := s.repo.GetPeer(ctx, networkID, peerID)
				if err == nil {
					net, err := s.repo.GetNetwork(ctx, networkID)
					if err == nil {
						// Check if there's already an open incident of this type for this peer
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

							// Only create security incident if security detection is enabled
							if securityConfig.Enabled {
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
		activeSessionThreshold := now.Add(-securityConfig.SessionConflictThreshold)

		var currentSess *network.AgentSession
		if len(existingSess) > 0 {
			for _, sess := range existingSess {
				// For non agent session
				if sess.Hostname == "" && sess.SystemUptime == -1 && sess.WireGuardUptime == -1 {
					currentSess = sess
					currentSess.LastSeen = now
					_ = s.repo.CreateOrUpdateSession(ctx, networkID, currentSess)
					break
				}
				// Consider sessions active if seen within threshold
				if sess.LastSeen.After(activeSessionThreshold) {
					currentSess = sess
					break
				}
			}
		} else {
			currentSess = &network.AgentSession{
				PeerID:          id,
				Hostname:        "",
				SystemUptime:    -1,
				WireGuardUptime: -1,
				LastSeen:        now,
				SessionID:       uuid.NewString(),
			}
		}
		if currentSess == nil {
			continue
		}
		if currentSess.ReportedEndpoint == "" {
			currentSess.ReportedEndpoint = endpoint
			_ = s.repo.CreateOrUpdateSession(ctx, networkID, currentSess)
			change := &network.EndpointChange{
				PeerID:      currentSess.PeerID,
				OldEndpoint: "",
				NewEndpoint: endpoint,
				ChangedAt:   now,
				Source:      peerID,
			}
			if err := s.repo.RecordEndpointChange(ctx, networkID, change); err != nil {
				// Log but don't fail on endpoint change recording error
				fmt.Printf("failed to record endpoint change: %v\n", err)
			}
			continue
		}
		if currentSess.ReportedEndpoint == endpoint {
			continue
		}
		changes, err := s.repo.GetEndpointChanges(ctx, networkID, currentSess.PeerID, now.Add(-24*time.Hour))

		// Check if this source has seen a different endpoint for this peer
		shouldRecordChange := false
		lastEndpointFromThisSource := ""

		if err == nil && len(changes) > 0 {
			// Look for the most recent change from the same source
			for _, change := range changes {
				if change.Source == peerID {
					lastEndpointFromThisSource = change.NewEndpoint
					break // Found the most recent change from this source
				}
			}
		}

		// Only record change if:
		// 1. This source has never seen this peer before (lastEndpointFromThisSource == ""), OR
		// 2. This source has seen this peer but with a different endpoint
		if lastEndpointFromThisSource == "" || lastEndpointFromThisSource != endpoint {
			shouldRecordChange = true
		}

		if shouldRecordChange {
			change := &network.EndpointChange{
				PeerID:      currentSess.PeerID,
				OldEndpoint: lastEndpointFromThisSource,
				NewEndpoint: endpoint,
				ChangedAt:   now,
				Source:      peerID,
			}
			if err := s.repo.RecordEndpointChange(ctx, networkID, change); err != nil {
				// Log but don't fail on endpoint change recording error
				fmt.Printf("failed to record endpoint change: %v\n", err)
			}

			// Only remove from whitelist if this represents an actual endpoint change (not first time seeing)
			if lastEndpointFromThisSource != "" {
				// Remove peer from whitelist due to endpoint change
				peer, err := s.repo.GetPeer(ctx, networkID, currentSess.PeerID)
				if err == nil && !peer.UseAgent {
					// This is a non-agent peer with endpoint change - remove from whitelist
					if err := s.RemoveFromWhitelistOnEndpointChange(ctx, networkID, peer.Address); err != nil {
						log.Error().Err(err).Str("peer_ip", peer.Address).Msg("failed to remove peer from whitelist on endpoint change")
					}
				}
			}
		}

		// Always update the session's last seen time, but don't update ReportedEndpoint
		// since multiple sources might see different endpoints for the same peer
		currentSess.LastSeen = now
		_ = s.repo.CreateOrUpdateSession(ctx, networkID, currentSess)
	}

	// If this is a jump peer, cleanup whitelist for disconnected peers
	peer, err := s.repo.GetPeer(ctx, networkID, peerID)
	if err == nil && peer.IsJump {
		// Build map of active peer IPs from the heartbeat
		activePeerIPs := make(map[string]bool)
		for _, p := range peers {
			if !p.UseAgent {
				// Check if this peer has an active endpoint reported
				if _, hasEndpoint := heartbeat.PeerEndpoints[p.PublicKey]; hasEndpoint {
					activePeerIPs[p.Address] = true
				}
			}
		}

		// Cleanup whitelist for disconnected peers
		if err := s.CleanupWhitelistForDisconnectedPeers(ctx, networkID, peerID, activePeerIPs); err != nil {
			log.Error().Err(err).Msg("failed to cleanup whitelist for disconnected peers")
		}
	}

	err = s.detectAndHandleSharedConfigs(ctx, networkID)
	if err != nil {
		log.Error().Err(err).Msg("failed to detect and handle shared configs")
	}
	err = s.detectAndHandlePortChanges(ctx, networkID)
	if err != nil {
		log.Error().Err(err).Msg("failed to detect and handle port changes")
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
	securityConfig := s.getSecurityConfigWithDefaults(ctx, networkID)
	activeThreshold := now.Add(-securityConfig.SessionConflictThreshold)

	status := &network.PeerSessionStatus{
		PeerID:                peerID,
		HasActiveAgent:        false,
		ConflictingSessions:   []network.AgentSession{},
		RecentEndpointChanges: []network.EndpointChange{},
		SuspiciousActivity:    false,
		LastChecked:           now,
	}

	// Check if peer has an active WebSocket connection to the server
	if s.wsConnectionChecker != nil {
		status.HasActiveAgent = s.wsConnectionChecker.IsConnected(networkID, peerID)
	}

	// Find active sessions (for security incident detection)
	var activeSessions []*network.AgentSession
	for _, session := range sessions {
		if session.LastSeen.After(activeThreshold) {
			activeSessions = append(activeSessions, session)
		}
	}

	if len(activeSessions) > 0 {
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
		if len(changes) >= securityConfig.MaxEndpointChangesPerDay {
			status.SuspiciousActivity = true
		}

		// Check for rapid endpoint changes
		for i := 1; i < len(changes); i++ {
			timeDiff := changes[i].ChangedAt.Sub(changes[i-1].ChangedAt)
			if timeDiff < securityConfig.EndpointChangeThreshold {
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
	// First get the incident to find the peer ID
	incident, err := s.repo.GetSecurityIncident(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("failed to get incident: %w", err)
	}

	// Resolve the incident
	if err := s.repo.ResolveSecurityIncident(ctx, incidentID, resolvedBy); err != nil {
		return err
	}

	log.Info().
		Str("incident_id", incidentID).
		Str("peer_id", incident.PeerID).
		Str("peer_name", incident.PeerName).
		Str("resolved_by", resolvedBy).
		Msg("SECURITY: Security incident resolved")

	// Clean up endpoint changes for the peer
	if incident.PeerID != "" && incident.NetworkID != "" {
		// Delete all endpoint changes for this peer
		if err := s.repo.DeleteEndpointChanges(ctx, incident.NetworkID, incident.PeerID); err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", incident.PeerID).
				Str("network_id", incident.NetworkID).
				Msg("failed to delete endpoint changes")
		} else {
			log.Info().
				Str("peer_id", incident.PeerID).
				Str("network_id", incident.NetworkID).
				Msg("deleted endpoint changes for resolved incident")
		}

		// Reset the reported endpoint in agent sessions
		sessions, err := s.repo.GetActiveSessionsForPeer(ctx, incident.NetworkID, incident.PeerID)
		if err == nil {
			for _, session := range sessions {
				// Reset the reported endpoint to empty
				session.ReportedEndpoint = ""
				if err := s.repo.CreateOrUpdateSession(ctx, incident.NetworkID, session); err != nil {
					log.Warn().
						Err(err).
						Str("peer_id", incident.PeerID).
						Str("session_id", session.SessionID).
						Msg("failed to reset reported endpoint for session")
				} else {
					log.Info().
						Str("peer_id", incident.PeerID).
						Str("session_id", session.SessionID).
						Msg("reset reported endpoint for session")
				}
			}
		}

		if err := s.reconnectPeer(ctx, incident.NetworkID, incident.PeerID); err != nil {
			return fmt.Errorf("can't remove peer from quarantine: %w", err)
		}
	}

	return nil
}

// ReconnectPeer removes a peer from the quarantine group to restore network access
func (s *Service) reconnectPeer(ctx context.Context, networkID, peerID string) error {
	peer, err := s.repo.GetPeer(ctx, networkID, peerID)
	if err != nil {
		return fmt.Errorf("peer not found: %w", err)
	}

	log.Info().
		Str("peer_id", peerID).
		Str("peer_name", peer.Name).
		Msg("SECURITY: Peer reconnect requested - removing from quarantine group")

	// Check if group repository is available
	if s.groupRepo == nil {
		log.Error().Msg("cannot reconnect peer: group repository not available")
		return fmt.Errorf("policy-based reconnection not available")
	}

	// Get the quarantine group
	quarantineGroupID, err := s.getQuarantineGroupID(ctx, networkID)
	if err != nil {
		// If quarantine group doesn't exist, peer is not quarantined
		log.Info().
			Str("peer_id", peerID).
			Msg("Quarantine group does not exist, peer is not quarantined")
		return nil
	}

	// Remove peer from quarantine group (idempotent - no error if peer not in group)
	err = s.groupRepo.RemovePeerFromGroup(ctx, networkID, quarantineGroupID, peerID)
	if err != nil {
		log.Error().Err(err).Msg("failed to remove peer from quarantine group")
		return fmt.Errorf("failed to reconnect peer: %w", err)
	}

	log.Info().
		Str("peer_id", peerID).
		Str("peer_name", peer.Name).
		Str("group_id", quarantineGroupID).
		Msg("SECURITY: Peer reconnected successfully")

	// Notify all peers to regenerate configs
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// getQuarantineGroupID retrieves the quarantine group ID for a network
func (s *Service) getQuarantineGroupID(ctx context.Context, networkID string) (string, error) {
	groups, err := s.groupRepo.ListGroups(ctx, networkID)
	if err != nil {
		return "", fmt.Errorf("failed to list groups: %w", err)
	}

	for _, group := range groups {
		if group.Name == "quarantine" {
			return group.ID, nil
		}
	}

	return "", fmt.Errorf("quarantine group not found")
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

// extractIP returns the IP part of a host:port endpoint string, stripping the port.
// Handles IPv6 bracket notation ([::1]:port) and plain IPv4 (1.2.3.4:port).
func extractIP(endpoint string) string {
	if strings.HasPrefix(endpoint, "[") {
		if idx := strings.LastIndex(endpoint, "]:"); idx != -1 {
			return endpoint[1:idx] // strip brackets
		}
		return strings.Trim(endpoint, "[]")
	}
	if idx := strings.LastIndex(endpoint, ":"); idx != -1 {
		return endpoint[:idx]
	}
	return endpoint
}

// detectAndHandleSharedConfigs detects if the same peer is seen at multiple
// *different IP addresses* from the same source within a short time window.
// This is a strong signal that the WireGuard private key is being used by
// different people/machines.  Port-only changes (NAT rebinding) are handled
// separately by detectAndHandlePortChanges.
func (s *Service) detectAndHandleSharedConfigs(ctx context.Context, networkID string) error {
	securityConfig := s.getSecurityConfigWithDefaults(ctx, networkID)
	if !securityConfig.Enabled {
		return nil
	}

	peers, err := s.repo.ListPeers(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to list peers: %w", err)
	}

	now := time.Now()
	checkWindow := now.Add(-securityConfig.EndpointChangeThreshold)

	sharedConfigPeers := make(map[string]bool)

	for _, peer := range peers {
		changes, err := s.repo.GetEndpointChanges(ctx, networkID, peer.ID, checkWindow)
		if err != nil || len(changes) < 2 {
			continue
		}

		// Group changes by source (jump server that observed them)
		changesBySource := make(map[string][]*network.EndpointChange)
		for _, change := range changes {
			changesBySource[change.Source] = append(changesBySource[change.Source], change)
		}

		for sourceID, sourceChanges := range changesBySource {
			if len(sourceChanges) < 2 {
				continue
			}

			// Collect unique IPs (not full endpoints) seen from this source
			ipSet := make(map[string]bool)
			var uniqueIPs []string
			// Also collect all full endpoints for the incident details
			epSet := make(map[string]bool)
			var endpoints []string
			for _, change := range sourceChanges {
				if change.NewEndpoint == "" {
					continue
				}
				if !epSet[change.NewEndpoint] {
					epSet[change.NewEndpoint] = true
					endpoints = append(endpoints, change.NewEndpoint)
				}
				ip := extractIP(change.NewEndpoint)
				if !ipSet[ip] {
					ipSet[ip] = true
					uniqueIPs = append(uniqueIPs, ip)
				}
			}

			// 2+ different IPs from the same source = shared config (strong signal)
			if len(uniqueIPs) >= 2 {
				log.Warn().
					Str("peer_name", peer.Name).
					Str("peer_id", peer.ID).
					Str("source", sourceID).
					Dur("window", securityConfig.EndpointChangeThreshold).
					Strs("ips", uniqueIPs).
					Strs("endpoints", endpoints).
					Msg("Shared config detected: peer seen at multiple IPs from same source")

				sharedConfigPeers[peer.ID] = true

				if err := s.createIncidentIfNew(ctx, networkID, peer, network.IncidentTypeSharedConfig, endpoints,
					fmt.Sprintf("Peer %s detected at %d different IPs from source %s within %v: %v",
						peer.Name, len(uniqueIPs), sourceID, securityConfig.EndpointChangeThreshold, uniqueIPs),
				); err != nil {
					log.Error().Err(err).Str("peer_id", peer.ID).Msg("failed to create shared-config incident")
				}
				break // one source is enough to flag this peer
			}
		}
	}

	// Quarantine peers with shared configs
	for affectedPeerID := range sharedConfigPeers {
		if err := s.blockPeerInACL(ctx, networkID, affectedPeerID, "shared configuration detection"); err != nil {
			log.Error().Err(err).Str("peer_id", affectedPeerID).Msg("failed to block peer in ACL")
		}
	}

	return nil
}

// detectAndHandlePortChanges detects excessive port-only changes (same IP,
// different port) which typically indicate NAT rebinding.  These are far less
// severe than IP changes and do NOT quarantine the peer — they only create
// an incident for visibility.
func (s *Service) detectAndHandlePortChanges(ctx context.Context, networkID string) error {
	securityConfig := s.getSecurityConfigWithDefaults(ctx, networkID)
	if !securityConfig.Enabled {
		return nil
	}

	peers, err := s.repo.ListPeers(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to list peers: %w", err)
	}

	now := time.Now()
	checkWindow := now.Add(-securityConfig.PortChangeThreshold)

	for _, peer := range peers {
		changes, err := s.repo.GetEndpointChanges(ctx, networkID, peer.ID, checkWindow)
		if err != nil || len(changes) < 2 {
			continue
		}

		changesBySource := make(map[string][]*network.EndpointChange)
		for _, change := range changes {
			changesBySource[change.Source] = append(changesBySource[change.Source], change)
		}

		for sourceID, sourceChanges := range changesBySource {
			// Count port-only changes: same IP, different full endpoint
			ipEndpoints := make(map[string]map[string]bool) // ip -> set of full endpoints
			for _, change := range sourceChanges {
				if change.NewEndpoint == "" {
					continue
				}
				ip := extractIP(change.NewEndpoint)
				if ipEndpoints[ip] == nil {
					ipEndpoints[ip] = make(map[string]bool)
				}
				ipEndpoints[ip][change.NewEndpoint] = true
			}

			for ip, epSet := range ipEndpoints {
				portChangeCount := len(epSet)
				if portChangeCount > securityConfig.MaxPortChangesPerWindow {
					var endpoints []string
					for ep := range epSet {
						endpoints = append(endpoints, ep)
					}

					log.Warn().
						Str("peer_name", peer.Name).
						Str("peer_id", peer.ID).
						Str("source", sourceID).
						Str("ip", ip).
						Int("port_changes", portChangeCount).
						Int("threshold", securityConfig.MaxPortChangesPerWindow).
						Dur("window", securityConfig.PortChangeThreshold).
						Msg("Excessive port changes detected (NAT rebinding)")

					// Create incident but do NOT quarantine — port changes are usually benign
					if err := s.createIncidentIfNew(ctx, networkID, peer, network.IncidentTypeSuspiciousActivity, endpoints,
						fmt.Sprintf("Peer %s has %d port changes from IP %s (source %s) within %v (threshold: %d)",
							peer.Name, portChangeCount, ip, sourceID, securityConfig.PortChangeThreshold, securityConfig.MaxPortChangesPerWindow),
					); err != nil {
						log.Error().Err(err).Str("peer_id", peer.ID).Msg("failed to create port-change incident")
					}
				}
			}
		}
	}

	return nil
}

func (s *Service) detectAndHandleSuspicousActivity(ctx context.Context, networkID string) error {
	securityConfig := s.getSecurityConfigWithDefaults(ctx, networkID)
	if !securityConfig.Enabled {
		return nil
	}

	peers, err := s.repo.ListPeers(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to list peers: %w", err)
	}

	now := time.Now()
	checkWindow := now.Add(-time.Hour * 24)

	suspiciousActivityPeers := make(map[string]bool)

	for _, peer := range peers {
		changes, err := s.repo.GetEndpointChanges(ctx, networkID, peer.ID, checkWindow)
		if err != nil {
			continue
		}

		changesBySource := make(map[string][]*network.EndpointChange)
		for _, change := range changes {
			changesBySource[change.Source] = append(changesBySource[change.Source], change)
		}

		for sourceID, sourceChanges := range changesBySource {
			// Only count IP-level changes (not port-only) toward the daily limit
			ipChanges := 0
			var prevIP string
			epSet := make(map[string]bool)
			var endpoints []string
			for _, change := range sourceChanges {
				if change.NewEndpoint == "" {
					continue
				}
				if !epSet[change.NewEndpoint] {
					epSet[change.NewEndpoint] = true
					endpoints = append(endpoints, change.NewEndpoint)
				}
				ip := extractIP(change.NewEndpoint)
				if prevIP != "" && ip != prevIP {
					ipChanges++
				}
				prevIP = ip
			}

			if ipChanges >= securityConfig.MaxEndpointChangesPerDay {
				log.Warn().
					Str("peer_name", peer.Name).
					Str("peer_id", peer.ID).
					Str("source", sourceID).
					Int("ip_changes", ipChanges).
					Int("threshold", securityConfig.MaxEndpointChangesPerDay).
					Msg("Suspicious activity: excessive IP-level endpoint changes in 24h")

				suspiciousActivityPeers[peer.ID] = true

				if err := s.createIncidentIfNew(ctx, networkID, peer, network.IncidentTypeSuspiciousActivity, endpoints,
					fmt.Sprintf("Peer %s has %d IP-level endpoint changes from source %s within 24h (threshold: %d)",
						peer.Name, ipChanges, sourceID, securityConfig.MaxEndpointChangesPerDay),
				); err != nil {
					log.Error().Err(err).Str("peer_id", peer.ID).Msg("failed to create suspicious-activity incident")
				}
			}
		}
	}

	// Quarantine peers with excessive IP changes
	for affectedPeerID := range suspiciousActivityPeers {
		if err := s.blockPeerInACL(ctx, networkID, affectedPeerID, "suspicious activity detection"); err != nil {
			log.Error().Err(err).Str("peer_id", affectedPeerID).Msg("failed to block peer in ACL")
		}
	}

	return nil
}

// createIncidentIfNew creates a security incident only if no open incident of the
// same type already exists for the given peer.
func (s *Service) createIncidentIfNew(ctx context.Context, networkID string, peer *network.Peer, incidentType string, endpoints []string, details string) error {
	net, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("network not found: %w", err)
	}

	resolved := false
	existingIncidents, err := s.repo.ListSecurityIncidentsByNetwork(ctx, networkID, &resolved)
	if err == nil {
		for _, existing := range existingIncidents {
			if existing.IncidentType == incidentType && existing.PeerID == peer.ID {
				return nil // already have an open incident
			}
		}
	}

	incident := &network.SecurityIncident{
		ID:           fmt.Sprintf("incident-%s-%d", peer.ID, time.Now().Unix()),
		PeerID:       peer.ID,
		PeerName:     peer.Name,
		NetworkID:    networkID,
		NetworkName:  net.Name,
		IncidentType: incidentType,
		DetectedAt:   time.Now(),
		PublicKey:     peer.PublicKey,
		Endpoints:    endpoints,
		Details:      details,
		Resolved:     false,
	}

	return s.repo.CreateSecurityIncident(ctx, incident)
}

// CreateCaptivePortalToken creates a short-lived token for the captive portal flow.
// Called by the jump peer agent when a new peer connects and needs authentication.
func (s *Service) CreateCaptivePortalToken(ctx context.Context, networkID, jumpPeerID, peerIP string) (*network.CaptivePortalToken, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	now := time.Now()
	token := &network.CaptivePortalToken{
		Token:      "cpt_" + base64.RawURLEncoding.EncodeToString(tokenBytes),
		NetworkID:  networkID,
		JumpPeerID: jumpPeerID,
		PeerIP:     peerIP,
		CreatedAt:  now,
		ExpiresAt:  now.Add(10 * time.Minute),
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
	if err := s.AddCaptivePortalWhitelist(ctx, cpt.NetworkID, cpt.JumpPeerID, cpt.PeerIP); err != nil {
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

// AddCaptivePortalWhitelist adds a peer IP to the captive portal whitelist
func (s *Service) AddCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	if err := s.repo.AddCaptivePortalWhitelist(ctx, networkID, jumpPeerID, peerIP); err != nil {
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

	// Remove peers that are no longer active
	for _, ip := range whitelist {
		if !activePeerIPs[ip] {
			log.Info().
				Str("network_id", networkID).
				Str("jump_peer_id", jumpPeerID).
				Str("peer_ip", ip).
				Msg("removing disconnected peer from whitelist")

			if err := s.repo.RemoveCaptivePortalWhitelist(ctx, networkID, jumpPeerID, ip); err != nil {
				log.Error().Err(err).Str("peer_ip", ip).Msg("failed to remove peer from whitelist")
			}
		}
	}

	// Notify jump peer to update firewall rules
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// RemoveFromWhitelistOnEndpointChange removes a peer from whitelist when endpoint changes
func (s *Service) RemoveFromWhitelistOnEndpointChange(ctx context.Context, networkID string, peerIP string) error {
	// Get all jump peers in the network
	net, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}

	// Remove from whitelist for all jump peers
	for _, jumpPeer := range net.GetJumpServers() {
		whitelist, err := s.repo.GetCaptivePortalWhitelist(ctx, networkID, jumpPeer.ID)
		if err != nil {
			continue
		}

		// Check if this IP is in the whitelist
		found := false
		for _, ip := range whitelist {
			if ip == peerIP {
				found = true
				break
			}
		}

		if found {
			log.Info().
				Str("network_id", networkID).
				Str("jump_peer_id", jumpPeer.ID).
				Str("peer_ip", peerIP).
				Msg("removing peer from whitelist due to endpoint change")

			if err := s.repo.RemoveCaptivePortalWhitelist(ctx, networkID, jumpPeer.ID, peerIP); err != nil {
				log.Error().Err(err).Str("peer_ip", peerIP).Msg("failed to remove peer from whitelist")
			}
		}
	}

	// Notify jump peers to update firewall rules
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// filterQuarantinedPeers removes quarantined peers from the allowed peers list
// This ensures quarantined peers are completely disconnected from jump server WireGuard configs
func (s *Service) filterQuarantinedPeers(ctx context.Context, networkID string, allowedPeers []*network.Peer) []*network.Peer {
	// If group repository is not available, return all peers (no filtering)
	if s.groupRepo == nil {
		return allowedPeers
	}

	// Get quarantine group ID
	quarantineGroupID, err := s.getQuarantineGroupID(ctx, networkID)
	if err != nil {
		// If quarantine group doesn't exist, no peers are quarantined
		return allowedPeers
	}

	// Get the quarantine group directly
	quarantineGroup, err := s.groupRepo.GetGroup(ctx, networkID, quarantineGroupID)
	if err != nil {
		// If we can't get the quarantine group, return all peers (no filtering)
		return allowedPeers
	}

	// Build a map of quarantined peer IDs for quick lookup
	quarantinedPeerIDs := make(map[string]bool)
	for _, peerID := range quarantineGroup.PeerIDs {
		quarantinedPeerIDs[peerID] = true
	}

	// Filter out quarantined peers
	var filteredPeers []*network.Peer
	for _, peer := range allowedPeers {
		if !quarantinedPeerIDs[peer.ID] {
			filteredPeers = append(filteredPeers, peer)
		} else {
			log.Debug().
				Str("network_id", networkID).
				Str("peer_id", peer.ID).
				Str("peer_name", peer.Name).
				Msg("Excluding quarantined peer from jump server WireGuard config")
		}
	}

	return filteredPeers
}

// Security config operations

// GetSecurityConfig retrieves the security configuration for a network
func (s *Service) GetSecurityConfig(ctx context.Context, networkID string) (*network.SecurityConfig, error) {
	// Verify network exists
	_, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	config, err := s.repo.GetSecurityConfig(ctx, networkID)
	if err != nil {
		// If no config exists, return default config
		defaultConfig := network.DefaultSecurityConfig()
		defaultConfig.NetworkID = networkID
		return defaultConfig, nil
	}

	return config, nil
}

// UpdateSecurityConfig updates the security configuration for a network
func (s *Service) UpdateSecurityConfig(ctx context.Context, networkID string, req *network.SecurityConfigUpdateRequest) (*network.SecurityConfig, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid security config: %w", err)
	}

	// Verify network exists
	_, err := s.repo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	// Get existing config or create default
	config, err := s.repo.GetSecurityConfig(ctx, networkID)
	if err != nil {
		// If no config exists, create one with default values
		config = network.DefaultSecurityConfig()
		config.NetworkID = networkID
		if err := s.repo.CreateSecurityConfig(ctx, networkID, config); err != nil {
			return nil, fmt.Errorf("failed to create default security config: %w", err)
		}
	}

	// Update fields if provided
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.SessionConflictThreshold != nil {
		config.SessionConflictThreshold = *req.SessionConflictThreshold
	}
	if req.EndpointChangeThreshold != nil {
		config.EndpointChangeThreshold = *req.EndpointChangeThreshold
	}
	if req.MaxEndpointChangesPerDay != nil {
		config.MaxEndpointChangesPerDay = *req.MaxEndpointChangesPerDay
	}
	if req.PortChangeThreshold != nil {
		config.PortChangeThreshold = *req.PortChangeThreshold
	}
	if req.MaxPortChangesPerWindow != nil {
		config.MaxPortChangesPerWindow = *req.MaxPortChangesPerWindow
	}

	// Validate the updated config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid updated security config: %w", err)
	}

	if err := s.repo.UpdateSecurityConfig(ctx, networkID, config); err != nil {
		return nil, fmt.Errorf("failed to update security config: %w", err)
	}

	log.Info().
		Str("network_id", networkID).
		Bool("enabled", config.Enabled).
		Dur("session_conflict_threshold", config.SessionConflictThreshold).
		Dur("endpoint_change_threshold", config.EndpointChangeThreshold).
		Int("max_endpoint_changes_per_day", config.MaxEndpointChangesPerDay).
		Dur("port_change_threshold", config.PortChangeThreshold).
		Int("max_port_changes_per_window", config.MaxPortChangesPerWindow).
		Msg("Security config updated")

	return config, nil
}

// getSecurityConfigWithDefaults gets the security configuration for a network, returning defaults if not found
func (s *Service) getSecurityConfigWithDefaults(ctx context.Context, networkID string) *network.SecurityConfig {
	config, err := s.repo.GetSecurityConfig(ctx, networkID)
	if err != nil {
		// Return default config if not found or error
		return network.DefaultSecurityConfig()
	}

	return config
}
