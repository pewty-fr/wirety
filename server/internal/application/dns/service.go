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

// DNSRecord represents a combined DNS record (peer or route-based)
type DNSRecord struct {
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	FQDN      string `json:"fqdn"`
	Type      string `json:"type"` // "peer" or "route"
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

// CreateDNSMapping creates a new DNS mapping with IP validation within route CIDR
func (s *Service) CreateDNSMapping(ctx context.Context, routeID string, req *network.DNSMappingCreateRequest) (*network.DNSMapping, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get route to validate IP is within CIDR
	route, err := s.routeRepo.GetRoute(ctx, "", routeID)
	if err != nil {
		return nil, fmt.Errorf("route not found: %w", err)
	}

	// Validate IP is within route's CIDR
	if err := network.ValidateIPInCIDR(req.IPAddress, route.DestinationCIDR); err != nil {
		return nil, fmt.Errorf("IP validation failed: %w", err)
	}

	now := time.Now()
	mapping := &network.DNSMapping{
		ID:        uuid.New().String(),
		RouteID:   routeID,
		Name:      req.Name,
		IPAddress: req.IPAddress,
		CreatedAt: now,
		UpdatedAt: now,
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
func (s *Service) UpdateDNSMapping(ctx context.Context, routeID, mappingID string, req *network.DNSMappingUpdateRequest) (*network.DNSMapping, error) {
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
	route, err := s.routeRepo.GetRoute(ctx, "", routeID)
	if err != nil {
		return nil, fmt.Errorf("route not found: %w", err)
	}

	// Update fields
	if req.Name != "" {
		mapping.Name = req.Name
	}
	if req.IPAddress != "" {
		// Validate new IP is within route's CIDR
		if err := network.ValidateIPInCIDR(req.IPAddress, route.DestinationCIDR); err != nil {
			return nil, fmt.Errorf("IP validation failed: %w", err)
		}
		mapping.IPAddress = req.IPAddress
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
func (s *Service) DeleteDNSMapping(ctx context.Context, routeID, mappingID string) error {
	// Get route for network ID
	route, err := s.routeRepo.GetRoute(ctx, "", routeID)
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
func (s *Service) ListDNSMappings(ctx context.Context, routeID string) ([]*network.DNSMapping, error) {
	// Verify route exists
	_, err := s.routeRepo.GetRoute(ctx, "", routeID)
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
			Name:      peer.Name,
			IPAddress: peer.Address,
			FQDN:      fqdn,
			Type:      "peer",
		})
	}

	// Get route DNS records
	routeMappings, err := s.dnsRepo.GetNetworkDNSMappings(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network DNS mappings: %w", err)
	}

	// For each mapping, get the route to build FQDN
	for _, mapping := range routeMappings {
		route, err := s.routeRepo.GetRoute(ctx, networkID, mapping.RouteID)
		if err != nil {
			// Skip if route not found
			continue
		}

		fqdn := mapping.GetFQDN(route)
		records = append(records, DNSRecord{
			Name:      mapping.Name,
			IPAddress: mapping.IPAddress,
			FQDN:      fqdn,
			Type:      "route",
		})
	}

	return records, nil
}
