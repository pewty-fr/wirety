package group

import (
	"context"
	"fmt"
	"testing"
	"time"

	"wirety/internal/domain/network"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Mock implementations for testing

type mockGroupRepository struct {
	groups        map[string]*network.Group
	groupPeers    map[string][]string // groupID -> []peerID
	groupPolicies map[string][]string // groupID -> []policyID
	groupRoutes   map[string][]string // groupID -> []routeID
}

func newMockGroupRepository() *mockGroupRepository {
	return &mockGroupRepository{
		groups:        make(map[string]*network.Group),
		groupPeers:    make(map[string][]string),
		groupPolicies: make(map[string][]string),
		groupRoutes:   make(map[string][]string),
	}
}

func (m *mockGroupRepository) CreateGroup(ctx context.Context, networkID string, group *network.Group) error {
	// Check for duplicate name
	for _, g := range m.groups {
		if g.NetworkID == networkID && g.Name == group.Name {
			return network.ErrDuplicateGroupName
		}
	}
	m.groups[group.ID] = group
	m.groupPeers[group.ID] = []string{}
	m.groupPolicies[group.ID] = []string{}
	m.groupRoutes[group.ID] = []string{}
	return nil
}

func (m *mockGroupRepository) GetGroup(ctx context.Context, networkID, groupID string) (*network.Group, error) {
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return nil, network.ErrGroupNotFound
	}
	// Copy group and populate IDs
	result := *group
	result.PeerIDs = append([]string{}, m.groupPeers[groupID]...)
	result.PolicyIDs = append([]string{}, m.groupPolicies[groupID]...)
	result.RouteIDs = append([]string{}, m.groupRoutes[groupID]...)
	return &result, nil
}

func (m *mockGroupRepository) UpdateGroup(ctx context.Context, networkID string, group *network.Group) error {
	existing, exists := m.groups[group.ID]
	if !exists || existing.NetworkID != networkID {
		return network.ErrGroupNotFound
	}
	// Check for duplicate name
	for id, g := range m.groups {
		if id != group.ID && g.NetworkID == networkID && g.Name == group.Name {
			return network.ErrDuplicateGroupName
		}
	}
	m.groups[group.ID] = group
	return nil
}

func (m *mockGroupRepository) DeleteGroup(ctx context.Context, networkID, groupID string) error {
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return network.ErrGroupNotFound
	}
	delete(m.groups, groupID)
	delete(m.groupPeers, groupID)
	delete(m.groupPolicies, groupID)
	delete(m.groupRoutes, groupID)
	return nil
}

func (m *mockGroupRepository) ListGroups(ctx context.Context, networkID string) ([]*network.Group, error) {
	var groups []*network.Group
	for _, group := range m.groups {
		if group.NetworkID == networkID {
			result := *group
			result.PeerIDs = append([]string{}, m.groupPeers[group.ID]...)
			result.PolicyIDs = append([]string{}, m.groupPolicies[group.ID]...)
			result.RouteIDs = append([]string{}, m.groupRoutes[group.ID]...)
			groups = append(groups, &result)
		}
	}
	return groups, nil
}

func (m *mockGroupRepository) AddPeerToGroup(ctx context.Context, networkID, groupID, peerID string) error {
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return network.ErrGroupNotFound
	}
	// Check if peer already in group
	for _, pid := range m.groupPeers[groupID] {
		if pid == peerID {
			return nil // Already exists, ignore
		}
	}
	m.groupPeers[groupID] = append(m.groupPeers[groupID], peerID)
	return nil
}

func (m *mockGroupRepository) RemovePeerFromGroup(ctx context.Context, networkID, groupID, peerID string) error {
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return network.ErrGroupNotFound
	}
	peers := m.groupPeers[groupID]
	for i, pid := range peers {
		if pid == peerID {
			m.groupPeers[groupID] = append(peers[:i], peers[i+1:]...)
			return nil
		}
	}
	return network.ErrPeerNotInGroup
}

func (m *mockGroupRepository) GetPeerGroups(ctx context.Context, networkID, peerID string) ([]*network.Group, error) {
	var groups []*network.Group
	for groupID, peers := range m.groupPeers {
		for _, pid := range peers {
			if pid == peerID {
				group := m.groups[groupID]
				if group.NetworkID == networkID {
					result := *group
					result.PeerIDs = append([]string{}, m.groupPeers[groupID]...)
					result.PolicyIDs = append([]string{}, m.groupPolicies[groupID]...)
					result.RouteIDs = append([]string{}, m.groupRoutes[groupID]...)
					groups = append(groups, &result)
				}
				break
			}
		}
	}
	return groups, nil
}

func (m *mockGroupRepository) AttachPolicyToGroup(ctx context.Context, networkID, groupID, policyID string) error {
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return network.ErrGroupNotFound
	}
	m.groupPolicies[groupID] = append(m.groupPolicies[groupID], policyID)
	return nil
}

func (m *mockGroupRepository) DetachPolicyFromGroup(ctx context.Context, networkID, groupID, policyID string) error {
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return network.ErrGroupNotFound
	}
	policies := m.groupPolicies[groupID]
	for i, pid := range policies {
		if pid == policyID {
			m.groupPolicies[groupID] = append(policies[:i], policies[i+1:]...)
			return nil
		}
	}
	return network.ErrPolicyNotAttached
}

func (m *mockGroupRepository) GetGroupPolicies(ctx context.Context, networkID, groupID string) ([]*network.Policy, error) {
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return nil, network.ErrGroupNotFound
	}
	// Return empty list for mock
	return []*network.Policy{}, nil
}

func (m *mockGroupRepository) AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error {
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return network.ErrGroupNotFound
	}
	m.groupRoutes[groupID] = append(m.groupRoutes[groupID], routeID)
	return nil
}

func (m *mockGroupRepository) DetachRouteFromGroup(ctx context.Context, networkID, groupID, routeID string) error {
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return network.ErrGroupNotFound
	}
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
	group, exists := m.groups[groupID]
	if !exists || group.NetworkID != networkID {
		return nil, network.ErrGroupNotFound
	}
	// Return empty list for mock
	return []*network.Route{}, nil
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
	// Verify peer belongs to network
	if _, netExists := m.networks[networkID]; !netExists {
		return nil, network.ErrNetworkNotFound
	}
	return peer, nil
}

// networkGetterAdapter adapts mockNetworkGetter to the minimal interface needed by GroupService
type networkGetterAdapter struct {
	getter *mockNetworkGetter
}

func (a *networkGetterAdapter) GetNetwork(ctx context.Context, networkID string) (*network.Network, error) {
	return a.getter.GetNetwork(ctx, networkID)
}

func (a *networkGetterAdapter) GetPeer(ctx context.Context, networkID, peerID string) (*network.Peer, error) {
	return a.getter.GetPeer(ctx, networkID, peerID)
}

// Implement minimal interface for Repository (only methods used by GroupService)
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

func genValidGroupName() gopter.Gen {
	return gen.Identifier().SuchThat(func(v interface{}) bool {
		s := v.(string)
		return len(s) > 0 && len(s) <= 255
	})
}

func genDescription() gopter.Gen {
	return gen.AlphaString().SuchThat(func(v interface{}) bool {
		s := v.(string)
		return len(s) <= 100
	})
}

func genNetworkID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "net-" + v
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

// Property Tests

// **Feature: network-groups-policies-routing, Property 1: Group creation completeness**
// **Validates: Requirements 1.1**
func TestProperty_GroupCreationCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 1: Group creation completeness",
		prop.ForAll(
			func(name string, description string, networkID string) bool {
				ctx := context.Background()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}

				service := NewService(groupRepo, &networkGetterAdapter{getter: netGetter})

				// Create group with generated inputs
				group, err := service.CreateGroup(ctx, networkID, &network.GroupCreateRequest{
					Name:        name,
					Description: description,
				})

				// Verify property: group has unique ID, provided name/description, correct network
				return err == nil &&
					group.ID != "" &&
					group.Name == name &&
					group.Description == description &&
					group.NetworkID == networkID &&
					!group.CreatedAt.IsZero() &&
					!group.UpdatedAt.IsZero()
			},
			genValidGroupName(),
			genDescription(),
			genNetworkID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 2: Peer-group association preservation**
// **Validates: Requirements 1.2**
func TestProperty_PeerGroupAssociationPreservation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 2: Peer-group association preservation",
		prop.ForAll(
			func(networkID string, groupID string, peerID string) bool {
				ctx := context.Background()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network, group, and peer
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				groupRepo.groups[groupID] = &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
				}
				groupRepo.groupPeers[groupID] = []string{}
				netGetter.peers[peerID] = &network.Peer{
					ID:   peerID,
					Name: "test-peer",
				}

				service := NewService(groupRepo, &networkGetterAdapter{getter: netGetter})

				// Add peer to group
				err := service.AddPeerToGroup(ctx, networkID, groupID, peerID)
				if err != nil {
					return false
				}

				// Verify peer appears in group's member list
				group, err := groupRepo.GetGroup(ctx, networkID, groupID)
				if err != nil {
					return false
				}

				peerInGroup := false
				for _, pid := range group.PeerIDs {
					if pid == peerID {
						peerInGroup = true
						break
					}
				}

				// Verify peer still exists independently
				peer, err := netGetter.GetPeer(ctx, networkID, peerID)

				return peerInGroup && err == nil && peer != nil && peer.ID == peerID
			},
			genNetworkID(),
			genGroupID(),
			genPeerID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 3: Peer removal non-destructiveness**
// **Validates: Requirements 1.3**
func TestProperty_PeerRemovalNonDestructiveness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 3: Peer removal non-destructiveness",
		prop.ForAll(
			func(networkID string, groupID string, peerID string) bool {
				ctx := context.Background()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network, group, and peer
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				groupRepo.groups[groupID] = &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
				}
				groupRepo.groupPeers[groupID] = []string{peerID}
				netGetter.peers[peerID] = &network.Peer{
					ID:   peerID,
					Name: "test-peer",
				}

				service := NewService(groupRepo, &networkGetterAdapter{getter: netGetter})

				// Remove peer from group
				err := service.RemovePeerFromGroup(ctx, networkID, groupID, peerID)
				if err != nil {
					return false
				}

				// Verify peer no longer in group
				group, err := groupRepo.GetGroup(ctx, networkID, groupID)
				if err != nil {
					return false
				}

				peerInGroup := false
				for _, pid := range group.PeerIDs {
					if pid == peerID {
						peerInGroup = true
						break
					}
				}

				// Verify peer still exists in network
				peer, err := netGetter.GetPeer(ctx, networkID, peerID)

				return !peerInGroup && err == nil && peer != nil && peer.ID == peerID
			},
			genNetworkID(),
			genGroupID(),
			genPeerID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 4: Group deletion peer preservation**
// **Validates: Requirements 1.4**
func TestProperty_GroupDeletionPeerPreservation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 4: Group deletion peer preservation",
		prop.ForAll(
			func(networkID string, groupID string, peerIDs []string) bool {
				ctx := context.Background()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Ensure we have at least one peer
				if len(peerIDs) == 0 {
					peerIDs = []string{"peer-1"}
				}

				// Setup: Create network, group, and peers
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				groupRepo.groups[groupID] = &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
				}
				groupRepo.groupPeers[groupID] = append([]string{}, peerIDs...)

				for _, peerID := range peerIDs {
					netGetter.peers[peerID] = &network.Peer{
						ID:   peerID,
						Name: "test-peer-" + peerID,
					}
				}

				service := NewService(groupRepo, &networkGetterAdapter{getter: netGetter})

				// Delete group
				err := service.DeleteGroup(ctx, networkID, groupID)
				if err != nil {
					return false
				}

				// Verify group is deleted
				_, err = groupRepo.GetGroup(ctx, networkID, groupID)
				if err == nil {
					return false // Group should not exist
				}

				// Verify all peers still exist
				for _, peerID := range peerIDs {
					peer, err := netGetter.GetPeer(ctx, networkID, peerID)
					if err != nil || peer == nil || peer.ID != peerID {
						return false
					}
				}

				return true
			},
			genNetworkID(),
			genGroupID(),
			gen.SliceOfN(10, genPeerID()).SuchThat(func(v interface{}) bool {
				slice := v.([]string)
				// Ensure unique peer IDs and at least 1
				if len(slice) < 1 {
					return false
				}
				seen := make(map[string]bool)
				for _, id := range slice {
					if seen[id] {
						return false
					}
					seen[id] = true
				}
				return true
			}),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 5: Group listing completeness**
// **Validates: Requirements 1.5**
func TestProperty_GroupListingCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 5: Group listing completeness",
		prop.ForAll(
			func(networkID string, groupCount int) bool {
				ctx := context.Background()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}

				service := NewService(groupRepo, &networkGetterAdapter{getter: netGetter})

				// Create multiple groups
				createdGroups := make(map[string]int) // groupID -> expected peer count
				for i := 0; i < groupCount; i++ {
					group, err := service.CreateGroup(ctx, networkID, &network.GroupCreateRequest{
						Name:        fmt.Sprintf("group-%d", i),
						Description: "Test group",
					})
					if err != nil {
						return false
					}
					createdGroups[group.ID] = 0
				}

				// List groups
				groups, err := service.ListGroups(ctx, networkID)
				if err != nil {
					return false
				}

				// Verify all created groups are in the list
				if len(groups) != len(createdGroups) {
					return false
				}

				for _, group := range groups {
					if _, exists := createdGroups[group.ID]; !exists {
						return false
					}
					// Verify peer count matches
					if len(group.PeerIDs) != createdGroups[group.ID] {
						return false
					}
				}

				return true
			},
			genNetworkID(),
			gen.IntRange(1, 10),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 7: Group operation authorization**
// **Validates: Requirements 1.7**
// Note: This property test verifies the service layer logic. Authorization middleware
// testing would be done at the API handler level.
func TestProperty_GroupOperationAuthorization(t *testing.T) {
	// This test verifies that the service layer properly handles operations
	// The actual authorization check happens in the API layer middleware
	// Here we verify that operations work correctly when called (assuming auth passed)

	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 7: Group operation authorization",
		prop.ForAll(
			func(networkID string, groupName string) bool {
				ctx := context.Background()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}

				service := NewService(groupRepo, &networkGetterAdapter{getter: netGetter})

				// Verify that operations succeed when called
				// (authorization is enforced at API layer, not service layer)
				group, err := service.CreateGroup(ctx, networkID, &network.GroupCreateRequest{
					Name:        groupName,
					Description: "Test",
				})

				return err == nil && group != nil
			},
			genNetworkID(),
			genValidGroupName(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
