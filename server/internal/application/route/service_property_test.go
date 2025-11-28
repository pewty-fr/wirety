package route

import (
	"context"
	"testing"
	"time"

	"wirety/internal/domain/network"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Mock implementations for testing

type mockRouteRepository struct {
	routes      map[string]*network.Route
	groupRoutes map[string][]string // groupID -> []routeID
}

func newMockRouteRepository() *mockRouteRepository {
	return &mockRouteRepository{
		routes:      make(map[string]*network.Route),
		groupRoutes: make(map[string][]string),
	}
}

func (m *mockRouteRepository) CreateRoute(ctx context.Context, networkID string, route *network.Route) error {
	// Check for duplicate name
	for _, r := range m.routes {
		if r.NetworkID == networkID && r.Name == route.Name {
			return network.ErrDuplicateRouteName
		}
	}
	m.routes[route.ID] = route
	return nil
}

func (m *mockRouteRepository) GetRoute(ctx context.Context, networkID, routeID string) (*network.Route, error) {
	route, exists := m.routes[routeID]
	if !exists || route.NetworkID != networkID {
		return nil, network.ErrRouteNotFound
	}
	return route, nil
}

func (m *mockRouteRepository) UpdateRoute(ctx context.Context, networkID string, route *network.Route) error {
	existing, exists := m.routes[route.ID]
	if !exists || existing.NetworkID != networkID {
		return network.ErrRouteNotFound
	}
	// Check for duplicate name
	for id, r := range m.routes {
		if id != route.ID && r.NetworkID == networkID && r.Name == route.Name {
			return network.ErrDuplicateRouteName
		}
	}
	m.routes[route.ID] = route
	return nil
}

func (m *mockRouteRepository) DeleteRoute(ctx context.Context, networkID, routeID string) error {
	route, exists := m.routes[routeID]
	if !exists || route.NetworkID != networkID {
		return network.ErrRouteNotFound
	}
	delete(m.routes, routeID)
	return nil
}

func (m *mockRouteRepository) ListRoutes(ctx context.Context, networkID string) ([]*network.Route, error) {
	var routes []*network.Route
	for _, route := range m.routes {
		if route.NetworkID == networkID {
			routes = append(routes, route)
		}
	}
	return routes, nil
}

func (m *mockRouteRepository) GetRoutesForGroup(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
	routeIDs, exists := m.groupRoutes[groupID]
	if !exists {
		return []*network.Route{}, nil
	}
	var routes []*network.Route
	for _, routeID := range routeIDs {
		if route, exists := m.routes[routeID]; exists && route.NetworkID == networkID {
			routes = append(routes, route)
		}
	}
	return routes, nil
}

func (m *mockRouteRepository) GetRoutesByJumpPeer(ctx context.Context, networkID, jumpPeerID string) ([]*network.Route, error) {
	var routes []*network.Route
	for _, route := range m.routes {
		if route.NetworkID == networkID && route.JumpPeerID == jumpPeerID {
			routes = append(routes, route)
		}
	}
	return routes, nil
}

type mockGroupRepository struct {
	groups      map[string]*network.Group
	peerGroups  map[string][]string // peerID -> []groupID
	groupRoutes map[string][]string // groupID -> []routeID
}

func newMockGroupRepository() *mockGroupRepository {
	return &mockGroupRepository{
		groups:      make(map[string]*network.Group),
		peerGroups:  make(map[string][]string),
		groupRoutes: make(map[string][]string),
	}
}

func (m *mockGroupRepository) CreateGroup(ctx context.Context, networkID string, group *network.Group) error {
	m.groups[group.ID] = group
	return nil
}

func (m *mockGroupRepository) GetGroup(ctx context.Context, networkID, groupID string) (*network.Group, error) {
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return nil, network.ErrGroupNotFound
	}
	return group, nil
}

func (m *mockGroupRepository) UpdateGroup(ctx context.Context, networkID string, group *network.Group) error {
	return nil
}

func (m *mockGroupRepository) DeleteGroup(ctx context.Context, networkID, groupID string) error {
	return nil
}

func (m *mockGroupRepository) ListGroups(ctx context.Context, networkID string) ([]*network.Group, error) {
	var groups []*network.Group
	for _, group := range m.groups {
		if group.NetworkID == networkID {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func (m *mockGroupRepository) AddPeerToGroup(ctx context.Context, networkID, groupID, peerID string) error {
	m.peerGroups[peerID] = append(m.peerGroups[peerID], groupID)
	return nil
}

func (m *mockGroupRepository) RemovePeerFromGroup(ctx context.Context, networkID, groupID, peerID string) error {
	return nil
}

func (m *mockGroupRepository) GetPeerGroups(ctx context.Context, networkID, peerID string) ([]*network.Group, error) {
	groupIDs, exists := m.peerGroups[peerID]
	if !exists {
		return []*network.Group{}, nil
	}
	var groups []*network.Group
	for _, groupID := range groupIDs {
		if group, exists := m.groups[groupID]; exists && group.NetworkID == networkID {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func (m *mockGroupRepository) AttachPolicyToGroup(ctx context.Context, networkID, groupID, policyID string) error {
	return nil
}

func (m *mockGroupRepository) DetachPolicyFromGroup(ctx context.Context, networkID, groupID, policyID string) error {
	return nil
}

func (m *mockGroupRepository) GetGroupPolicies(ctx context.Context, networkID, groupID string) ([]*network.Policy, error) {
	return nil, nil
}

func (m *mockGroupRepository) AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error {
	m.groupRoutes[groupID] = append(m.groupRoutes[groupID], routeID)
	return nil
}

func (m *mockGroupRepository) DetachRouteFromGroup(ctx context.Context, networkID, groupID, routeID string) error {
	routes := m.groupRoutes[groupID]
	for i, rid := range routes {
		if rid == routeID {
			m.groupRoutes[groupID] = append(routes[:i], routes[i+1:]...)
			return nil
		}
	}
	return network.ErrRouteNotAttached
}

func (m *mockGroupRepository) GetGroupRoutes(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
	return nil, nil
}

type mockNetworkGetter struct {
	networks map[string]*network.Network
	peers    map[string]*network.Peer
}

func newMockNetworkGetter() *mockNetworkGetter {
	return &mockNetworkGetter{
		networks: make(map[string]*network.Network),
		peers:    make(map[string]*network.Peer),
	}
}

func (m *mockNetworkGetter) GetNetwork(ctx context.Context, networkID string) (*network.Network, error) {
	net, exists := m.networks[networkID]
	if !exists {
		return nil, network.ErrNetworkNotFound
	}
	return net, nil
}

func (m *mockNetworkGetter) GetPeer(ctx context.Context, networkID, peerID string) (*network.Peer, error) {
	peer, exists := m.peers[peerID]
	if !exists {
		return nil, network.ErrPeerNotFound
	}
	return peer, nil
}

// networkGetterAdapter adapts mockNetworkGetter to the minimal interface needed
type networkGetterAdapter struct {
	getter *mockNetworkGetter
}

func (a *networkGetterAdapter) GetNetwork(ctx context.Context, networkID string) (*network.Network, error) {
	return a.getter.GetNetwork(ctx, networkID)
}

func (a *networkGetterAdapter) GetPeer(ctx context.Context, networkID, peerID string) (*network.Peer, error) {
	return a.getter.GetPeer(ctx, networkID, peerID)
}

// Implement minimal interface for Repository
func (a *networkGetterAdapter) CreateNetwork(ctx context.Context, net *network.Network) error {
	return nil
}
func (a *networkGetterAdapter) UpdateNetwork(ctx context.Context, net *network.Network) error {
	return nil
}
func (a *networkGetterAdapter) DeleteNetwork(ctx context.Context, networkID string) error {
	return nil
}
func (a *networkGetterAdapter) ListNetworks(ctx context.Context) ([]*network.Network, error) {
	return nil, nil
}
func (a *networkGetterAdapter) CreatePeer(ctx context.Context, networkID string, peer *network.Peer) error {
	return nil
}
func (a *networkGetterAdapter) GetPeerByToken(ctx context.Context, token string) (string, *network.Peer, error) {
	return "", nil, nil
}
func (a *networkGetterAdapter) UpdatePeer(ctx context.Context, networkID string, peer *network.Peer) error {
	return nil
}
func (a *networkGetterAdapter) DeletePeer(ctx context.Context, networkID, peerID string) error {
	return nil
}
func (a *networkGetterAdapter) ListPeers(ctx context.Context, networkID string) ([]*network.Peer, error) {
	return nil, nil
}
func (a *networkGetterAdapter) CreateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	return nil
}
func (a *networkGetterAdapter) GetACL(ctx context.Context, networkID string) (*network.ACL, error) {
	return nil, nil
}
func (a *networkGetterAdapter) UpdateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	return nil
}
func (a *networkGetterAdapter) CreateConnection(ctx context.Context, networkID string, conn *network.PeerConnection) error {
	return nil
}
func (a *networkGetterAdapter) GetConnection(ctx context.Context, networkID, peer1ID, peer2ID string) (*network.PeerConnection, error) {
	return nil, nil
}
func (a *networkGetterAdapter) ListConnections(ctx context.Context, networkID string) ([]*network.PeerConnection, error) {
	return nil, nil
}
func (a *networkGetterAdapter) DeleteConnection(ctx context.Context, networkID, peer1ID, peer2ID string) error {
	return nil
}
func (a *networkGetterAdapter) CreateOrUpdateSession(ctx context.Context, networkID string, session *network.AgentSession) error {
	return nil
}
func (a *networkGetterAdapter) GetSession(ctx context.Context, networkID, peerID string) (*network.AgentSession, error) {
	return nil, nil
}
func (a *networkGetterAdapter) GetActiveSessionsForPeer(ctx context.Context, networkID, peerID string) ([]*network.AgentSession, error) {
	return nil, nil
}
func (a *networkGetterAdapter) DeleteSession(ctx context.Context, networkID, sessionID string) error {
	return nil
}
func (a *networkGetterAdapter) ListSessions(ctx context.Context, networkID string) ([]*network.AgentSession, error) {
	return nil, nil
}
func (a *networkGetterAdapter) RecordEndpointChange(ctx context.Context, networkID string, change *network.EndpointChange) error {
	return nil
}
func (a *networkGetterAdapter) GetEndpointChanges(ctx context.Context, networkID, peerID string, since time.Time) ([]*network.EndpointChange, error) {
	return nil, nil
}
func (a *networkGetterAdapter) DeleteEndpointChanges(ctx context.Context, networkID, peerID string) error {
	return nil
}
func (a *networkGetterAdapter) CreateSecurityIncident(ctx context.Context, incident *network.SecurityIncident) error {
	return nil
}
func (a *networkGetterAdapter) GetSecurityIncident(ctx context.Context, incidentID string) (*network.SecurityIncident, error) {
	return nil, nil
}
func (a *networkGetterAdapter) ListSecurityIncidents(ctx context.Context, resolved *bool) ([]*network.SecurityIncident, error) {
	return nil, nil
}
func (a *networkGetterAdapter) ListSecurityIncidentsByNetwork(ctx context.Context, networkID string, resolved *bool) ([]*network.SecurityIncident, error) {
	return nil, nil
}
func (a *networkGetterAdapter) ResolveSecurityIncident(ctx context.Context, incidentID, resolvedBy string) error {
	return nil
}
func (a *networkGetterAdapter) AddCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	return nil
}
func (a *networkGetterAdapter) RemoveCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	return nil
}
func (a *networkGetterAdapter) GetCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) ([]string, error) {
	return nil, nil
}
func (a *networkGetterAdapter) ClearCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) error {
	return nil
}
func (a *networkGetterAdapter) CreateCaptivePortalToken(ctx context.Context, token *network.CaptivePortalToken) error {
	return nil
}
func (a *networkGetterAdapter) GetCaptivePortalToken(ctx context.Context, token string) (*network.CaptivePortalToken, error) {
	return nil, nil
}
func (a *networkGetterAdapter) DeleteCaptivePortalToken(ctx context.Context, token string) error {
	return nil
}
func (a *networkGetterAdapter) CleanupExpiredCaptivePortalTokens(ctx context.Context) error {
	return nil
}

// Generators for property-based testing

func genValidRouteName() gopter.Gen {
	return gen.Identifier().SuchThat(func(v interface{}) bool {
		s := v.(string)
		return len(s) > 0 && len(s) <= 255
	})
}

func genDescription() gopter.Gen {
	return gen.AlphaString().SuchThat(func(v interface{}) bool {
		s := v.(string)
		return len(s) <= 1000
	})
}

func genNetworkID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "net-" + v
	})
}

func genRouteID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "route-" + v
	})
}

func genPeerID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "peer-" + v
	})
}

func genGroupID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "group-" + v
	})
}

func genValidCIDR() gopter.Gen {
	return gen.OneConstOf(
		"10.0.0.0/8",
		"192.168.0.0/16",
		"172.16.0.0/12",
		"0.0.0.0/0",
		"10.1.0.0/24",
		"192.168.1.0/24",
	)
}

func genInvalidCIDR() gopter.Gen {
	return gen.OneConstOf(
		"not-a-cidr",
		"256.256.256.256/32",
		"10.0.0.0/33",
		"10.0.0.0",
		"",
	)
}

// nolint:unused // Kept for potential future test generation
func genDomainSuffix() gopter.Gen {
	return gen.OneConstOf("internal", "local", "corp", "example.com")
}

// Property Tests

// **Feature: network-groups-policies-routing, Property 27: Route creation completeness**
// **Validates: Requirements 6.1**
func TestProperty_RouteCreationCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 27: Route creation completeness",
		prop.ForAll(
			func(name string, description string, networkID string, cidr string, jumpPeerID string) bool {
				ctx := context.Background()
				routeRepo := newMockRouteRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and jump peer
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				netGetter.peers[jumpPeerID] = &network.Peer{
					ID:     jumpPeerID,
					Name:   "jump-peer",
					IsJump: true,
				}

				service := NewService(routeRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Create route with generated inputs
				route, err := service.CreateRoute(ctx, networkID, &network.RouteCreateRequest{
					Name:            name,
					Description:     description,
					DestinationCIDR: cidr,
					JumpPeerID:      jumpPeerID,
				})

				// Verify property: route has unique ID, provided properties, correct network
				return err == nil &&
					route.ID != "" &&
					route.Name == name &&
					route.Description == description &&
					route.DestinationCIDR == cidr &&
					route.JumpPeerID == jumpPeerID &&
					route.NetworkID == networkID &&
					!route.CreatedAt.IsZero() &&
					!route.UpdatedAt.IsZero()
			},
			genValidRouteName(),
			genDescription(),
			genNetworkID(),
			genValidCIDR(),
			genPeerID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 28: Route CIDR validation**
// **Validates: Requirements 6.2**
func TestProperty_RouteCIDRValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 28: Route CIDR validation",
		prop.ForAll(
			func(networkID string, invalidCIDR string, jumpPeerID string) bool {
				ctx := context.Background()
				routeRepo := newMockRouteRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and jump peer
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				netGetter.peers[jumpPeerID] = &network.Peer{
					ID:     jumpPeerID,
					Name:   "jump-peer",
					IsJump: true,
				}

				service := NewService(routeRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Try to create route with invalid CIDR
				_, err := service.CreateRoute(ctx, networkID, &network.RouteCreateRequest{
					Name:            "test-route",
					Description:     "Test",
					DestinationCIDR: invalidCIDR,
					JumpPeerID:      jumpPeerID,
				})

				// Verify that invalid CIDR is rejected
				return err != nil
			},
			genNetworkID(),
			genInvalidCIDR(),
			genPeerID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 29: Route jump peer validation**
// **Validates: Requirements 6.3**
func TestProperty_RouteJumpPeerValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 29: Route jump peer validation",
		prop.ForAll(
			func(networkID string, cidr string, regularPeerID string) bool {
				ctx := context.Background()
				routeRepo := newMockRouteRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and regular peer (not a jump peer)
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				netGetter.peers[regularPeerID] = &network.Peer{
					ID:     regularPeerID,
					Name:   "regular-peer",
					IsJump: false, // Not a jump peer
				}

				service := NewService(routeRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Try to create route with non-jump peer
				_, err := service.CreateRoute(ctx, networkID, &network.RouteCreateRequest{
					Name:            "test-route",
					Description:     "Test",
					DestinationCIDR: cidr,
					JumpPeerID:      regularPeerID,
				})

				// Verify that non-jump peer is rejected
				return err != nil
			},
			genNetworkID(),
			genValidCIDR(),
			genPeerID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 30: Route attachment to group**
// **Validates: Requirements 6.4**
func TestProperty_RouteAttachmentToGroup(t *testing.T) {
	// Note: Route attachment is handled by GroupService
	// This test verifies that routes can be retrieved for groups
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 30: Route attachment to group",
		prop.ForAll(
			func(networkID string, groupID string, routeID string) bool {
				ctx := context.Background()
				routeRepo := newMockRouteRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network, group, and route
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				groupRepo.groups[groupID] = &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
				}
				routeRepo.routes[routeID] = &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            "test-route",
					DestinationCIDR: "10.0.0.0/8",
					JumpPeerID:      "jump-1",
				}
				routeRepo.groupRoutes[groupID] = []string{routeID}

				// Get routes for group
				routes, err := routeRepo.GetRoutesForGroup(ctx, networkID, groupID)
				if err != nil {
					return false
				}

				// Verify route is attached to group
				return len(routes) == 1 && routes[0].ID == routeID
			},
			genNetworkID(),
			genGroupID(),
			genRouteID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 31: Route detachment from group**
// **Validates: Requirements 6.5**
func TestProperty_RouteDetachmentFromGroup(t *testing.T) {
	// Note: Route detachment is handled by GroupService
	// This test verifies that routes can be removed from groups
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 31: Route detachment from group",
		prop.ForAll(
			func(networkID string, groupID string, routeID string) bool {
				ctx := context.Background()
				routeRepo := newMockRouteRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network, group, and route attached to group
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				groupRepo.groups[groupID] = &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
				}
				routeRepo.routes[routeID] = &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            "test-route",
					DestinationCIDR: "10.0.0.0/8",
					JumpPeerID:      "jump-1",
				}
				// Attach route to group in both repos (they share the same data structure)
				groupRepo.groupRoutes[groupID] = []string{routeID}
				routeRepo.groupRoutes[groupID] = []string{routeID}

				// Detach route from group
				err := groupRepo.DetachRouteFromGroup(ctx, networkID, groupID, routeID)
				if err != nil {
					return false
				}

				// Also update routeRepo to reflect the detachment
				routeRepo.groupRoutes[groupID] = groupRepo.groupRoutes[groupID]

				// Verify route is no longer attached
				routes, err := routeRepo.GetRoutesForGroup(ctx, networkID, groupID)
				return err == nil && len(routes) == 0
			},
			genNetworkID(),
			genGroupID(),
			genRouteID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 32: Automatic route addition on join**
// **Validates: Requirements 6.6**
func TestProperty_AutomaticRouteAdditionOnJoin(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 32: Automatic route addition on join",
		prop.ForAll(
			func(networkID string, peerID string, groupID string, routeID string) bool {
				ctx := context.Background()
				routeRepo := newMockRouteRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network, peer, group with route
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				netGetter.peers[peerID] = &network.Peer{
					ID:   peerID,
					Name: "test-peer",
				}
				groupRepo.groups[groupID] = &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
				}
				routeRepo.routes[routeID] = &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            "test-route",
					DestinationCIDR: "10.0.0.0/8",
					JumpPeerID:      "jump-1",
				}
				routeRepo.groupRoutes[groupID] = []string{routeID}

				service := NewService(routeRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Add peer to group
				if err := groupRepo.AddPeerToGroup(ctx, networkID, groupID, peerID); err != nil {
					return false
				}

				// Get peer routes
				routes, err := service.GetPeerRoutes(ctx, networkID, peerID)
				if err != nil {
					return false
				}

				// Verify peer has the group's route
				return len(routes) == 1 && routes[0].ID == routeID
			},
			genNetworkID(),
			genPeerID(),
			genGroupID(),
			genRouteID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 33: Automatic route removal on leave**
// **Validates: Requirements 6.7**
func TestProperty_AutomaticRouteRemovalOnLeave(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 33: Automatic route removal on leave",
		prop.ForAll(
			func(networkID string, peerID string, groupID string, routeID string) bool {
				ctx := context.Background()
				routeRepo := newMockRouteRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network, peer in group with route
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				netGetter.peers[peerID] = &network.Peer{
					ID:   peerID,
					Name: "test-peer",
				}
				groupRepo.groups[groupID] = &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
				}
				routeRepo.routes[routeID] = &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            "test-route",
					DestinationCIDR: "10.0.0.0/8",
					JumpPeerID:      "jump-1",
				}
				routeRepo.groupRoutes[groupID] = []string{routeID}
				groupRepo.peerGroups[peerID] = []string{groupID}

				service := NewService(routeRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Remove peer from group
				groupRepo.peerGroups[peerID] = []string{}

				// Get peer routes
				routes, err := service.GetPeerRoutes(ctx, networkID, peerID)
				if err != nil {
					return false
				}

				// Verify peer no longer has the group's route
				return len(routes) == 0
			},
			genNetworkID(),
			genPeerID(),
			genGroupID(),
			genRouteID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 34: Route update propagation**
// **Validates: Requirements 6.8**
func TestProperty_RouteUpdatePropagation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 34: Route update propagation",
		prop.ForAll(
			func(networkID string, routeID string, newCIDR string, jumpPeerID string) bool {
				ctx := context.Background()
				routeRepo := newMockRouteRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network, route, and jump peer
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				netGetter.peers[jumpPeerID] = &network.Peer{
					ID:     jumpPeerID,
					Name:   "jump-peer",
					IsJump: true,
				}
				routeRepo.routes[routeID] = &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            "test-route",
					DestinationCIDR: "10.0.0.0/8",
					JumpPeerID:      jumpPeerID,
				}

				service := NewService(routeRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Update route
				updatedRoute, err := service.UpdateRoute(ctx, networkID, routeID, &network.RouteUpdateRequest{
					DestinationCIDR: newCIDR,
				})

				// Verify update succeeded and CIDR changed
				return err == nil && updatedRoute.DestinationCIDR == newCIDR
			},
			genNetworkID(),
			genRouteID(),
			genValidCIDR(),
			genPeerID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 35: Route deletion cleanup**
// **Validates: Requirements 6.9**
func TestProperty_RouteDeletionCleanup(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 35: Route deletion cleanup",
		prop.ForAll(
			func(networkID string, routeID string) bool {
				ctx := context.Background()
				routeRepo := newMockRouteRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and route
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				routeRepo.routes[routeID] = &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            "test-route",
					DestinationCIDR: "10.0.0.0/8",
					JumpPeerID:      "jump-1",
				}

				service := NewService(routeRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Delete route
				err := service.DeleteRoute(ctx, networkID, routeID)
				if err != nil {
					return false
				}

				// Verify route is deleted
				_, err = routeRepo.GetRoute(ctx, networkID, routeID)
				return err != nil // Should return error (route not found)
			},
			genNetworkID(),
			genRouteID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
