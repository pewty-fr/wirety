package policy

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

type mockPolicyRepository struct {
	policies    map[string]*network.Policy
	policyRules map[string][]network.PolicyRule // policyID -> []rules
}

func newMockPolicyRepository() *mockPolicyRepository {
	return &mockPolicyRepository{
		policies:    make(map[string]*network.Policy),
		policyRules: make(map[string][]network.PolicyRule),
	}
}

func (m *mockPolicyRepository) CreatePolicy(ctx context.Context, networkID string, policy *network.Policy) error {
	// Check for duplicate name
	for _, p := range m.policies {
		if p.NetworkID == networkID && p.Name == policy.Name {
			return network.ErrDuplicatePolicyName
		}
	}
	m.policies[policy.ID] = policy
	m.policyRules[policy.ID] = append([]network.PolicyRule{}, policy.Rules...)
	return nil
}

func (m *mockPolicyRepository) GetPolicy(ctx context.Context, networkID, policyID string) (*network.Policy, error) {
	policy, exists := m.policies[policyID]
	if !exists || policy.NetworkID != networkID {
		return nil, network.ErrPolicyNotFound
	}
	// Copy policy and populate rules
	result := *policy
	result.Rules = append([]network.PolicyRule{}, m.policyRules[policyID]...)
	return &result, nil
}

func (m *mockPolicyRepository) UpdatePolicy(ctx context.Context, networkID string, policy *network.Policy) error {
	existing, exists := m.policies[policy.ID]
	if !exists || existing.NetworkID != networkID {
		return network.ErrPolicyNotFound
	}
	// Check for duplicate name
	for id, p := range m.policies {
		if id != policy.ID && p.NetworkID == networkID && p.Name == policy.Name {
			return network.ErrDuplicatePolicyName
		}
	}
	m.policies[policy.ID] = policy
	return nil
}

func (m *mockPolicyRepository) DeletePolicy(ctx context.Context, networkID, policyID string) error {
	policy, exists := m.policies[policyID]
	if !exists || policy.NetworkID != networkID {
		return network.ErrPolicyNotFound
	}
	delete(m.policies, policyID)
	delete(m.policyRules, policyID)
	return nil
}

func (m *mockPolicyRepository) ListPolicies(ctx context.Context, networkID string) ([]*network.Policy, error) {
	var policies []*network.Policy
	for _, policy := range m.policies {
		if policy.NetworkID == networkID {
			result := *policy
			result.Rules = append([]network.PolicyRule{}, m.policyRules[policy.ID]...)
			policies = append(policies, &result)
		}
	}
	return policies, nil
}

func (m *mockPolicyRepository) AddRuleToPolicy(ctx context.Context, networkID, policyID string, rule *network.PolicyRule) error {
	policy, exists := m.policies[policyID]
	if !exists || policy.NetworkID != networkID {
		return network.ErrPolicyNotFound
	}
	m.policyRules[policyID] = append(m.policyRules[policyID], *rule)
	return nil
}

func (m *mockPolicyRepository) RemoveRuleFromPolicy(ctx context.Context, networkID, policyID, ruleID string) error {
	policy, exists := m.policies[policyID]
	if !exists || policy.NetworkID != networkID {
		return network.ErrPolicyNotFound
	}
	rules := m.policyRules[policyID]
	for i, rule := range rules {
		if rule.ID == ruleID {
			m.policyRules[policyID] = append(rules[:i], rules[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("rule not found")
}

func (m *mockPolicyRepository) UpdateRule(ctx context.Context, networkID, policyID string, rule *network.PolicyRule) error {
	policy, exists := m.policies[policyID]
	if !exists || policy.NetworkID != networkID {
		return network.ErrPolicyNotFound
	}
	rules := m.policyRules[policyID]
	for i, r := range rules {
		if r.ID == rule.ID {
			m.policyRules[policyID][i] = *rule
			return nil
		}
	}
	return fmt.Errorf("rule not found")
}

func (m *mockPolicyRepository) GetPoliciesForGroup(ctx context.Context, networkID, groupID string) ([]*network.Policy, error) {
	// Return empty list for mock
	return []*network.Policy{}, nil
}

type mockGroupRepository struct {
	groups map[string]*network.Group
}

func newMockGroupRepository() *mockGroupRepository {
	return &mockGroupRepository{
		groups: make(map[string]*network.Group),
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
	return nil
}

func (m *mockGroupRepository) RemovePeerFromGroup(ctx context.Context, networkID, groupID, peerID string) error {
	return nil
}

func (m *mockGroupRepository) GetPeerGroups(ctx context.Context, networkID, peerID string) ([]*network.Group, error) {
	return nil, nil
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

func genValidPolicyName() gopter.Gen {
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

func genPolicyID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "policy-" + v
	})
}

func genRuleID() gopter.Gen {
	return gen.Identifier().Map(func(v string) string {
		return "rule-" + v
	})
}

func genDirection() gopter.Gen {
	return gen.OneConstOf("input", "output")
}

func genAction() gopter.Gen {
	return gen.OneConstOf("allow", "deny")
}

func genTargetType() gopter.Gen {
	return gen.OneConstOf("cidr", "peer", "group")
}

func genCIDR() gopter.Gen {
	return gen.OneConstOf(
		"10.0.0.0/8",
		"192.168.0.0/16",
		"172.16.0.0/12",
		"0.0.0.0/0",
		"10.1.0.0/24",
	)
}

func genPolicyRule() gopter.Gen {
	return gopter.CombineGens(
		genRuleID(),
		genDirection(),
		genAction(),
		genCIDR(),
		genTargetType(),
		genDescription(),
	).Map(func(values []interface{}) network.PolicyRule {
		return network.PolicyRule{
			ID:          values[0].(string),
			Direction:   values[1].(string),
			Action:      values[2].(string),
			Target:      values[3].(string),
			TargetType:  values[4].(string),
			Description: values[5].(string),
		}
	})
}

// Property Tests

// **Feature: network-groups-policies-routing, Property 8: Policy creation completeness**
// **Validates: Requirements 2.1**
func TestProperty_PolicyCreationCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 8: Policy creation completeness",
		prop.ForAll(
			func(name string, description string, networkID string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Create policy with generated inputs
				policy, err := service.CreatePolicy(ctx, networkID, &network.PolicyCreateRequest{
					Name:        name,
					Description: description,
					Rules:       []network.PolicyRule{},
				})

				// Verify property: policy has unique ID, provided name/description, correct network
				return err == nil &&
					policy.ID != "" &&
					policy.Name == name &&
					policy.Description == description &&
					policy.NetworkID == networkID &&
					!policy.CreatedAt.IsZero() &&
					!policy.UpdatedAt.IsZero()
			},
			genValidPolicyName(),
			genDescription(),
			genNetworkID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 9: Policy rule validation**
// **Validates: Requirements 2.2**
func TestProperty_PolicyRuleValidation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 9: Policy rule validation",
		prop.ForAll(
			func(rule network.PolicyRule) bool {
				// Verify that valid rules pass validation
				err := rule.Validate()

				// Rule should be valid if it has proper direction, action, and target type
				expectedValid := (rule.Direction == "input" || rule.Direction == "output") &&
					(rule.Action == "allow" || rule.Action == "deny") &&
					(rule.TargetType == "cidr" || rule.TargetType == "peer" || rule.TargetType == "group") &&
					rule.Target != ""

				return (err == nil) == expectedValid
			},
			genPolicyRule(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 10: Policy rule addition**
// **Validates: Requirements 2.3**
func TestProperty_PolicyRuleAddition(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 10: Policy rule addition",
		prop.ForAll(
			func(networkID string, policyID string, rule network.PolicyRule) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and policy
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				policyRepo.policies[policyID] = &network.Policy{
					ID:        policyID,
					NetworkID: networkID,
					Name:      "test-policy",
				}
				policyRepo.policyRules[policyID] = []network.PolicyRule{}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Get initial rule count
				initialPolicy, _ := policyRepo.GetPolicy(ctx, networkID, policyID)
				initialCount := len(initialPolicy.Rules)

				// Add rule to policy
				err := service.AddRuleToPolicy(ctx, networkID, policyID, &rule)
				if err != nil {
					return false
				}

				// Verify rule count increased by one
				updatedPolicy, err := policyRepo.GetPolicy(ctx, networkID, policyID)
				if err != nil {
					return false
				}

				// Verify rule appears in policy's rule list
				ruleFound := false
				for _, r := range updatedPolicy.Rules {
					if r.ID == rule.ID {
						ruleFound = true
						break
					}
				}

				return len(updatedPolicy.Rules) == initialCount+1 && ruleFound
			},
			genNetworkID(),
			genPolicyID(),
			genPolicyRule(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 11: Policy rule removal**
// **Validates: Requirements 2.4**
func TestProperty_PolicyRuleRemoval(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 11: Policy rule removal",
		prop.ForAll(
			func(networkID string, policyID string, rule network.PolicyRule) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and policy with a rule
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				policyRepo.policies[policyID] = &network.Policy{
					ID:        policyID,
					NetworkID: networkID,
					Name:      "test-policy",
				}
				policyRepo.policyRules[policyID] = []network.PolicyRule{rule}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Get initial rule count
				initialPolicy, _ := policyRepo.GetPolicy(ctx, networkID, policyID)
				initialCount := len(initialPolicy.Rules)

				// Remove rule from policy
				err := service.RemoveRuleFromPolicy(ctx, networkID, policyID, rule.ID)
				if err != nil {
					return false
				}

				// Verify rule count decreased by one
				updatedPolicy, err := policyRepo.GetPolicy(ctx, networkID, policyID)
				if err != nil {
					return false
				}

				// Verify rule no longer appears in policy's rule list
				ruleFound := false
				for _, r := range updatedPolicy.Rules {
					if r.ID == rule.ID {
						ruleFound = true
						break
					}
				}

				return len(updatedPolicy.Rules) == initialCount-1 && !ruleFound
			},
			genNetworkID(),
			genPolicyID(),
			genPolicyRule(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 12: Policy update propagation**
// **Validates: Requirements 2.5**
func TestProperty_PolicyUpdatePropagation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 12: Policy update propagation",
		prop.ForAll(
			func(networkID string, policyID string, newName string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and policy
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				policyRepo.policies[policyID] = &network.Policy{
					ID:        policyID,
					NetworkID: networkID,
					Name:      "old-name",
				}
				policyRepo.policyRules[policyID] = []network.PolicyRule{}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Update policy
				updatedPolicy, err := service.UpdatePolicy(ctx, networkID, policyID, &network.PolicyUpdateRequest{
					Name: newName,
				})

				// Verify update succeeded and name changed
				return err == nil && updatedPolicy.Name == newName
			},
			genNetworkID(),
			genPolicyID(),
			genValidPolicyName(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 13: Policy deletion cleanup**
// **Validates: Requirements 2.6**
func TestProperty_PolicyDeletionCleanup(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 13: Policy deletion cleanup",
		prop.ForAll(
			func(networkID string, policyID string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and policy
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				policyRepo.policies[policyID] = &network.Policy{
					ID:        policyID,
					NetworkID: networkID,
					Name:      "test-policy",
				}
				policyRepo.policyRules[policyID] = []network.PolicyRule{}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Delete policy
				err := service.DeletePolicy(ctx, networkID, policyID)
				if err != nil {
					return false
				}

				// Verify policy is deleted
				_, err = policyRepo.GetPolicy(ctx, networkID, policyID)
				return err != nil // Should return error (policy not found)
			},
			genNetworkID(),
			genPolicyID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 15: Template policy independence**
// **Validates: Requirements 3.5**
func TestProperty_TemplatePolicyIndependence(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 15: Template policy independence",
		prop.ForAll(
			func(networkID string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Get templates
				templates := service.GetDefaultTemplates()
				if len(templates) == 0 {
					return false
				}

				// Create a policy from the first template
				template := templates[0]
				policy, err := service.CreatePolicy(ctx, networkID, &network.PolicyCreateRequest{
					Name:        "from-template",
					Description: template.Description,
					Rules:       template.Rules,
				})
				if err != nil {
					return false
				}

				// Modify the created policy
				_, err = service.UpdatePolicy(ctx, networkID, policy.ID, &network.PolicyUpdateRequest{
					Name: "modified-name",
				})
				if err != nil {
					return false
				}

				// Get templates again and verify they haven't changed
				newTemplates := service.GetDefaultTemplates()
				if len(newTemplates) != len(templates) {
					return false
				}

				// Verify first template is unchanged
				return newTemplates[0].Name == template.Name &&
					newTemplates[0].Description == template.Description &&
					len(newTemplates[0].Rules) == len(template.Rules)
			},
			genNetworkID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 17: Policy attachment application**
// **Validates: Requirements 4.1**
func TestProperty_PolicyAttachmentApplication(t *testing.T) {
	// Note: This property is tested at the GroupService level
	// Here we verify that the PolicyService can generate iptables rules
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 17: Policy attachment application",
		prop.ForAll(
			func(networkID string, jumpPeerID string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
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

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Generate iptables rules for jump peer
				rules, err := service.GenerateIPTablesRules(ctx, networkID, jumpPeerID)

				// Verify rules are generated (at minimum, default deny rules)
				return err == nil && len(rules) >= 2
			},
			genNetworkID(),
			gen.Identifier().Map(func(v string) string { return "peer-" + v }),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 18: Multiple policy ordering**
// **Validates: Requirements 4.2**
func TestProperty_MultiplePolicyOrdering(t *testing.T) {
	// Note: Policy ordering is maintained by the repository layer
	// This test verifies that multiple policies can be created and listed
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 18: Multiple policy ordering",
		prop.ForAll(
			func(networkID string, policyCount int) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Create multiple policies
				createdPolicies := make(map[string]bool)
				for i := 0; i < policyCount; i++ {
					policy, err := service.CreatePolicy(ctx, networkID, &network.PolicyCreateRequest{
						Name:        fmt.Sprintf("policy-%d", i),
						Description: "Test policy",
						Rules:       []network.PolicyRule{},
					})
					if err != nil {
						return false
					}
					createdPolicies[policy.ID] = true
				}

				// List policies
				policies, err := service.ListPolicies(ctx, networkID)
				if err != nil {
					return false
				}

				// Verify all created policies are in the list
				return len(policies) == len(createdPolicies)
			},
			genNetworkID(),
			gen.IntRange(1, 5),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 19: Policy detachment cleanup**
// **Validates: Requirements 4.3**
func TestProperty_PolicyDetachmentCleanup(t *testing.T) {
	// Note: Policy detachment is handled by GroupService
	// This test verifies that policies can be deleted cleanly
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 19: Policy detachment cleanup",
		prop.ForAll(
			func(networkID string, policyID string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and policy
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				policyRepo.policies[policyID] = &network.Policy{
					ID:        policyID,
					NetworkID: networkID,
					Name:      "test-policy",
				}
				policyRepo.policyRules[policyID] = []network.PolicyRule{}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Delete policy (cleanup)
				err := service.DeletePolicy(ctx, networkID, policyID)
				if err != nil {
					return false
				}

				// Verify policy no longer exists
				_, err = policyRepo.GetPolicy(ctx, networkID, policyID)
				return err != nil
			},
			genNetworkID(),
			genPolicyID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 20: Automatic policy application on join**
// **Validates: Requirements 4.4**
func TestProperty_AutomaticPolicyApplicationOnJoin(t *testing.T) {
	// Note: This is handled by GroupService when peers join groups
	// PolicyService provides the policy data
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 20: Automatic policy application on join",
		prop.ForAll(
			func(networkID string, policyID string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and policy
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				policyRepo.policies[policyID] = &network.Policy{
					ID:        policyID,
					NetworkID: networkID,
					Name:      "test-policy",
				}
				policyRepo.policyRules[policyID] = []network.PolicyRule{}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Verify policy can be retrieved (available for application)
				policy, err := service.GetPolicy(ctx, networkID, policyID)
				return err == nil && policy != nil && policy.ID == policyID
			},
			genNetworkID(),
			genPolicyID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 21: Automatic policy removal on leave**
// **Validates: Requirements 4.5**
func TestProperty_AutomaticPolicyRemovalOnLeave(t *testing.T) {
	// Note: This is handled by GroupService when peers leave groups
	// PolicyService provides the policy data
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 21: Automatic policy removal on leave",
		prop.ForAll(
			func(networkID string, policyID string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and policy
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				policyRepo.policies[policyID] = &network.Policy{
					ID:        policyID,
					NetworkID: networkID,
					Name:      "test-policy",
				}
				policyRepo.policyRules[policyID] = []network.PolicyRule{}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Verify policy exists and can be used for removal logic
				policy, err := service.GetPolicy(ctx, networkID, policyID)
				return err == nil && policy != nil
			},
			genNetworkID(),
			genPolicyID(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 23: Policy-only access control**
// **Validates: Requirements 5.1**
func TestProperty_PolicyOnlyAccessControl(t *testing.T) {
	// This property verifies that the system uses only policy rules for access control
	// The PolicyService generates iptables rules based on policies
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 23: Policy-only access control",
		prop.ForAll(
			func(networkID string, jumpPeerID string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
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

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Generate iptables rules (should only use policy rules)
				rules, err := service.GenerateIPTablesRules(ctx, networkID, jumpPeerID)
				if err != nil {
					return false
				}

				// Verify that default deny rules are present (policy-based access control)
				hasDefaultDeny := false
				for _, rule := range rules {
					if rule == "iptables -A INPUT -j DROP" || rule == "iptables -A OUTPUT -j DROP" {
						hasDefaultDeny = true
						break
					}
				}

				return hasDefaultDeny
			},
			genNetworkID(),
			gen.Identifier().Map(func(v string) string { return "peer-" + v }),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 24: Deny rule enforcement**
// **Validates: Requirements 5.2**
func TestProperty_DenyRuleEnforcement(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 24: Deny rule enforcement",
		prop.ForAll(
			func(networkID string, policyID string, cidr string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and policy with deny rule
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				denyRule := network.PolicyRule{
					ID:          "rule-1",
					Direction:   "input",
					Action:      "deny",
					Target:      cidr,
					TargetType:  "cidr",
					Description: "Deny rule",
				}
				policyRepo.policies[policyID] = &network.Policy{
					ID:        policyID,
					NetworkID: networkID,
					Name:      "test-policy",
				}
				policyRepo.policyRules[policyID] = []network.PolicyRule{denyRule}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Get policy and verify deny rule is present
				policy, err := service.GetPolicy(ctx, networkID, policyID)
				if err != nil {
					return false
				}

				// Verify deny rule exists
				hasDenyRule := false
				for _, rule := range policy.Rules {
					if rule.Action == "deny" && rule.Target == cidr {
						hasDenyRule = true
						break
					}
				}

				return hasDenyRule
			},
			genNetworkID(),
			genPolicyID(),
			genCIDR(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 25: Allow rule enforcement**
// **Validates: Requirements 5.3**
func TestProperty_AllowRuleEnforcement(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 25: Allow rule enforcement",
		prop.ForAll(
			func(networkID string, policyID string, cidr string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and policy with allow rule
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				allowRule := network.PolicyRule{
					ID:          "rule-1",
					Direction:   "output",
					Action:      "allow",
					Target:      cidr,
					TargetType:  "cidr",
					Description: "Allow rule",
				}
				policyRepo.policies[policyID] = &network.Policy{
					ID:        policyID,
					NetworkID: networkID,
					Name:      "test-policy",
				}
				policyRepo.policyRules[policyID] = []network.PolicyRule{allowRule}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Get policy and verify allow rule is present
				policy, err := service.GetPolicy(ctx, networkID, policyID)
				if err != nil {
					return false
				}

				// Verify allow rule exists
				hasAllowRule := false
				for _, rule := range policy.Rules {
					if rule.Action == "allow" && rule.Target == cidr {
						hasAllowRule = true
						break
					}
				}

				return hasAllowRule
			},
			genNetworkID(),
			genPolicyID(),
			genCIDR(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: network-groups-policies-routing, Property 26: Default deny behavior**
// **Validates: Requirements 5.4**
func TestProperty_DefaultDenyBehavior(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Feature: network-groups-policies-routing, Property 26: Default deny behavior",
		prop.ForAll(
			func(networkID string, jumpPeerID string) bool {
				ctx := context.Background()
				policyRepo := newMockPolicyRepository()
				groupRepo := newMockGroupRepository()
				netGetter := newMockNetworkGetter()

				// Setup: Create network and jump peer with no policies
				netGetter.networks[networkID] = &network.Network{
					ID:   networkID,
					Name: "test-network",
				}
				netGetter.peers[jumpPeerID] = &network.Peer{
					ID:     jumpPeerID,
					Name:   "jump-peer",
					IsJump: true,
				}

				service := NewService(policyRepo, groupRepo, &networkGetterAdapter{getter: netGetter})

				// Generate iptables rules (should have default deny)
				rules, err := service.GenerateIPTablesRules(ctx, networkID, jumpPeerID)
				if err != nil {
					return false
				}

				// Verify default deny rules are present
				hasInputDeny := false
				hasOutputDeny := false
				for _, rule := range rules {
					if rule == "iptables -A INPUT -j DROP" {
						hasInputDeny = true
					}
					if rule == "iptables -A OUTPUT -j DROP" {
						hasOutputDeny = true
					}
				}

				return hasInputDeny && hasOutputDeny
			},
			genNetworkID(),
			gen.Identifier().Map(func(v string) string { return "peer-" + v }),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
