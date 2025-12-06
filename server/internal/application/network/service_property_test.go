package network

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"wirety/internal/domain/auth"
	"wirety/internal/domain/network"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Mock implementations for testing

type mockAuthRepository struct {
	users map[string]*auth.User
}

func newMockAuthRepository() *mockAuthRepository {
	return &mockAuthRepository{
		users: make(map[string]*auth.User),
	}
}

func (m *mockAuthRepository) GetUser(userID string) (*auth.User, error) {
	user, exists := m.users[userID]
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

func (m *mockAuthRepository) GetUserByEmail(email string) (*auth.User, error) {
	return nil, fmt.Errorf("user not found")
}

func (m *mockAuthRepository) CreateUser(user *auth.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockAuthRepository) UpdateUser(user *auth.User) error {
	m.users[user.ID] = user
	return nil
}

func (m *mockAuthRepository) DeleteUser(userID string) error {
	delete(m.users, userID)
	return nil
}

func (m *mockAuthRepository) ListUsers() ([]*auth.User, error) {
	var users []*auth.User
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, nil
}

func (m *mockAuthRepository) GetFirstUser() (*auth.User, error) {
	return nil, fmt.Errorf("user not found")
}

func (m *mockAuthRepository) GetDefaultPermissions() (*auth.DefaultNetworkPermissions, error) {
	return nil, nil
}

func (m *mockAuthRepository) SetDefaultPermissions(perms *auth.DefaultNetworkPermissions) error {
	return nil
}

func (m *mockAuthRepository) CreateSession(session *auth.Session) error {
	return nil
}

func (m *mockAuthRepository) GetSession(sessionHash string) (*auth.Session, error) {
	return nil, fmt.Errorf("session not found")
}

func (m *mockAuthRepository) UpdateSession(session *auth.Session) error {
	return nil
}

func (m *mockAuthRepository) DeleteSession(sessionHash string) error {
	return nil
}

func (m *mockAuthRepository) DeleteUserSessions(userID string) error {
	return nil
}

func (m *mockAuthRepository) CleanupExpiredSessions() error {
	return nil
}

type mockGroupRepository struct {
	groups         map[string]*network.Group
	groupPeers     map[string][]string // groupID -> []peerID
	getGroupRoutes func(ctx context.Context, networkID, groupID string) ([]*network.Route, error)
	getPeerGroups  func(ctx context.Context, networkID, peerID string) ([]*network.Group, error)
}

func newMockGroupRepository() *mockGroupRepository {
	return &mockGroupRepository{
		groups:     make(map[string]*network.Group),
		groupPeers: make(map[string][]string),
	}
}

type mockRouteRepository struct {
	routes map[string]*network.Route
}

func newMockRouteRepository() *mockRouteRepository {
	return &mockRouteRepository{
		routes: make(map[string]*network.Route),
	}
}

func (m *mockRouteRepository) CreateRoute(ctx context.Context, networkID string, route *network.Route) error {
	m.routes[route.ID] = route
	return nil
}

func (m *mockRouteRepository) GetRoute(ctx context.Context, networkID, routeID string) (*network.Route, error) {
	route, exists := m.routes[routeID]
	if !exists {
		return nil, network.ErrRouteNotFound
	}
	return route, nil
}

func (m *mockRouteRepository) UpdateRoute(ctx context.Context, networkID string, route *network.Route) error {
	m.routes[route.ID] = route
	return nil
}

func (m *mockRouteRepository) DeleteRoute(ctx context.Context, networkID, routeID string) error {
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
	return nil, nil
}

func (m *mockRouteRepository) GetRoutesByJumpPeer(ctx context.Context, networkID, jumpPeerID string) ([]*network.Route, error) {
	return nil, nil
}

type mockDNSRepository struct {
	mappings map[string]*network.DNSMapping
}

func newMockDNSRepository() *mockDNSRepository {
	return &mockDNSRepository{
		mappings: make(map[string]*network.DNSMapping),
	}
}

func (m *mockDNSRepository) CreateDNSMapping(ctx context.Context, routeID string, mapping *network.DNSMapping) error {
	m.mappings[mapping.ID] = mapping
	return nil
}

func (m *mockDNSRepository) GetDNSMapping(ctx context.Context, routeID, mappingID string) (*network.DNSMapping, error) {
	mapping, exists := m.mappings[mappingID]
	if !exists {
		return nil, network.ErrDNSMappingNotFound
	}
	return mapping, nil
}

func (m *mockDNSRepository) UpdateDNSMapping(ctx context.Context, routeID string, mapping *network.DNSMapping) error {
	m.mappings[mapping.ID] = mapping
	return nil
}

func (m *mockDNSRepository) DeleteDNSMapping(ctx context.Context, routeID, mappingID string) error {
	delete(m.mappings, mappingID)
	return nil
}

func (m *mockDNSRepository) ListDNSMappings(ctx context.Context, routeID string) ([]*network.DNSMapping, error) {
	var mappings []*network.DNSMapping
	for _, mapping := range m.mappings {
		if mapping.RouteID == routeID {
			mappings = append(mappings, mapping)
		}
	}
	return mappings, nil
}

func (m *mockDNSRepository) GetNetworkDNSMappings(ctx context.Context, networkID string) ([]*network.DNSMapping, error) {
	var mappings []*network.DNSMapping
	for _, mapping := range m.mappings {
		mappings = append(mappings, mapping)
	}
	return mappings, nil
}

func (m *mockGroupRepository) CreateGroup(ctx context.Context, networkID string, group *network.Group) error {
	m.groups[group.ID] = group
	m.groupPeers[group.ID] = []string{}
	return nil
}

func (m *mockGroupRepository) GetGroup(ctx context.Context, networkID, groupID string) (*network.Group, error) {
	group, exists := m.groups[groupID]
	if !exists {
		return nil, network.ErrGroupNotFound
	}
	return group, nil
}

func (m *mockGroupRepository) UpdateGroup(ctx context.Context, networkID string, group *network.Group) error {
	m.groups[group.ID] = group
	return nil
}

func (m *mockGroupRepository) DeleteGroup(ctx context.Context, networkID, groupID string) error {
	delete(m.groups, groupID)
	delete(m.groupPeers, groupID)
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
	if _, exists := m.groups[groupID]; !exists {
		return network.ErrGroupNotFound
	}
	m.groupPeers[groupID] = append(m.groupPeers[groupID], peerID)
	return nil
}

func (m *mockGroupRepository) RemovePeerFromGroup(ctx context.Context, networkID, groupID, peerID string) error {
	return nil
}

func (m *mockGroupRepository) GetPeerGroups(ctx context.Context, networkID, peerID string) ([]*network.Group, error) {
	if m.getPeerGroups != nil {
		return m.getPeerGroups(ctx, networkID, peerID)
	}
	var groups []*network.Group
	for groupID, peers := range m.groupPeers {
		for _, pid := range peers {
			if pid == peerID {
				if group, exists := m.groups[groupID]; exists {
					groups = append(groups, group)
				}
				break
			}
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
	return nil
}

func (m *mockGroupRepository) DetachRouteFromGroup(ctx context.Context, networkID, groupID, routeID string) error {
	return nil
}

func (m *mockGroupRepository) GetGroupRoutes(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
	if m.getGroupRoutes != nil {
		return m.getGroupRoutes(ctx, networkID, groupID)
	}
	return nil, nil
}

func (m *mockGroupRepository) ReorderGroupPolicies(ctx context.Context, networkID, groupID string, policyIDs []string) error {
	return nil
}

// Minimal mock for FullRepository - only implementing methods needed for AddPeer
type mockFullRepository struct {
	networks map[string]*network.Network
	peers    map[string]*network.Peer
	ipam     *mockIPAMRepository
}

func newMockFullRepository() *mockFullRepository {
	return &mockFullRepository{
		networks: make(map[string]*network.Network),
		peers:    make(map[string]*network.Peer),
		ipam:     newMockIPAMRepository(),
	}
}

func (m *mockFullRepository) GetNetwork(ctx context.Context, networkID string) (*network.Network, error) {
	net, exists := m.networks[networkID]
	if !exists {
		return nil, network.ErrNetworkNotFound
	}
	return net, nil
}

func (m *mockFullRepository) CreatePeer(ctx context.Context, networkID string, peer *network.Peer) error {
	m.peers[peer.ID] = peer
	return nil
}

func (m *mockFullRepository) GetPeer(ctx context.Context, networkID, peerID string) (*network.Peer, error) {
	peer, exists := m.peers[peerID]
	if !exists {
		return nil, network.ErrPeerNotFound
	}
	return peer, nil
}

func (m *mockFullRepository) ListPeers(ctx context.Context, networkID string) ([]*network.Peer, error) {
	var peers []*network.Peer
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}
	return peers, nil
}

func (m *mockFullRepository) AcquireIP(ctx context.Context, cidr string) (string, error) {
	return m.ipam.AcquireIP(ctx, cidr)
}

func (m *mockFullRepository) ReleaseIP(ctx context.Context, cidr, ip string) error {
	return m.ipam.ReleaseIP(ctx, cidr, ip)
}

func (m *mockFullRepository) EnsureRootPrefix(ctx context.Context, cidr string) (*network.IPAMPrefix, error) {
	return &network.IPAMPrefix{CIDR: cidr}, nil
}

func (m *mockFullRepository) AcquireChildPrefix(ctx context.Context, parentCIDR string, prefixLen uint8) (*network.IPAMPrefix, error) {
	return nil, nil
}

func (m *mockFullRepository) AcquireSpecificChildPrefix(ctx context.Context, parentCIDR string, cidr string) (*network.IPAMPrefix, error) {
	return nil, nil
}

func (m *mockFullRepository) ReleaseChildPrefix(ctx context.Context, cidr string) error {
	return nil
}

func (m *mockFullRepository) DeletePrefix(ctx context.Context, cidr string) error {
	return nil
}

func (m *mockFullRepository) ListChildPrefixes(ctx context.Context, parentCIDR string) ([]*network.IPAMPrefix, error) {
	return nil, nil
}

func (m *mockFullRepository) CreateConnection(ctx context.Context, networkID string, conn *network.PeerConnection) error {
	return nil
}

// Stub methods for interface compliance
func (m *mockFullRepository) CreateNetwork(ctx context.Context, net *network.Network) error {
	return nil
}
func (m *mockFullRepository) UpdateNetwork(ctx context.Context, net *network.Network) error {
	return nil
}
func (m *mockFullRepository) DeleteNetwork(ctx context.Context, networkID string) error {
	return nil
}
func (m *mockFullRepository) ListNetworks(ctx context.Context) ([]*network.Network, error) {
	return nil, nil
}
func (m *mockFullRepository) GetPeerByToken(ctx context.Context, token string) (string, *network.Peer, error) {
	return "", nil, nil
}
func (m *mockFullRepository) UpdatePeer(ctx context.Context, networkID string, peer *network.Peer) error {
	return nil
}
func (m *mockFullRepository) DeletePeer(ctx context.Context, networkID, peerID string) error {
	return nil
}
func (m *mockFullRepository) CreateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	return nil
}
func (m *mockFullRepository) GetACL(ctx context.Context, networkID string) (*network.ACL, error) {
	return nil, nil
}
func (m *mockFullRepository) UpdateACL(ctx context.Context, networkID string, acl *network.ACL) error {
	return nil
}
func (m *mockFullRepository) GetConnection(ctx context.Context, networkID, peer1ID, peer2ID string) (*network.PeerConnection, error) {
	return nil, nil
}
func (m *mockFullRepository) ListConnections(ctx context.Context, networkID string) ([]*network.PeerConnection, error) {
	return nil, nil
}
func (m *mockFullRepository) DeleteConnection(ctx context.Context, networkID, peer1ID, peer2ID string) error {
	return nil
}
func (m *mockFullRepository) CreateOrUpdateSession(ctx context.Context, networkID string, session *network.AgentSession) error {
	return nil
}
func (m *mockFullRepository) GetSession(ctx context.Context, networkID, peerID string) (*network.AgentSession, error) {
	return nil, nil
}
func (m *mockFullRepository) GetActiveSessionsForPeer(ctx context.Context, networkID, peerID string) ([]*network.AgentSession, error) {
	return nil, nil
}
func (m *mockFullRepository) DeleteSession(ctx context.Context, networkID, sessionID string) error {
	return nil
}
func (m *mockFullRepository) ListSessions(ctx context.Context, networkID string) ([]*network.AgentSession, error) {
	return nil, nil
}
func (m *mockFullRepository) RecordEndpointChange(ctx context.Context, networkID string, change *network.EndpointChange) error {
	return nil
}
func (m *mockFullRepository) GetEndpointChanges(ctx context.Context, networkID, peerID string, since time.Time) ([]*network.EndpointChange, error) {
	return nil, nil
}
func (m *mockFullRepository) DeleteEndpointChanges(ctx context.Context, networkID, peerID string) error {
	return nil
}
func (m *mockFullRepository) CreateSecurityIncident(ctx context.Context, incident *network.SecurityIncident) error {
	return nil
}
func (m *mockFullRepository) GetSecurityIncident(ctx context.Context, incidentID string) (*network.SecurityIncident, error) {
	return nil, nil
}
func (m *mockFullRepository) ListSecurityIncidents(ctx context.Context, resolved *bool) ([]*network.SecurityIncident, error) {
	return nil, nil
}
func (m *mockFullRepository) ListSecurityIncidentsByNetwork(ctx context.Context, networkID string, resolved *bool) ([]*network.SecurityIncident, error) {
	return nil, nil
}
func (m *mockFullRepository) ResolveSecurityIncident(ctx context.Context, incidentID, resolvedBy string) error {
	return nil
}
func (m *mockFullRepository) AddCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	return nil
}
func (m *mockFullRepository) RemoveCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error {
	return nil
}
func (m *mockFullRepository) GetCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) ([]string, error) {
	return nil, nil
}
func (m *mockFullRepository) ClearCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) error {
	return nil
}
func (m *mockFullRepository) CreateCaptivePortalToken(ctx context.Context, token *network.CaptivePortalToken) error {
	return nil
}
func (m *mockFullRepository) GetCaptivePortalToken(ctx context.Context, token string) (*network.CaptivePortalToken, error) {
	return nil, nil
}
func (m *mockFullRepository) DeleteCaptivePortalToken(ctx context.Context, token string) error {
	return nil
}
func (m *mockFullRepository) CleanupExpiredCaptivePortalTokens(ctx context.Context) error {
	return nil
}

// Security config operations
func (m *mockFullRepository) CreateSecurityConfig(ctx context.Context, networkID string, config *network.SecurityConfig) error {
	return nil
}
func (m *mockFullRepository) GetSecurityConfig(ctx context.Context, networkID string) (*network.SecurityConfig, error) {
	return nil, nil
}
func (m *mockFullRepository) UpdateSecurityConfig(ctx context.Context, networkID string, config *network.SecurityConfig) error {
	return nil
}
func (m *mockFullRepository) DeleteSecurityConfig(ctx context.Context, networkID string) error {
	return nil
}

type mockIPAMRepository struct {
	nextIP int
}

func newMockIPAMRepository() *mockIPAMRepository {
	return &mockIPAMRepository{nextIP: 10}
}

func (m *mockIPAMRepository) AcquireIP(ctx context.Context, cidr string) (string, error) {
	ip := fmt.Sprintf("10.0.0.%d", m.nextIP)
	m.nextIP++
	return ip, nil
}

func (m *mockIPAMRepository) ReleaseIP(ctx context.Context, cidr, ip string) error {
	return nil
}

func (m *mockIPAMRepository) EnsureRootPrefix(ctx context.Context, cidr string) (*network.IPAMPrefix, error) {
	return &network.IPAMPrefix{CIDR: cidr}, nil
}

func (m *mockIPAMRepository) AcquireChildPrefix(ctx context.Context, parentCIDR string, prefixLen uint8) (*network.IPAMPrefix, error) {
	return nil, nil
}

func (m *mockIPAMRepository) AcquireSpecificChildPrefix(ctx context.Context, parentCIDR string, cidr string) (*network.IPAMPrefix, error) {
	return nil, nil
}

func (m *mockIPAMRepository) ReleaseChildPrefix(ctx context.Context, cidr string) error {
	return nil
}

func (m *mockIPAMRepository) DeletePrefix(ctx context.Context, cidr string) error {
	return nil
}

func (m *mockIPAMRepository) ListChildPrefixes(ctx context.Context, parentCIDR string) ([]*network.IPAMPrefix, error) {
	return nil, nil
}

// Generators for property-based testing

func genUserID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "user-" + v
	})
}

func genNetworkID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "net-" + v
	})
}

func genGroupID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "group-" + v
	})
}

func genPeerID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "peer-" + v
	})
}

func genPeerName() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		// Convert to lowercase and limit length for DNS compliance
		s := strings.ToLower(v)
		if len(s) > 63 {
			s = s[:63]
		}
		// Ensure it doesn't start or end with hyphen
		s = strings.Trim(s, "-")
		if s == "" {
			s = "peer"
		}
		return s
	})
}

// Property Tests

// **Feature: network-groups-policies-routing, Property 39: Non-admin peer auto-assignment**
// **Validates: Requirements 7.2**
func TestProperty_NonAdminPeerAutoAssignment(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 39: Non-admin peer auto-assignment",
		prop.ForAll(
			func(networkID string, userID string, peerName string, defaultGroupIDs []string) bool {
				ctx := context.Background()

				// Ensure we have at least one default group
				if len(defaultGroupIDs) == 0 {
					defaultGroupIDs = []string{"group-default"}
				}

				// Setup mocks
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				fullRepo := newMockFullRepository()

				// Create non-admin user
				authRepo.users[userID] = &auth.User{
					ID:   userID,
					Role: auth.RoleUser, // Non-admin
				}

				// Create network with default groups
				fullRepo.networks[networkID] = &network.Network{
					ID:              networkID,
					Name:            "test-network",
					CIDR:            "10.0.0.0/16",
					DefaultGroupIDs: defaultGroupIDs,
				}

				// Create default groups
				for _, groupID := range defaultGroupIDs {
					groupRepo.groups[groupID] = &network.Group{
						ID:        groupID,
						NetworkID: networkID,
						Name:      "default-group",
					}
					groupRepo.groupPeers[groupID] = []string{}
				}

				// Create service
				service := &Service{
					repo:      fullRepo,
					authRepo:  authRepo,
					groupRepo: groupRepo,
				}

				// Add peer as non-admin user
				peer, err := service.AddPeer(ctx, networkID, &network.PeerCreateRequest{
					Name: peerName,
				}, userID)

				if err != nil {
					return false
				}

				// Verify peer was added to all default groups
				for _, groupID := range defaultGroupIDs {
					peers := groupRepo.groupPeers[groupID]
					found := false
					for _, peerID := range peers {
						if peerID == peer.ID {
							found = true
							break
						}
					}
					if !found {
						return false
					}
				}

				return true
			},
			genNetworkID(),
			genUserID(),
			genPeerName(),
			gen.SliceOfN(3, genGroupID()).SuchThat(func(v interface{}) bool {
				slice := v.([]string)
				// Ensure unique group IDs
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

// **Feature: network-groups-policies-routing, Property 40: Admin peer no auto-assignment**
// **Validates: Requirements 7.3**
func TestProperty_AdminPeerNoAutoAssignment(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 40: Admin peer no auto-assignment",
		prop.ForAll(
			func(networkID string, userID string, peerName string, defaultGroupIDs []string) bool {
				ctx := context.Background()

				// Ensure we have at least one default group
				if len(defaultGroupIDs) == 0 {
					defaultGroupIDs = []string{"group-default"}
				}

				// Setup mocks
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				fullRepo := newMockFullRepository()

				// Create admin user
				authRepo.users[userID] = &auth.User{
					ID:   userID,
					Role: auth.RoleAdministrator, // Admin
				}

				// Create network with default groups
				fullRepo.networks[networkID] = &network.Network{
					ID:              networkID,
					Name:            "test-network",
					CIDR:            "10.0.0.0/16",
					DefaultGroupIDs: defaultGroupIDs,
				}

				// Create default groups
				for _, groupID := range defaultGroupIDs {
					groupRepo.groups[groupID] = &network.Group{
						ID:        groupID,
						NetworkID: networkID,
						Name:      "default-group",
					}
					groupRepo.groupPeers[groupID] = []string{}
				}

				// Create service
				service := &Service{
					repo:      fullRepo,
					authRepo:  authRepo,
					groupRepo: groupRepo,
				}

				// Add peer as admin user
				peer, err := service.AddPeer(ctx, networkID, &network.PeerCreateRequest{
					Name: peerName,
				}, userID)

				if err != nil {
					return false
				}

				// Verify peer was NOT added to any default groups
				for _, groupID := range defaultGroupIDs {
					peers := groupRepo.groupPeers[groupID]
					for _, peerID := range peers {
						if peerID == peer.ID {
							return false // Peer should not be in default group
						}
					}
				}

				return true
			},
			genNetworkID(),
			genUserID(),
			genPeerName(),
			gen.SliceOfN(3, genGroupID()).SuchThat(func(v interface{}) bool {
				slice := v.([]string)
				// Ensure unique group IDs
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

// **Feature: network-groups-policies-routing, Property 53: DNS server initialization completeness**
// **Validates: Requirements 10.1**
func TestProperty_DNSServerInitializationCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 53: DNS server initialization completeness",
		prop.ForAll(
			func(networkID string, peerNames []string, routeNames []string, dnsNames []string) bool {
				ctx := context.Background()

				// Ensure we have at least one peer and one route
				if len(peerNames) == 0 {
					peerNames = []string{"peer1"}
				}
				if len(routeNames) == 0 {
					routeNames = []string{"route1"}
				}
				if len(dnsNames) == 0 {
					dnsNames = []string{"server1"}
				}

				// Setup mocks
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				fullRepo := newMockFullRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				fullRepo.networks[networkID] = &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         "10.0.0.0/16",
					DomainSuffix: "internal",
					Peers:        make(map[string]*network.Peer),
				}

				// Create peers (including at least one jump peer)
				jumpPeerID := "jump-peer-1"
				jumpPeer := &network.Peer{
					ID:      jumpPeerID,
					Name:    "jump-peer",
					Address: "10.0.0.1",
					IsJump:  true,
				}
				fullRepo.peers[jumpPeerID] = jumpPeer
				fullRepo.networks[networkID].Peers[jumpPeerID] = jumpPeer

				peerCount := 0
				for _, peerName := range peerNames {
					peerID := fmt.Sprintf("peer-%d", peerCount)
					peer := &network.Peer{
						ID:      peerID,
						Name:    peerName,
						Address: fmt.Sprintf("10.0.0.%d", peerCount+10),
						IsJump:  false,
					}
					fullRepo.peers[peerID] = peer
					fullRepo.networks[networkID].Peers[peerID] = peer
					peerCount++
				}

				// Create routes with DNS mappings
				routeCount := 0
				for i, routeName := range routeNames {
					routeID := fmt.Sprintf("route-%d", routeCount)
					route := &network.Route{
						ID:              routeID,
						NetworkID:       networkID,
						Name:            routeName,
						DestinationCIDR: fmt.Sprintf("192.168.%d.0/24", i),
						JumpPeerID:      jumpPeerID,
						DomainSuffix:    "internal",
					}
					routeRepo.routes[routeID] = route

					// Create DNS mapping for this route
					if i < len(dnsNames) {
						mappingID := fmt.Sprintf("dns-%d", i)
						mapping := &network.DNSMapping{
							ID:        mappingID,
							RouteID:   routeID,
							Name:      dnsNames[i],
							IPAddress: fmt.Sprintf("192.168.%d.10", i),
						}
						dnsRepo.mappings[mappingID] = mapping
					}

					routeCount++
				}

				// Create service
				service := &Service{
					repo:      fullRepo,
					authRepo:  authRepo,
					groupRepo: groupRepo,
					routeRepo: routeRepo,
					dnsRepo:   dnsRepo,
				}

				// Generate DNS config for jump peer
				_, dnsConfig, _, err := service.GeneratePeerConfigWithDNS(ctx, networkID, jumpPeerID)
				if err != nil {
					return false
				}

				// Verify DNS config is not nil for jump peer
				if dnsConfig == nil {
					return false
				}

				// Verify all peers are in DNS config
				peerDNSCount := 0
				for _, dnsPeer := range dnsConfig.Peers {
					// Check if this is a peer DNS record (not a route DNS record)
					// Peer DNS records have simple names, route DNS records have FQDNs
					if !strings.Contains(dnsPeer.Name, ".") {
						peerDNSCount++
					}
				}

				// Should have all peers (including jump peer itself)
				expectedPeerCount := len(peerNames) + 1 // +1 for jump peer
				if peerDNSCount != expectedPeerCount {
					return false
				}

				// Verify route DNS mappings are included
				routeDNSCount := 0
				for _, dnsPeer := range dnsConfig.Peers {
					// Route DNS records have FQDNs (contain dots)
					if strings.Contains(dnsPeer.Name, ".") {
						routeDNSCount++
					}
				}

				// Should have DNS mappings for routes
				expectedRouteDNSCount := len(dnsNames)
				if len(routeNames) < len(dnsNames) {
					expectedRouteDNSCount = len(routeNames)
				}
				if routeDNSCount != expectedRouteDNSCount {
					return false
				}

				return true
			},
			genNetworkID(),
			gen.SliceOfN(3, genPeerName()).SuchThat(func(v interface{}) bool {
				slice := v.([]string)
				// Ensure unique peer names
				seen := make(map[string]bool)
				for _, name := range slice {
					if seen[name] {
						return false
					}
					seen[name] = true
				}
				return true
			}),
			gen.SliceOfN(2, genPeerName()).SuchThat(func(v interface{}) bool {
				slice := v.([]string)
				// Ensure unique route names
				seen := make(map[string]bool)
				for _, name := range slice {
					if seen[name] {
						return false
					}
					seen[name] = true
				}
				return true
			}),
			gen.SliceOfN(2, genPeerName()).SuchThat(func(v interface{}) bool {
				slice := v.([]string)
				// Ensure unique DNS names
				seen := make(map[string]bool)
				for _, name := range slice {
					if seen[name] {
						return false
					}
					seen[name] = true
				}
				return true
			}),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 54: Route DNS query resolution**
// **Validates: Requirements 10.2**
func TestProperty_RouteDNSQueryResolution(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 54: Route DNS query resolution",
		prop.ForAll(
			func(networkID string, routeName string, dnsName string, ipAddress string) bool {
				ctx := context.Background()

				// Setup mocks
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				fullRepo := newMockFullRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				fullRepo.networks[networkID] = &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         "10.0.0.0/16",
					DomainSuffix: "internal",
					Peers:        make(map[string]*network.Peer),
				}

				// Create jump peer
				jumpPeerID := "jump-peer-1"
				jumpPeer := &network.Peer{
					ID:      jumpPeerID,
					Name:    "jump-peer",
					Address: "10.0.0.1",
					IsJump:  true,
				}
				fullRepo.peers[jumpPeerID] = jumpPeer
				fullRepo.networks[networkID].Peers[jumpPeerID] = jumpPeer

				// Create route
				routeID := "route-1"
				route := &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            routeName,
					DestinationCIDR: "192.168.1.0/24",
					JumpPeerID:      jumpPeerID,
					DomainSuffix:    "internal",
				}
				routeRepo.routes[routeID] = route

				// Create DNS mapping for the route
				mappingID := "dns-1"
				mapping := &network.DNSMapping{
					ID:        mappingID,
					RouteID:   routeID,
					Name:      dnsName,
					IPAddress: ipAddress,
				}
				dnsRepo.mappings[mappingID] = mapping

				// Create service
				service := &Service{
					repo:      fullRepo,
					authRepo:  authRepo,
					groupRepo: groupRepo,
					routeRepo: routeRepo,
					dnsRepo:   dnsRepo,
				}

				// Generate DNS config for jump peer
				_, dnsConfig, _, err := service.GeneratePeerConfigWithDNS(ctx, networkID, jumpPeerID)
				if err != nil {
					return false
				}

				// Verify DNS config contains the route DNS mapping
				if dnsConfig == nil {
					return false
				}

				// Build expected FQDN: name.route_name.domain_suffix
				expectedFQDN := fmt.Sprintf("%s.%s.%s", sanitizeDNSLabel(dnsName), sanitizeDNSLabel(routeName), "internal")

				// Find the DNS record for the route
				found := false
				for _, dnsPeer := range dnsConfig.Peers {
					if dnsPeer.Name == expectedFQDN && dnsPeer.IP == ipAddress {
						found = true
						break
					}
				}

				return found
			},
			genNetworkID(),
			genPeerName(),             // Use as route name
			genPeerName(),             // Use as DNS name
			gen.Const("192.168.1.10"), // Fixed IP within route CIDR
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 55: Peer DNS query resolution**
// **Validates: Requirements 10.3**
func TestProperty_PeerDNSQueryResolution(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 55: Peer DNS query resolution",
		prop.ForAll(
			func(networkID string, peerName string, peerAddress string) bool {
				ctx := context.Background()

				// Setup mocks
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				fullRepo := newMockFullRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				fullRepo.networks[networkID] = &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         "10.0.0.0/16",
					DomainSuffix: "internal",
					Peers:        make(map[string]*network.Peer),
				}

				// Create jump peer
				jumpPeerID := "jump-peer-1"
				jumpPeer := &network.Peer{
					ID:      jumpPeerID,
					Name:    "jump-peer",
					Address: "10.0.0.1",
					IsJump:  true,
				}
				fullRepo.peers[jumpPeerID] = jumpPeer
				fullRepo.networks[networkID].Peers[jumpPeerID] = jumpPeer

				// Create regular peer
				peerID := "peer-1"
				peer := &network.Peer{
					ID:      peerID,
					Name:    peerName,
					Address: peerAddress,
					IsJump:  false,
				}
				fullRepo.peers[peerID] = peer
				fullRepo.networks[networkID].Peers[peerID] = peer

				// Create service
				service := &Service{
					repo:      fullRepo,
					authRepo:  authRepo,
					groupRepo: groupRepo,
					routeRepo: routeRepo,
					dnsRepo:   dnsRepo,
				}

				// Generate DNS config for jump peer
				_, dnsConfig, _, err := service.GeneratePeerConfigWithDNS(ctx, networkID, jumpPeerID)
				if err != nil {
					return false
				}

				// Verify DNS config contains the peer DNS record
				if dnsConfig == nil {
					return false
				}

				// Find the DNS record for the peer
				// Peer DNS records have sanitized names (not FQDNs)
				sanitizedName := sanitizeDNSLabel(peerName)
				found := false
				for _, dnsPeer := range dnsConfig.Peers {
					// Peer DNS records don't contain dots (unlike route DNS records which are FQDNs)
					if dnsPeer.Name == sanitizedName && dnsPeer.IP == peerAddress {
						found = true
						break
					}
				}

				return found
			},
			genNetworkID(),
			genPeerName(),
			gen.Const("10.0.0.10"), // Fixed IP within network CIDR
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 56: WireGuard config route inclusion**
// **Validates: Requirements 17.1**
func TestProperty_WireGuardConfigRouteInclusion(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 56: WireGuard config route inclusion",
		prop.ForAll(
			func(networkID string, peerName string, routeCIDRs []string) bool {
				ctx := context.Background()

				// Ensure we have at least one route
				if len(routeCIDRs) == 0 {
					routeCIDRs = []string{"192.168.1.0/24"}
				}

				// Setup mocks
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				fullRepo := newMockFullRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				fullRepo.networks[networkID] = &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         "10.0.0.0/16",
					DomainSuffix: "internal",
					Peers:        make(map[string]*network.Peer),
				}

				// Create jump peer
				jumpPeerID := "jump-peer-1"
				jumpPeer := &network.Peer{
					ID:         jumpPeerID,
					Name:       "jump-peer",
					Address:    "10.0.0.1",
					IsJump:     true,
					PublicKey:  "jump-public-key",
					PrivateKey: "jump-private-key",
					Endpoint:   "jump.example.com",
					ListenPort: 51820,
				}
				fullRepo.peers[jumpPeerID] = jumpPeer
				fullRepo.networks[networkID].Peers[jumpPeerID] = jumpPeer

				// Create regular peer
				peerID := "peer-1"
				peer := &network.Peer{
					ID:         peerID,
					Name:       peerName,
					Address:    "10.0.0.10",
					IsJump:     false,
					PublicKey:  "peer-public-key",
					PrivateKey: "peer-private-key",
					GroupIDs:   []string{"group-1"},
				}
				fullRepo.peers[peerID] = peer
				fullRepo.networks[networkID].Peers[peerID] = peer

				// Create group
				groupID := "group-1"
				groupRepo.groups[groupID] = &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
				}
				groupRepo.groupPeers[groupID] = []string{peerID}

				// Create routes and attach to group
				routes := []*network.Route{}
				for i, cidr := range routeCIDRs {
					routeID := fmt.Sprintf("route-%d", i)
					route := &network.Route{
						ID:              routeID,
						NetworkID:       networkID,
						Name:            fmt.Sprintf("route-%d", i),
						DestinationCIDR: cidr,
						JumpPeerID:      jumpPeerID,
						DomainSuffix:    "internal",
					}
					routeRepo.routes[routeID] = route
					routes = append(routes, route)
				}

				// Mock GetGroupRoutes to return the routes
				groupRepo.getGroupRoutes = func(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
					return routes, nil
				}

				// Create service
				service := &Service{
					repo:      fullRepo,
					authRepo:  authRepo,
					groupRepo: groupRepo,
					routeRepo: routeRepo,
					dnsRepo:   dnsRepo,
				}

				// Generate config for regular peer
				config, err := service.GeneratePeerConfig(ctx, networkID, peerID)
				if err != nil {
					return false
				}

				// Verify all route CIDRs are in the config's AllowedIPs for the jump peer
				for _, cidr := range routeCIDRs {
					if !strings.Contains(config, cidr) {
						return false
					}
				}

				return true
			},
			genNetworkID(),
			genPeerName(),
			gen.SliceOfN(2, gen.OneConstOf("192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24", "10.1.0.0/16")).SuchThat(func(v interface{}) bool {
				slice := v.([]string)
				// Ensure unique CIDRs
				seen := make(map[string]bool)
				for _, cidr := range slice {
					if seen[cidr] {
						return false
					}
					seen[cidr] = true
				}
				return true
			}),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 57: WireGuard config network CIDR inclusion**
// **Validates: Requirements 17.2**
func TestProperty_WireGuardConfigNetworkCIDRInclusion(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 57: WireGuard config network CIDR inclusion",
		prop.ForAll(
			func(networkID string, peerName string, networkCIDR string) bool {
				ctx := context.Background()

				// Setup mocks
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				fullRepo := newMockFullRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				fullRepo.networks[networkID] = &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         networkCIDR,
					DomainSuffix: "internal",
					Peers:        make(map[string]*network.Peer),
				}

				// Create jump peer
				jumpPeerID := "jump-peer-1"
				jumpPeer := &network.Peer{
					ID:         jumpPeerID,
					Name:       "jump-peer",
					Address:    "10.0.0.1",
					IsJump:     true,
					PublicKey:  "jump-public-key",
					PrivateKey: "jump-private-key",
					Endpoint:   "jump.example.com",
					ListenPort: 51820,
				}
				fullRepo.peers[jumpPeerID] = jumpPeer
				fullRepo.networks[networkID].Peers[jumpPeerID] = jumpPeer

				// Create regular peer
				peerID := "peer-1"
				peer := &network.Peer{
					ID:         peerID,
					Name:       peerName,
					Address:    "10.0.0.10",
					IsJump:     false,
					PublicKey:  "peer-public-key",
					PrivateKey: "peer-private-key",
				}
				fullRepo.peers[peerID] = peer
				fullRepo.networks[networkID].Peers[peerID] = peer

				// Create service
				service := &Service{
					repo:      fullRepo,
					authRepo:  authRepo,
					groupRepo: groupRepo,
					routeRepo: routeRepo,
					dnsRepo:   dnsRepo,
				}

				// Generate config for regular peer
				config, err := service.GeneratePeerConfig(ctx, networkID, peerID)
				if err != nil {
					return false
				}

				// Verify network CIDR is in the config's AllowedIPs for the jump peer
				if !strings.Contains(config, networkCIDR) {
					return false
				}

				return true
			},
			genNetworkID(),
			genPeerName(),
			gen.OneConstOf("10.0.0.0/16", "172.16.0.0/12", "192.168.0.0/16"),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 58: WireGuard config route gateway**
// **Validates: Requirements 17.3**
func TestProperty_WireGuardConfigRouteGateway(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 58: WireGuard config route gateway",
		prop.ForAll(
			func(networkID string, peerName string, routeCIDR string) bool {
				ctx := context.Background()

				// Setup mocks
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				fullRepo := newMockFullRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				fullRepo.networks[networkID] = &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         "10.0.0.0/16",
					DomainSuffix: "internal",
					Peers:        make(map[string]*network.Peer),
				}

				// Create jump peer
				jumpPeerID := "jump-peer-1"
				jumpPeer := &network.Peer{
					ID:         jumpPeerID,
					Name:       "jump-peer",
					Address:    "10.0.0.1",
					IsJump:     true,
					PublicKey:  "jump-public-key",
					PrivateKey: "jump-private-key",
					Endpoint:   "jump.example.com",
					ListenPort: 51820,
				}
				fullRepo.peers[jumpPeerID] = jumpPeer
				fullRepo.networks[networkID].Peers[jumpPeerID] = jumpPeer

				// Create regular peer
				peerID := "peer-1"
				peer := &network.Peer{
					ID:         peerID,
					Name:       peerName,
					Address:    "10.0.0.10",
					IsJump:     false,
					PublicKey:  "peer-public-key",
					PrivateKey: "peer-private-key",
					GroupIDs:   []string{"group-1"},
				}
				fullRepo.peers[peerID] = peer
				fullRepo.networks[networkID].Peers[peerID] = peer

				// Create group
				groupID := "group-1"
				groupRepo.groups[groupID] = &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
				}
				groupRepo.groupPeers[groupID] = []string{peerID}

				// Create route
				routeID := "route-1"
				route := &network.Route{
					ID:              routeID,
					NetworkID:       networkID,
					Name:            "test-route",
					DestinationCIDR: routeCIDR,
					JumpPeerID:      jumpPeerID,
					DomainSuffix:    "internal",
				}
				routeRepo.routes[routeID] = route

				// Mock GetGroupRoutes to return the route
				groupRepo.getGroupRoutes = func(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
					return []*network.Route{route}, nil
				}

				// Create service
				service := &Service{
					repo:      fullRepo,
					authRepo:  authRepo,
					groupRepo: groupRepo,
					routeRepo: routeRepo,
					dnsRepo:   dnsRepo,
				}

				// Generate config for regular peer
				config, err := service.GeneratePeerConfig(ctx, networkID, peerID)
				if err != nil {
					return false
				}

				// Verify the config contains a [Peer] section for the jump peer
				if !strings.Contains(config, "# Name: jump-peer") {
					return false
				}

				// Verify the jump peer section contains the route CIDR in AllowedIPs
				// Find the jump peer section
				lines := strings.Split(config, "\n")
				inJumpPeerSection := false
				foundRouteInAllowedIPs := false

				for _, line := range lines {
					if strings.Contains(line, "# Name: jump-peer") {
						inJumpPeerSection = true
					} else if inJumpPeerSection && strings.HasPrefix(line, "[") {
						// Entered a new section
						break
					} else if inJumpPeerSection && strings.HasPrefix(line, "AllowedIPs") {
						if strings.Contains(line, routeCIDR) {
							foundRouteInAllowedIPs = true
							break
						}
					}
				}

				return foundRouteInAllowedIPs
			},
			genNetworkID(),
			genPeerName(),
			gen.OneConstOf("192.168.1.0/24", "192.168.2.0/24", "10.1.0.0/16"),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 59: Jump peer config completeness**
// **Validates: Requirements 17.4**
func TestProperty_JumpPeerConfigCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 59: Jump peer config completeness",
		prop.ForAll(
			func(networkID string, peerAddresses []string) bool {
				ctx := context.Background()

				// Ensure we have at least one regular peer
				if len(peerAddresses) == 0 {
					peerAddresses = []string{"10.0.0.10"}
				}

				// Setup mocks
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				fullRepo := newMockFullRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				fullRepo.networks[networkID] = &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         "10.0.0.0/16",
					DomainSuffix: "internal",
					Peers:        make(map[string]*network.Peer),
				}

				// Create jump peer
				jumpPeerID := "jump-peer-1"
				jumpPeer := &network.Peer{
					ID:         jumpPeerID,
					Name:       "jump-peer",
					Address:    "10.0.0.1",
					IsJump:     true,
					PublicKey:  "jump-public-key",
					PrivateKey: "jump-private-key",
					ListenPort: 51820,
				}
				fullRepo.peers[jumpPeerID] = jumpPeer
				fullRepo.networks[networkID].Peers[jumpPeerID] = jumpPeer

				// Create regular peers
				for i, address := range peerAddresses {
					peerID := fmt.Sprintf("peer-%d", i)
					peer := &network.Peer{
						ID:         peerID,
						Name:       fmt.Sprintf("peer-%d", i),
						Address:    address,
						IsJump:     false,
						PublicKey:  fmt.Sprintf("peer-public-key-%d", i),
						PrivateKey: fmt.Sprintf("peer-private-key-%d", i),
					}
					fullRepo.peers[peerID] = peer
					fullRepo.networks[networkID].Peers[peerID] = peer
				}

				// Create service
				service := &Service{
					repo:      fullRepo,
					authRepo:  authRepo,
					groupRepo: groupRepo,
					routeRepo: routeRepo,
					dnsRepo:   dnsRepo,
				}

				// Generate config for jump peer
				config, err := service.GeneratePeerConfig(ctx, networkID, jumpPeerID)
				if err != nil {
					return false
				}

				// Verify network CIDR is in the config (which encompasses all peer addresses)
				if !strings.Contains(config, "10.0.0.0/16") {
					return false
				}

				return true
			},
			genNetworkID(),
			gen.SliceOfN(3, gen.OneConstOf("10.0.0.10", "10.0.0.11", "10.0.0.12", "10.0.0.13", "10.0.0.14")).SuchThat(func(v interface{}) bool {
				slice := v.([]string)
				// Ensure unique addresses
				seen := make(map[string]bool)
				for _, addr := range slice {
					if seen[addr] {
						return false
					}
					seen[addr] = true
				}
				return true
			}),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 60: Jump peer config route CIDRs**
// **Validates: Requirements 17.5**
func TestProperty_JumpPeerConfigRouteCIDRs(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 60: Jump peer config route CIDRs",
		prop.ForAll(
			func(networkID string, routeCIDRs []string) bool {
				ctx := context.Background()

				// Ensure we have at least one route
				if len(routeCIDRs) == 0 {
					routeCIDRs = []string{"192.168.1.0/24"}
				}

				// Setup mocks
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				fullRepo := newMockFullRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				fullRepo.networks[networkID] = &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         "10.0.0.0/16",
					DomainSuffix: "internal",
					Peers:        make(map[string]*network.Peer),
				}

				// Create jump peer
				jumpPeerID := "jump-peer-1"
				jumpPeer := &network.Peer{
					ID:         jumpPeerID,
					Name:       "jump-peer",
					Address:    "10.0.0.1",
					IsJump:     true,
					PublicKey:  "jump-public-key",
					PrivateKey: "jump-private-key",
					ListenPort: 51820,
					GroupIDs:   []string{"group-1"},
				}
				fullRepo.peers[jumpPeerID] = jumpPeer
				fullRepo.networks[networkID].Peers[jumpPeerID] = jumpPeer

				// Create a regular peer so the jump peer has someone to connect to
				regularPeerID := "regular-peer-1"
				regularPeer := &network.Peer{
					ID:         regularPeerID,
					Name:       "regular-peer",
					Address:    "10.0.0.10",
					IsJump:     false,
					PublicKey:  "regular-public-key",
					PrivateKey: "regular-private-key",
				}
				fullRepo.peers[regularPeerID] = regularPeer
				fullRepo.networks[networkID].Peers[regularPeerID] = regularPeer

				// Create group
				groupID := "group-1"
				groupRepo.groups[groupID] = &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
				}
				groupRepo.groupPeers[groupID] = []string{jumpPeerID}

				// Create routes
				routes := []*network.Route{}
				for i, cidr := range routeCIDRs {
					routeID := fmt.Sprintf("route-%d", i)
					route := &network.Route{
						ID:              routeID,
						NetworkID:       networkID,
						Name:            fmt.Sprintf("route-%d", i),
						DestinationCIDR: cidr,
						JumpPeerID:      jumpPeerID,
						DomainSuffix:    "internal",
					}
					routeRepo.routes[routeID] = route
					routes = append(routes, route)
				}

				// Mock GetPeerGroups to return the group for the jump peer
				groupRepo.getPeerGroups = func(ctx context.Context, networkID, peerID string) ([]*network.Group, error) {
					if peerID == jumpPeerID {
						return []*network.Group{groupRepo.groups[groupID]}, nil
					}
					return nil, nil
				}

				// Mock GetGroupRoutes to return the routes
				groupRepo.getGroupRoutes = func(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
					return routes, nil
				}

				// Create service
				service := &Service{
					repo:      fullRepo,
					authRepo:  authRepo,
					groupRepo: groupRepo,
					routeRepo: routeRepo,
					dnsRepo:   dnsRepo,
				}

				// Generate config for jump peer
				config, err := service.GeneratePeerConfig(ctx, networkID, jumpPeerID)
				if err != nil {
					t.Logf("Error generating config: %v", err)
					return false
				}

				// Verify all route CIDRs are in the config
				for _, cidr := range routeCIDRs {
					if !strings.Contains(config, cidr) {
						t.Logf("Config missing CIDR %s. Config:\n%s", cidr, config)
						return false
					}
				}

				return true
			},
			genNetworkID(),
			gen.SliceOfN(2, gen.OneConstOf("192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24", "10.1.0.0/16")).SuchThat(func(v interface{}) bool {
				slice := v.([]string)
				// Ensure unique CIDRs
				seen := make(map[string]bool)
				for _, cidr := range slice {
					if seen[cidr] {
						return false
					}
					seen[cidr] = true
				}
				return true
			}),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 61: Jump peer iptables generation**
// **Validates: Requirements 17.6**
func TestProperty_JumpPeerIPTablesGeneration(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 61: Jump peer iptables generation",
		prop.ForAll(
			func(networkID string, jumpPeerID string, numPolicies int) bool {
				ctx := context.Background()

				// Create mock repositories
				fullRepo := newMockFullRepository()
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				net := &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         "10.0.0.0/24",
					Peers:        make(map[string]*network.Peer),
					DomainSuffix: "internal",
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				fullRepo.networks[networkID] = net

				// Create jump peer
				jumpPeer := &network.Peer{
					ID:        jumpPeerID,
					Name:      "jump-peer",
					Address:   "10.0.0.1",
					IsJump:    true,
					UseAgent:  true,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				net.Peers[jumpPeerID] = jumpPeer

				// Create groups with policies
				groupID := "group-1"
				group := &network.Group{
					ID:        groupID,
					NetworkID: networkID,
					Name:      "test-group",
					PeerIDs:   []string{jumpPeerID},
				}

				// Mock GetPeerGroups to return the group
				groupRepo.getPeerGroups = func(ctx context.Context, networkID, peerID string) ([]*network.Group, error) {
					return []*network.Group{group}, nil
				}

				// Mock GetGroupRoutes to return empty routes
				groupRepo.getGroupRoutes = func(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
					return []*network.Route{}, nil
				}

				// Create mock policy service that generates iptables rules
				mockPolicyService := &mockPolicyService{
					rules: make([]string, numPolicies+2), // +2 for default deny rules
				}
				for i := 0; i < numPolicies; i++ {
					mockPolicyService.rules[i] = fmt.Sprintf("iptables -A INPUT -s 10.0.0.%d/32 -j ACCEPT", i+10)
				}
				// Add default deny rules
				mockPolicyService.rules[numPolicies] = "iptables -A INPUT -j DROP"
				mockPolicyService.rules[numPolicies+1] = "iptables -A OUTPUT -j DROP"

				// Create service
				service := &Service{
					repo:          fullRepo,
					authRepo:      authRepo,
					groupRepo:     groupRepo,
					routeRepo:     routeRepo,
					dnsRepo:       dnsRepo,
					policyService: mockPolicyService,
				}

				// Generate config with DNS and policy
				_, _, policy, err := service.GeneratePeerConfigWithDNS(ctx, networkID, jumpPeerID)
				if err != nil {
					t.Logf("Error generating config: %v", err)
					return false
				}

				// Verify iptables rules are generated
				if policy == nil {
					t.Logf("Policy is nil for jump peer")
					return false
				}

				if len(policy.IPTablesRules) != numPolicies+2 {
					t.Logf("Expected %d iptables rules, got %d", numPolicies+2, len(policy.IPTablesRules))
					return false
				}

				// Verify rules match what was generated
				for i, rule := range policy.IPTablesRules {
					if rule != mockPolicyService.rules[i] {
						t.Logf("Rule mismatch at index %d: expected %s, got %s", i, mockPolicyService.rules[i], rule)
						return false
					}
				}

				return true
			},
			genNetworkID(),
			genPeerID(),
			gen.IntRange(0, 5), // Number of policies
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 62: IPtables input deny rule**
// **Validates: Requirements 17.7**
func TestProperty_IPTablesInputDenyRule(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 62: IPtables input deny rule",
		prop.ForAll(
			func(networkID string, jumpPeerID string) bool {
				ctx := context.Background()

				// Create mock repositories
				fullRepo := newMockFullRepository()
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				net := &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         "10.0.0.0/24",
					Peers:        make(map[string]*network.Peer),
					DomainSuffix: "internal",
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				fullRepo.networks[networkID] = net

				// Create jump peer
				jumpPeer := &network.Peer{
					ID:        jumpPeerID,
					Name:      "jump-peer",
					Address:   "10.0.0.1",
					IsJump:    true,
					UseAgent:  true,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				net.Peers[jumpPeerID] = jumpPeer

				// Mock GetPeerGroups to return empty groups
				groupRepo.getPeerGroups = func(ctx context.Context, networkID, peerID string) ([]*network.Group, error) {
					return []*network.Group{}, nil
				}

				// Mock GetGroupRoutes to return empty routes
				groupRepo.getGroupRoutes = func(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
					return []*network.Route{}, nil
				}

				// Create mock policy service with input deny rule
				mockPolicyService := &mockPolicyService{
					rules: []string{
						"iptables -A INPUT -j DROP",
						"iptables -A OUTPUT -j DROP",
					},
				}

				// Create service
				service := &Service{
					repo:          fullRepo,
					authRepo:      authRepo,
					groupRepo:     groupRepo,
					routeRepo:     routeRepo,
					dnsRepo:       dnsRepo,
					policyService: mockPolicyService,
				}

				// Generate config with DNS and policy
				_, _, policy, err := service.GeneratePeerConfigWithDNS(ctx, networkID, jumpPeerID)
				if err != nil {
					t.Logf("Error generating config: %v", err)
					return false
				}

				// Verify iptables rules contain input deny rule
				if policy == nil {
					t.Logf("Policy is nil for jump peer")
					return false
				}

				foundInputDeny := false
				for _, rule := range policy.IPTablesRules {
					if strings.Contains(rule, "INPUT") && strings.Contains(rule, "DROP") {
						foundInputDeny = true
						break
					}
				}

				if !foundInputDeny {
					t.Logf("Input deny rule not found in iptables rules")
					return false
				}

				return true
			},
			genNetworkID(),
			genPeerID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 63: IPtables output deny rule**
// **Validates: Requirements 17.8**
func TestProperty_IPTablesOutputDenyRule(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 63: IPtables output deny rule",
		prop.ForAll(
			func(networkID string, jumpPeerID string) bool {
				ctx := context.Background()

				// Create mock repositories
				fullRepo := newMockFullRepository()
				authRepo := newMockAuthRepository()
				groupRepo := newMockGroupRepository()
				routeRepo := newMockRouteRepository()
				dnsRepo := newMockDNSRepository()

				// Create network
				net := &network.Network{
					ID:           networkID,
					Name:         "test-network",
					CIDR:         "10.0.0.0/24",
					Peers:        make(map[string]*network.Peer),
					DomainSuffix: "internal",
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				fullRepo.networks[networkID] = net

				// Create jump peer
				jumpPeer := &network.Peer{
					ID:        jumpPeerID,
					Name:      "jump-peer",
					Address:   "10.0.0.1",
					IsJump:    true,
					UseAgent:  true,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				net.Peers[jumpPeerID] = jumpPeer

				// Mock GetPeerGroups to return empty groups
				groupRepo.getPeerGroups = func(ctx context.Context, networkID, peerID string) ([]*network.Group, error) {
					return []*network.Group{}, nil
				}

				// Mock GetGroupRoutes to return empty routes
				groupRepo.getGroupRoutes = func(ctx context.Context, networkID, groupID string) ([]*network.Route, error) {
					return []*network.Route{}, nil
				}

				// Create mock policy service with output deny rule
				mockPolicyService := &mockPolicyService{
					rules: []string{
						"iptables -A INPUT -j DROP",
						"iptables -A OUTPUT -j DROP",
					},
				}

				// Create service
				service := &Service{
					repo:          fullRepo,
					authRepo:      authRepo,
					groupRepo:     groupRepo,
					routeRepo:     routeRepo,
					dnsRepo:       dnsRepo,
					policyService: mockPolicyService,
				}

				// Generate config with DNS and policy
				_, _, policy, err := service.GeneratePeerConfigWithDNS(ctx, networkID, jumpPeerID)
				if err != nil {
					t.Logf("Error generating config: %v", err)
					return false
				}

				// Verify iptables rules contain output deny rule
				if policy == nil {
					t.Logf("Policy is nil for jump peer")
					return false
				}

				foundOutputDeny := false
				for _, rule := range policy.IPTablesRules {
					if strings.Contains(rule, "OUTPUT") && strings.Contains(rule, "DROP") {
						foundOutputDeny = true
						break
					}
				}

				if !foundOutputDeny {
					t.Logf("Output deny rule not found in iptables rules")
					return false
				}

				return true
			},
			genNetworkID(),
			genPeerID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Mock policy service for testing
type mockPolicyService struct {
	rules []string
}

func (m *mockPolicyService) GenerateIPTablesRules(ctx context.Context, networkID, jumpPeerID string) ([]string, error) {
	return m.rules, nil
}
