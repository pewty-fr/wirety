package dns

import (
	"context"
	"errors"
	"testing"
	"time"

	"wirety/internal/domain/network"
)

// Mock implementations for unit tests

type mockPeerRepository struct {
	networks map[string]*network.Network
	peers    map[string][]*network.Peer // networkID -> peers
}

func newMockPeerRepository() *mockPeerRepository {
	return &mockPeerRepository{
		networks: make(map[string]*network.Network),
		peers:    make(map[string][]*network.Peer),
	}
}

func (m *mockPeerRepository) GetNetwork(ctx context.Context, networkID string) (*network.Network, error) {
	net, exists := m.networks[networkID]
	if !exists {
		return nil, network.ErrNetworkNotFound
	}
	return net, nil
}

func (m *mockPeerRepository) ListPeers(ctx context.Context, networkID string) ([]*network.Peer, error) {
	peers, exists := m.peers[networkID]
	if !exists {
		return []*network.Peer{}, nil
	}
	return peers, nil
}

// Additional methods to satisfy network.Repository interface
func (m *mockPeerRepository) AddCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	return nil
}
func (m *mockPeerRepository) RemoveCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	return nil
}
func (m *mockPeerRepository) GetCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) ([]string, error) {
	return nil, nil
}
func (m *mockPeerRepository) ClearCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) error {
	return nil
}
func (m *mockPeerRepository) CreateCaptivePortalToken(ctx context.Context, token *network.CaptivePortalToken) error {
	return nil
}
func (m *mockPeerRepository) GetCaptivePortalToken(ctx context.Context, token string) (*network.CaptivePortalToken, error) {
	return nil, nil
}
func (m *mockPeerRepository) DeleteCaptivePortalToken(ctx context.Context, token string) error {
	return nil
}
func (m *mockPeerRepository) CleanupExpiredCaptivePortalTokens(ctx context.Context) error {
	return nil
}
func (m *mockPeerRepository) CreateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	return nil
}
func (m *mockPeerRepository) GetACL(ctx context.Context, networkID string) (*network.ACL, error) {
	return nil, nil
}
func (m *mockPeerRepository) UpdateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	return nil
}
func (m *mockPeerRepository) CreateConnection(ctx context.Context, networkID string, conn *network.PeerConnection) error {
	return nil
}
func (m *mockPeerRepository) GetConnection(ctx context.Context, networkID, peer1ID, peer2ID string) (*network.PeerConnection, error) {
	return nil, nil
}
func (m *mockPeerRepository) ListConnections(ctx context.Context, networkID string) ([]*network.PeerConnection, error) {
	return nil, nil
}
func (m *mockPeerRepository) DeleteConnection(ctx context.Context, networkID, peer1ID, peer2ID string) error {
	return nil
}
func (m *mockPeerRepository) CreateOrUpdateSession(ctx context.Context, networkID string, session *network.AgentSession) error {
	return nil
}
func (m *mockPeerRepository) GetSession(ctx context.Context, networkID, peerID string) (*network.AgentSession, error) {
	return nil, nil
}
func (m *mockPeerRepository) GetActiveSessionsForPeer(ctx context.Context, networkID, peerID string) ([]*network.AgentSession, error) {
	return nil, nil
}
func (m *mockPeerRepository) DeleteSession(ctx context.Context, networkID, sessionID string) error {
	return nil
}
func (m *mockPeerRepository) ListSessions(ctx context.Context, networkID string) ([]*network.AgentSession, error) {
	return nil, nil
}
func (m *mockPeerRepository) RecordEndpointChange(ctx context.Context, networkID string, change *network.EndpointChange) error {
	return nil
}
func (m *mockPeerRepository) GetEndpointChanges(ctx context.Context, networkID, peerID string, since time.Time) ([]*network.EndpointChange, error) {
	return nil, nil
}
func (m *mockPeerRepository) DeleteEndpointChanges(ctx context.Context, networkID, peerID string) error {
	return nil
}
func (m *mockPeerRepository) CreateSecurityIncident(ctx context.Context, incident *network.SecurityIncident) error {
	return nil
}
func (m *mockPeerRepository) GetSecurityIncident(ctx context.Context, incidentID string) (*network.SecurityIncident, error) {
	return nil, nil
}
func (m *mockPeerRepository) ListSecurityIncidents(ctx context.Context, resolved *bool) ([]*network.SecurityIncident, error) {
	return nil, nil
}
func (m *mockPeerRepository) ListSecurityIncidentsByNetwork(ctx context.Context, networkID string, resolved *bool) ([]*network.SecurityIncident, error) {
	return nil, nil
}
func (m *mockPeerRepository) ResolveSecurityIncident(ctx context.Context, incidentID, resolvedBy string) error {
	return nil
}
func (m *mockPeerRepository) CreateSecurityConfig(ctx context.Context, networkID string, config *network.SecurityConfig) error {
	return nil
}
func (m *mockPeerRepository) GetSecurityConfig(ctx context.Context, networkID string) (*network.SecurityConfig, error) {
	return nil, nil
}
func (m *mockPeerRepository) UpdateSecurityConfig(ctx context.Context, networkID string, config *network.SecurityConfig) error {
	return nil
}
func (m *mockPeerRepository) DeleteSecurityConfig(ctx context.Context, networkID string) error {
	return nil
}

// Stub methods for interface compliance
func (m *mockPeerRepository) CreateNetwork(ctx context.Context, net *network.Network) error {
	return nil
}
func (m *mockPeerRepository) UpdateNetwork(ctx context.Context, net *network.Network) error {
	return nil
}
func (m *mockPeerRepository) DeleteNetwork(ctx context.Context, networkID string) error {
	return nil
}
func (m *mockPeerRepository) ListNetworks(ctx context.Context) ([]*network.Network, error) {
	return nil, nil
}
func (m *mockPeerRepository) CreatePeer(ctx context.Context, networkID string, peer *network.Peer) error {
	return nil
}
func (m *mockPeerRepository) GetPeer(ctx context.Context, networkID, peerID string) (*network.Peer, error) {
	return nil, nil
}
func (m *mockPeerRepository) GetPeerByToken(ctx context.Context, token string) (string, *network.Peer, error) {
	return "", nil, nil
}
func (m *mockPeerRepository) UpdatePeer(ctx context.Context, networkID string, peer *network.Peer) error {
	return nil
}
func (m *mockPeerRepository) DeletePeer(ctx context.Context, networkID, peerID string) error {
	return nil
}

type mockWebSocketNotifier struct {
	notifiedNetworks []string
}

func (m *mockWebSocketNotifier) NotifyNetworkPeers(networkID string) {
	m.notifiedNetworks = append(m.notifiedNetworks, networkID)
}

func TestService_CreateDNSMapping(t *testing.T) {
	tests := []struct {
		name        string
		networkID   string
		routeID     string
		request     *network.DNSMappingCreateRequest
		setupRoute  *network.Route
		expectError bool
		errorType   error
	}{
		{
			name:      "successful creation",
			networkID: "net1",
			routeID:   "route1",
			request: &network.DNSMappingCreateRequest{
				Name:      "server1",
				IPAddress: "192.168.1.10",
			},
			setupRoute: &network.Route{
				ID:              "route1",
				NetworkID:       "net1",
				Name:            "test-route",
				DestinationCIDR: "192.168.1.0/24",
			},
			expectError: false,
		},
		{
			name:      "invalid request - empty name",
			networkID: "net1",
			routeID:   "route1",
			request: &network.DNSMappingCreateRequest{
				Name:      "",
				IPAddress: "192.168.1.10",
			},
			setupRoute: &network.Route{
				ID:              "route1",
				NetworkID:       "net1",
				Name:            "test-route",
				DestinationCIDR: "192.168.1.0/24",
			},
			expectError: true,
		},
		{
			name:      "route not found",
			networkID: "net1",
			routeID:   "nonexistent",
			request: &network.DNSMappingCreateRequest{
				Name:      "server1",
				IPAddress: "192.168.1.10",
			},
			setupRoute:  nil,
			expectError: true,
			errorType:   network.ErrRouteNotFound,
		},
		{
			name:      "IP not in CIDR",
			networkID: "net1",
			routeID:   "route1",
			request: &network.DNSMappingCreateRequest{
				Name:      "server1",
				IPAddress: "10.0.0.1", // Not in 192.168.1.0/24
			},
			setupRoute: &network.Route{
				ID:              "route1",
				NetworkID:       "net1",
				Name:            "test-route",
				DestinationCIDR: "192.168.1.0/24",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dnsRepo := newMockDNSRepository()
			routeRepo := newMockRouteRepository()
			peerRepo := newMockPeerRepository()
			wsNotifier := &mockWebSocketNotifier{}

			service := NewService(dnsRepo, routeRepo, peerRepo)
			service.SetWebSocketNotifier(wsNotifier)

			// Setup route if provided
			if tt.setupRoute != nil {
				err := routeRepo.CreateRoute(context.Background(), tt.networkID, tt.setupRoute)
				if err != nil {
					t.Fatalf("Failed to setup route: %v", err)
				}
			}

			// Execute test
			result, err := service.CreateDNSMapping(context.Background(), tt.networkID, tt.routeID, tt.request)

			// Verify expectations
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorType != nil && !errors.Is(err, tt.errorType) {
					t.Errorf("Expected error type %v, got %v", tt.errorType, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("Expected result but got nil")
				return
			}

			// Verify result properties
			if result.Name != tt.request.Name {
				t.Errorf("Expected name %s, got %s", tt.request.Name, result.Name)
			}
			if result.IPAddress != tt.request.IPAddress {
				t.Errorf("Expected IP %s, got %s", tt.request.IPAddress, result.IPAddress)
			}
			if result.RouteID != tt.routeID {
				t.Errorf("Expected route ID %s, got %s", tt.routeID, result.RouteID)
			}

			// Verify WebSocket notification was sent
			if len(wsNotifier.notifiedNetworks) != 1 || wsNotifier.notifiedNetworks[0] != tt.networkID {
				t.Errorf("Expected WebSocket notification for network %s", tt.networkID)
			}
		})
	}
}

func TestService_GetDNSMapping(t *testing.T) {
	dnsRepo := newMockDNSRepository()
	routeRepo := newMockRouteRepository()
	peerRepo := newMockPeerRepository()

	service := NewService(dnsRepo, routeRepo, peerRepo)

	// Create a test mapping
	mapping := &network.DNSMapping{
		ID:        "mapping1",
		RouteID:   "route1",
		Name:      "server1",
		IPAddress: "192.168.1.10",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	dnsRepo.mappings["mapping1"] = mapping

	// Test successful retrieval
	result, err := service.GetDNSMapping(context.Background(), "route1", "mapping1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != mapping {
		t.Error("Expected to get the same mapping")
	}

	// Test non-existent mapping
	_, err = service.GetDNSMapping(context.Background(), "route1", "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent mapping")
	}
}

func TestService_UpdateDNSMapping(t *testing.T) {
	tests := []struct {
		name         string
		networkID    string
		routeID      string
		mappingID    string
		request      *network.DNSMappingUpdateRequest
		setupRoute   *network.Route
		setupMapping *network.DNSMapping
		expectError  bool
	}{
		{
			name:      "successful update - name only",
			networkID: "net1",
			routeID:   "route1",
			mappingID: "mapping1",
			request: &network.DNSMappingUpdateRequest{
				Name: "updated-server",
			},
			setupRoute: &network.Route{
				ID:              "route1",
				NetworkID:       "net1",
				Name:            "test-route",
				DestinationCIDR: "192.168.1.0/24",
			},
			setupMapping: &network.DNSMapping{
				ID:        "mapping1",
				RouteID:   "route1",
				Name:      "server1",
				IPAddress: "192.168.1.10",
			},
			expectError: false,
		},
		{
			name:      "successful update - IP only",
			networkID: "net1",
			routeID:   "route1",
			mappingID: "mapping1",
			request: &network.DNSMappingUpdateRequest{
				IPAddress: "192.168.1.20",
			},
			setupRoute: &network.Route{
				ID:              "route1",
				NetworkID:       "net1",
				Name:            "test-route",
				DestinationCIDR: "192.168.1.0/24",
			},
			setupMapping: &network.DNSMapping{
				ID:        "mapping1",
				RouteID:   "route1",
				Name:      "server1",
				IPAddress: "192.168.1.10",
			},
			expectError: false,
		},
		{
			name:      "mapping not found",
			networkID: "net1",
			routeID:   "route1",
			mappingID: "nonexistent",
			request: &network.DNSMappingUpdateRequest{
				Name: "updated-server",
			},
			setupRoute: &network.Route{
				ID:              "route1",
				NetworkID:       "net1",
				Name:            "test-route",
				DestinationCIDR: "192.168.1.0/24",
			},
			setupMapping: nil,
			expectError:  true,
		},
		{
			name:      "IP not in CIDR",
			networkID: "net1",
			routeID:   "route1",
			mappingID: "mapping1",
			request: &network.DNSMappingUpdateRequest{
				IPAddress: "10.0.0.1", // Not in 192.168.1.0/24
			},
			setupRoute: &network.Route{
				ID:              "route1",
				NetworkID:       "net1",
				Name:            "test-route",
				DestinationCIDR: "192.168.1.0/24",
			},
			setupMapping: &network.DNSMapping{
				ID:        "mapping1",
				RouteID:   "route1",
				Name:      "server1",
				IPAddress: "192.168.1.10",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dnsRepo := newMockDNSRepository()
			routeRepo := newMockRouteRepository()
			peerRepo := newMockPeerRepository()
			wsNotifier := &mockWebSocketNotifier{}

			service := NewService(dnsRepo, routeRepo, peerRepo)
			service.SetWebSocketNotifier(wsNotifier)

			// Setup route
			if tt.setupRoute != nil {
				err := routeRepo.CreateRoute(context.Background(), tt.networkID, tt.setupRoute)
				if err != nil {
					t.Fatalf("Failed to setup route: %v", err)
				}
			}

			// Setup mapping
			if tt.setupMapping != nil {
				dnsRepo.mappings[tt.mappingID] = tt.setupMapping
			}

			// Execute test
			result, err := service.UpdateDNSMapping(context.Background(), tt.networkID, tt.routeID, tt.mappingID, tt.request)

			// Verify expectations
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("Expected result but got nil")
				return
			}

			// Verify updates were applied
			if tt.request.Name != "" && result.Name != tt.request.Name {
				t.Errorf("Expected name %s, got %s", tt.request.Name, result.Name)
			}
			if tt.request.IPAddress != "" && result.IPAddress != tt.request.IPAddress {
				t.Errorf("Expected IP %s, got %s", tt.request.IPAddress, result.IPAddress)
			}

			// Verify WebSocket notification was sent
			if len(wsNotifier.notifiedNetworks) != 1 || wsNotifier.notifiedNetworks[0] != tt.networkID {
				t.Errorf("Expected WebSocket notification for network %s", tt.networkID)
			}
		})
	}
}

func TestService_DeleteDNSMapping(t *testing.T) {
	dnsRepo := newMockDNSRepository()
	routeRepo := newMockRouteRepository()
	peerRepo := newMockPeerRepository()
	wsNotifier := &mockWebSocketNotifier{}

	service := NewService(dnsRepo, routeRepo, peerRepo)
	service.SetWebSocketNotifier(wsNotifier)

	// Setup route
	route := &network.Route{
		ID:              "route1",
		NetworkID:       "net1",
		Name:            "test-route",
		DestinationCIDR: "192.168.1.0/24",
	}
	err := routeRepo.CreateRoute(context.Background(), "net1", route)
	if err != nil {
		t.Fatalf("Failed to setup route: %v", err)
	}

	// Setup mapping
	mapping := &network.DNSMapping{
		ID:        "mapping1",
		RouteID:   "route1",
		Name:      "server1",
		IPAddress: "192.168.1.10",
	}
	dnsRepo.mappings["mapping1"] = mapping

	// Test successful deletion
	err = service.DeleteDNSMapping(context.Background(), "net1", "route1", "mapping1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify mapping was deleted
	_, exists := dnsRepo.mappings["mapping1"]
	if exists {
		t.Error("Expected mapping to be deleted")
	}

	// Verify WebSocket notification was sent
	if len(wsNotifier.notifiedNetworks) != 1 || wsNotifier.notifiedNetworks[0] != "net1" {
		t.Error("Expected WebSocket notification for network net1")
	}

	// Test deleting non-existent mapping
	err = service.DeleteDNSMapping(context.Background(), "net1", "route1", "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent mapping")
	}
}

func TestService_ListDNSMappings(t *testing.T) {
	dnsRepo := newMockDNSRepository()
	routeRepo := newMockRouteRepository()
	peerRepo := newMockPeerRepository()

	service := NewService(dnsRepo, routeRepo, peerRepo)

	// Setup route
	route := &network.Route{
		ID:              "route1",
		NetworkID:       "net1",
		Name:            "test-route",
		DestinationCIDR: "192.168.1.0/24",
	}
	err := routeRepo.CreateRoute(context.Background(), "net1", route)
	if err != nil {
		t.Fatalf("Failed to setup route: %v", err)
	}

	// Setup mappings
	mapping1 := &network.DNSMapping{
		ID:        "mapping1",
		RouteID:   "route1",
		Name:      "server1",
		IPAddress: "192.168.1.10",
	}
	mapping2 := &network.DNSMapping{
		ID:        "mapping2",
		RouteID:   "route1",
		Name:      "server2",
		IPAddress: "192.168.1.11",
	}
	mapping3 := &network.DNSMapping{
		ID:        "mapping3",
		RouteID:   "route2", // Different route
		Name:      "server3",
		IPAddress: "192.168.2.10",
	}

	dnsRepo.mappings["mapping1"] = mapping1
	dnsRepo.mappings["mapping2"] = mapping2
	dnsRepo.mappings["mapping3"] = mapping3

	// Test listing mappings for route1
	mappings, err := service.ListDNSMappings(context.Background(), "net1", "route1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(mappings) != 2 {
		t.Errorf("Expected 2 mappings, got %d", len(mappings))
	}

	// Verify correct mappings are returned
	mappingMap := make(map[string]*network.DNSMapping)
	for _, mapping := range mappings {
		mappingMap[mapping.ID] = mapping
	}

	if mappingMap["mapping1"] == nil || mappingMap["mapping2"] == nil {
		t.Error("Expected mapping1 and mapping2 to be returned")
	}
	if mappingMap["mapping3"] != nil {
		t.Error("Expected mapping3 to not be returned (different route)")
	}

	// Test listing for non-existent route
	_, err = service.ListDNSMappings(context.Background(), "net1", "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent route")
	}
}

func TestService_GetNetworkDNSRecords(t *testing.T) {
	dnsRepo := newMockDNSRepository()
	routeRepo := newMockRouteRepository()
	peerRepo := newMockPeerRepository()

	service := NewService(dnsRepo, routeRepo, peerRepo)

	// Setup network
	testNetwork := &network.Network{
		ID:           "net1",
		Name:         "testnet",
		DomainSuffix: "example.com",
	}
	peerRepo.networks["net1"] = testNetwork

	// Setup peers
	peers := []*network.Peer{
		{
			ID:      "peer1",
			Name:    "client1",
			Address: "10.0.0.10",
		},
		{
			ID:      "peer2",
			Name:    "client2",
			Address: "10.0.0.11",
		},
	}
	peerRepo.peers["net1"] = peers

	// Setup route
	route := &network.Route{
		ID:              "route1",
		NetworkID:       "net1",
		Name:            "backend",
		DestinationCIDR: "192.168.1.0/24",
		DomainSuffix:    "example.com",
	}
	routeRepo.routes["route1"] = route

	// Setup DNS mapping
	mapping := &network.DNSMapping{
		ID:        "mapping1",
		RouteID:   "route1",
		Name:      "api",
		IPAddress: "192.168.1.10",
	}
	dnsRepo.mappings["mapping1"] = mapping

	// Test getting DNS records
	records, err := service.GetNetworkDNSRecords(context.Background(), "net1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(records) != 3 { // 2 peers + 1 route mapping
		t.Errorf("Expected 3 DNS records, got %d", len(records))
	}

	// Verify peer records
	peerRecords := 0
	routeRecords := 0
	for _, record := range records {
		if record.Type == "peer" {
			peerRecords++
			expectedFQDN := record.Name + ".testnet.example.com"
			if record.FQDN != expectedFQDN {
				t.Errorf("Expected peer FQDN %s, got %s", expectedFQDN, record.FQDN)
			}
		} else if record.Type == "route" {
			routeRecords++
			expectedFQDN := "api.backend.example.com"
			if record.FQDN != expectedFQDN {
				t.Errorf("Expected route FQDN %s, got %s", expectedFQDN, record.FQDN)
			}
		}
	}

	if peerRecords != 2 {
		t.Errorf("Expected 2 peer records, got %d", peerRecords)
	}
	if routeRecords != 1 {
		t.Errorf("Expected 1 route record, got %d", routeRecords)
	}

	// Test with network using default domain suffix
	networkDefault := &network.Network{
		ID:           "net2",
		Name:         "defaultnet",
		DomainSuffix: "", // Should default to "internal"
	}
	peerRepo.networks["net2"] = networkDefault
	peerRepo.peers["net2"] = []*network.Peer{
		{
			ID:      "peer3",
			Name:    "client3",
			Address: "10.0.0.12",
		},
	}

	records, err = service.GetNetworkDNSRecords(context.Background(), "net2")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 DNS record, got %d", len(records))
	}

	expectedFQDN := "client3.defaultnet.internal"
	if records[0].FQDN != expectedFQDN {
		t.Errorf("Expected FQDN %s, got %s", expectedFQDN, records[0].FQDN)
	}

	// Test non-existent network
	_, err = service.GetNetworkDNSRecords(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent network")
	}
}
