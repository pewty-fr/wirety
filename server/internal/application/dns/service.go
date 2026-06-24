package dns

import (
	"context"
	"fmt"
	"time"

	"wirety/internal/domain/network"

	"github.com/google/uuid"
)

// WebSocketNotifier is an interface for notifying peers about config updates
type WebSocketNotifier interface {
	NotifyNetworkPeers(networkID string)
}

// DNSRecord represents a combined DNS record (peer or route-based).
//
// Dual-stack: a single record can carry both an IPv4 (IPAddress) and an IPv6
// (IPv6Address) value.  At least one is always non-empty.  For peer records
// IPv4 = peer.Address, IPv6 = peer.AddressV6.  For route-based records both
// fields come from the underlying DNSMapping (since migration 027).
type DNSRecord struct {
	Name        string `json:"name"`
	IPAddress   string `json:"ip_address,omitempty"`
	IPv6Address string `json:"ip_address_v6,omitempty"`
	FQDN        string `json:"fqdn"`
	Type        string `json:"type"` // "peer" or "route"
}

// Service implements the business logic for DNS mapping management
type Service struct {
	dnsRepo    network.DNSRepository
	routeRepo  network.RouteRepository
	peerRepo   network.Repository
	wsNotifier WebSocketNotifier
}

// NewService creates a new DNS service
func NewService(dnsRepo network.DNSRepository, routeRepo network.RouteRepository, peerRepo network.Repository) *Service {
	return &Service{
		dnsRepo:   dnsRepo,
		routeRepo: routeRepo,
		peerRepo:  peerRepo,
	}
}

// SetWebSocketNotifier sets the WebSocket notifier for the service
func (s *Service) SetWebSocketNotifier(notifier WebSocketNotifier) {
	s.wsNotifier = notifier
}

// CreateDNSMapping creates a new DNS mapping with IP validation within route CIDR.
// Dual-stack: each address is validated against the SAME-FAMILY CIDR on the
// route.  Submitting an IPv6 address on a route that has no IPv6 destination
// CIDR is a hard reject — there's no way that address could ever be reached.
func (s *Service) CreateDNSMapping(ctx context.Context, networkID, routeID string, req *network.DNSMappingCreateRequest) (*network.DNSMapping, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get route to validate IP is within CIDR
	route, err := s.routeRepo.GetRoute(ctx, networkID, routeID)
	if err != nil {
		return nil, fmt.Errorf("route not found: %w", err)
	}

	if req.IPAddress != "" {
		if route.DestinationCIDR == "" {
			return nil, fmt.Errorf("ip_address: route has no IPv4 destination CIDR")
		}
		if err := network.ValidateIPInCIDR(req.IPAddress, route.DestinationCIDR); err != nil {
			return nil, fmt.Errorf("ip_address validation failed: %w", err)
		}
	}
	if req.IPv6Address != "" {
		if route.DestinationCIDRv6 == "" {
			return nil, fmt.Errorf("ip_address_v6: route has no IPv6 destination CIDR")
		}
		if err := network.ValidateIPInCIDR(req.IPv6Address, route.DestinationCIDRv6); err != nil {
			return nil, fmt.Errorf("ip_address_v6 validation failed: %w", err)
		}
	}

	now := time.Now()
	mapping := &network.DNSMapping{
		ID:          uuid.New().String(),
		RouteID:     routeID,
		Name:        req.Name,
		IPAddress:   req.IPAddress,
		IPv6Address: req.IPv6Address,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.dnsRepo.CreateDNSMapping(ctx, routeID, mapping); err != nil {
		return nil, fmt.Errorf("failed to create DNS mapping: %w", err)
	}

	// Trigger DNS server updates via WebSocket
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(route.NetworkID)
	}

	return mapping, nil
}

// GetDNSMapping retrieves a DNS mapping by ID
func (s *Service) GetDNSMapping(ctx context.Context, routeID, mappingID string) (*network.DNSMapping, error) {
	mapping, err := s.dnsRepo.GetDNSMapping(ctx, routeID, mappingID)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS mapping: %w", err)
	}
	return mapping, nil
}

// UpdateDNSMapping updates an existing DNS mapping
func (s *Service) UpdateDNSMapping(ctx context.Context, networkID, routeID, mappingID string, req *network.DNSMappingUpdateRequest) (*network.DNSMapping, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing mapping
	mapping, err := s.dnsRepo.GetDNSMapping(ctx, routeID, mappingID)
	if err != nil {
		return nil, fmt.Errorf("DNS mapping not found: %w", err)
	}

	// Get route for validation
	route, err := s.routeRepo.GetRoute(ctx, networkID, routeID)
	if err != nil {
		return nil, fmt.Errorf("route not found: %w", err)
	}

	// Update fields
	if req.Name != "" {
		mapping.Name = req.Name
	}
	if req.IPAddress != "" {
		if route.DestinationCIDR == "" {
			return nil, fmt.Errorf("ip_address: route has no IPv4 destination CIDR")
		}
		if err := network.ValidateIPInCIDR(req.IPAddress, route.DestinationCIDR); err != nil {
			return nil, fmt.Errorf("ip_address validation failed: %w", err)
		}
		mapping.IPAddress = req.IPAddress
	}
	if req.IPv6Address != "" {
		if route.DestinationCIDRv6 == "" {
			return nil, fmt.Errorf("ip_address_v6: route has no IPv6 destination CIDR")
		}
		if err := network.ValidateIPInCIDR(req.IPv6Address, route.DestinationCIDRv6); err != nil {
			return nil, fmt.Errorf("ip_address_v6 validation failed: %w", err)
		}
		mapping.IPv6Address = req.IPv6Address
	}
	// Post-merge invariant: at least one family must remain set.
	if mapping.IPAddress == "" && mapping.IPv6Address == "" {
		return nil, fmt.Errorf("validation failed: at least one of ip_address or ip_address_v6 must remain set")
	}
	mapping.UpdatedAt = time.Now()

	if err := s.dnsRepo.UpdateDNSMapping(ctx, routeID, mapping); err != nil {
		return nil, fmt.Errorf("failed to update DNS mapping: %w", err)
	}

	// Trigger DNS server updates via WebSocket
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(route.NetworkID)
	}

	return mapping, nil
}

// DeleteDNSMapping deletes a DNS mapping
func (s *Service) DeleteDNSMapping(ctx context.Context, networkID, routeID, mappingID string) error {
	// Get route for network ID
	route, err := s.routeRepo.GetRoute(ctx, networkID, routeID)
	if err != nil {
		return fmt.Errorf("route not found: %w", err)
	}

	// Verify mapping exists
	_, err = s.dnsRepo.GetDNSMapping(ctx, routeID, mappingID)
	if err != nil {
		return fmt.Errorf("DNS mapping not found: %w", err)
	}

	if err := s.dnsRepo.DeleteDNSMapping(ctx, routeID, mappingID); err != nil {
		return fmt.Errorf("failed to delete DNS mapping: %w", err)
	}

	// Trigger DNS server updates via WebSocket
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(route.NetworkID)
	}

	return nil
}

// ListDNSMappings lists all DNS mappings for a route
func (s *Service) ListDNSMappings(ctx context.Context, networkID, routeID string) ([]*network.DNSMapping, error) {
	// Verify route exists
	_, err := s.routeRepo.GetRoute(ctx, networkID, routeID)
	if err != nil {
		return nil, fmt.Errorf("route not found: %w", err)
	}

	mappings, err := s.dnsRepo.ListDNSMappings(ctx, routeID)
	if err != nil {
		return nil, fmt.Errorf("failed to list DNS mappings: %w", err)
	}

	return mappings, nil
}

// GetNetworkDNSRecords combines peer and route DNS records
func (s *Service) GetNetworkDNSRecords(ctx context.Context, networkID string) ([]DNSRecord, error) {
	// Verify network exists
	net, err := s.peerRepo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	var records []DNSRecord

	// Get peer DNS records
	peers, err := s.peerRepo.ListPeers(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list peers: %w", err)
	}

	domainSuffix := net.DomainSuffix
	if domainSuffix == "" {
		domainSuffix = "internal"
	}

	for _, peer := range peers {
		fqdn := fmt.Sprintf("%s.%s.%s", peer.Name, net.Name, domainSuffix)
		records = append(records, DNSRecord{
			Name:        peer.Name,
			IPAddress:   peer.Address,
			IPv6Address: peer.AddressV6,
			FQDN:        fqdn,
			Type:        "peer",
		})
	}

	// Get route DNS records
	routeMappings, err := s.dnsRepo.GetNetworkDNSMappings(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network DNS mappings: %w", err)
	}

	// Iterate the mappings to build their FQDN.  The route the mapping
	// belongs to is no longer needed for the FQDN itself (the format is
	// <name>.<network>.<suffix>), but we still look it up to filter out
	// orphaned mappings whose route was deleted without a cascade.
	for _, mapping := range routeMappings {
		if _, err := s.routeRepo.GetRoute(ctx, networkID, mapping.RouteID); err != nil {
			// Skip if route not found
			continue
		}

		fqdn := mapping.GetFQDN(net)
		records = append(records, DNSRecord{
			Name:        mapping.Name,
			IPAddress:   mapping.IPAddress,
			IPv6Address: mapping.IPv6Address,
			FQDN:        fqdn,
			Type:        "route",
		})
	}

	return records, nil
}
