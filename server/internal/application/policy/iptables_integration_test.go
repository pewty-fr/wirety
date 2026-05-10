//go:build integration

package policy

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"wirety/internal/domain/network"
)

// integrationGroupRepo extends mockGroupRepository with configurable GetPeerGroups data.
type integrationGroupRepo struct {
	mockGroupRepository
	peerGroups    map[string][]*network.Group  // peerID -> ordered groups (by priority)
	groupPolicies map[string][]*network.Policy // groupID -> policies
}

func newIntegrationGroupRepo() *integrationGroupRepo {
	return &integrationGroupRepo{
		mockGroupRepository: *newMockGroupRepository(),
		peerGroups:          make(map[string][]*network.Group),
		groupPolicies:       make(map[string][]*network.Policy),
	}
}

func (r *integrationGroupRepo) GetPeerGroups(ctx context.Context, networkID, peerID string) ([]*network.Group, error) {
	return r.peerGroups[peerID], nil
}

// integrationPolicyRepo extends mockPolicyRepository with configurable GetPoliciesForGroup data.
type integrationPolicyRepo struct {
	mockPolicyRepository
	groupPolicies map[string][]*network.Policy // groupID -> policies
}

func newIntegrationPolicyRepo() *integrationPolicyRepo {
	return &integrationPolicyRepo{
		mockPolicyRepository: *newMockPolicyRepository(),
		groupPolicies:        make(map[string][]*network.Policy),
	}
}

func (r *integrationPolicyRepo) GetPoliciesForGroup(ctx context.Context, networkID, groupID string) ([]*network.Policy, error) {
	return r.groupPolicies[groupID], nil
}

// ruleGenFixture wires together a Service with configurable in-memory data.
type ruleGenFixture struct {
	networkID  string
	jumpPeerID string
	peer1ID    string
	peer2ID    string

	svc       *Service
	peerRepo  *networkGetterAdapter
	groupRepo *integrationGroupRepo
	polRepo   *integrationPolicyRepo
	routeRepo *mockRouteRepository
}

func newRuleGenFixture() *ruleGenFixture {
	const (
		networkID  = "net-1"
		jumpPeerID = "jump-1"
		peer1ID    = "peer-1"
		peer2ID    = "peer-2"
	)

	getter := newMockNetworkGetter()
	peerRepo := &networkGetterAdapter{getter: getter}
	groupRepo := newIntegrationGroupRepo()
	polRepo := newIntegrationPolicyRepo()
	routeRepo := newMockRouteRepository()

	now := time.Now()
	getter.networks[networkID] = &network.Network{ID: networkID, Name: "test-net", CIDR: "10.100.0.0/24"}
	getter.peers[jumpPeerID] = &network.Peer{
		ID:         jumpPeerID,
		Name:       "jump",
		Address:    "10.100.0.1",
		IsJump:     true,
		ListenPort: 51820,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	getter.peers[peer1ID] = &network.Peer{
		ID:        peer1ID,
		Name:      "peer1",
		Address:   "10.100.0.2",
		IsJump:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	getter.peers[peer2ID] = &network.Peer{
		ID:        peer2ID,
		Name:      "peer2",
		Address:   "10.100.0.3",
		IsJump:    false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	svc := NewService(polRepo, groupRepo, peerRepo, routeRepo)

	return &ruleGenFixture{
		networkID: networkID, jumpPeerID: jumpPeerID,
		peer1ID: peer1ID, peer2ID: peer2ID,
		svc: svc, peerRepo: peerRepo,
		groupRepo: groupRepo, polRepo: polRepo,
		routeRepo: routeRepo,
	}
}

// addPeerPolicy creates a group+policy and wires them to a peer.
func (f *ruleGenFixture) addPeerPolicy(peerID, groupID string, priority int, pol *network.Policy) {
	group := &network.Group{
		ID:        groupID,
		NetworkID: f.networkID,
		Name:      groupID,
		Priority:  priority,
	}
	f.groupRepo.groups[groupID] = group
	f.groupRepo.peerGroups[peerID] = append(f.groupRepo.peerGroups[peerID], group)
	f.polRepo.groupPolicies[groupID] = append(f.polRepo.groupPolicies[groupID], pol)
}

func mustPolicy(id, name string, rules ...network.PolicyRule) *network.Policy {
	return &network.Policy{
		ID:        id,
		NetworkID: "net-1",
		Name:      name,
		Rules:     rules,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func mustRule(id, direction, action, targetType, target string) network.PolicyRule {
	return network.PolicyRule{
		ID:         id,
		Direction:  direction,
		Action:     action,
		TargetType: targetType,
		Target:     target,
	}
}

// containsRule checks that the rule slice contains a rule string.
func containsRule(rules []string, substr string) bool {
	for _, r := range rules {
		if strings.Contains(r, substr) {
			return true
		}
	}
	return false
}

// findRule returns the exact rule string containing substr, or "".
func findRule(rules []string, substr string) string {
	for _, r := range rules {
		if strings.Contains(r, substr) {
			return r
		}
	}
	return ""
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestRuleGen_NonJumpPeer_FailsIfNotJump ensures GenerateIPTablesRules rejects non-jump peers.
func TestRuleGen_NonJumpPeer_FailsIfNotJump(t *testing.T) {
	f := newRuleGenFixture()
	_, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.peer1ID)
	if err == nil {
		t.Fatal("expected error for non-jump peer, got nil")
	}
}

// TestRuleGen_NoPolicies_OnlyDNSAndWireGuardAndDrop verifies the baseline output when
// no policies are attached: DNS rules + WireGuard handshake rules + FORWARD DROP.
func TestRuleGen_NoPolicies_OnlyDNSAndWireGuardAndDrop(t *testing.T) {
	f := newRuleGenFixture()
	rules, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.jumpPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// DNS: one INPUT and one OUTPUT per regular peer (2 peers × 2 = 4 rules)
	checkCount(t, rules, "udp --dport 53 -j ACCEPT", 2, "DNS INPUT rules")
	checkCount(t, rules, "udp --sport 53 -j ACCEPT", 2, "DNS OUTPUT rules")

	// WireGuard: one OUTPUT and one INPUT per regular peer (2 × 2 = 4 rules)
	checkCount(t, rules, "--sport 51820 -j ACCEPT", 2, "WG OUTPUT rules")
	checkCount(t, rules, "--dport 51820 -j ACCEPT", 2, "WG INPUT rules")

	// Default deny at the end — one per family.  The IPv6 DROP was added
	// alongside dual-stack policy support (the IPv6 policy chain wouldn't
	// terminate the way the IPv4 one did, allowing unmatched authenticated
	// traffic to fall back into WIRETY6_JUMP and trip the unauthenticated
	// REJECTs — see the policy/service.go FORWARD-DROP block).  Both
	// drops are emitted at the very end; we don't constrain ordering between
	// them, only that they are the final two.
	if len(rules) < 2 {
		t.Fatalf("expected at least 2 rules, got %d", len(rules))
	}
	tail := map[string]bool{
		rules[len(rules)-1]: true,
		rules[len(rules)-2]: true,
	}
	if !tail["iptables -A FORWARD -j DROP"] {
		t.Errorf("expected 'iptables -A FORWARD -j DROP' at the tail, got %v", rules[len(rules)-2:])
	}
	if !tail["ip6tables -A FORWARD -j DROP"] {
		t.Errorf("expected 'ip6tables -A FORWARD -j DROP' at the tail, got %v", rules[len(rules)-2:])
	}
}

// TestRuleGen_AllowOutputCIDR checks that an allow-output CIDR policy generates the
// correct pair of FORWARD ACCEPT rules for the peer.
func TestRuleGen_AllowOutputCIDR(t *testing.T) {
	f := newRuleGenFixture()
	f.addPeerPolicy(f.peer1ID, "g1", 100,
		mustPolicy("pol1", "web-allow",
			mustRule("r1", "output", "allow", "cidr", "192.168.1.0/24"),
		),
	)

	rules, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.jumpPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peerIP := "10.100.0.2"
	target := "192.168.1.0/24"

	wantOutbound := fmt.Sprintf("iptables -A FORWARD -s %s -d %s -j ACCEPT", peerIP, target)
	if !containsRule(rules, wantOutbound) {
		t.Errorf("missing outbound ACCEPT rule %q in:\n%s", wantOutbound, strings.Join(rules, "\n"))
	}

	// Return traffic: RELATED,ESTABLISHED
	if !containsRule(rules, fmt.Sprintf("-d %s -s %s -m state --state RELATED,ESTABLISHED -j ACCEPT", peerIP, target)) {
		t.Errorf("missing ESTABLISHED return rule in:\n%s", strings.Join(rules, "\n"))
	}
}

// TestRuleGen_DenyOutputCIDR checks that a deny-output rule generates a single DROP rule.
func TestRuleGen_DenyOutputCIDR(t *testing.T) {
	f := newRuleGenFixture()
	f.addPeerPolicy(f.peer1ID, "g1", 100,
		mustPolicy("pol1", "block-rfc1918",
			mustRule("r1", "output", "deny", "cidr", "10.0.0.0/8"),
		),
	)

	rules, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.jumpPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peerIP := "10.100.0.2"
	target := "10.0.0.0/8"
	wantDrop := fmt.Sprintf("iptables -A FORWARD -s %s -d %s -j DROP", peerIP, target)
	if !containsRule(rules, wantDrop) {
		t.Errorf("missing DROP rule %q in:\n%s", wantDrop, strings.Join(rules, "\n"))
	}

	// Verify no ACCEPT rule for this CIDR
	if containsRule(rules, fmt.Sprintf("-d %s -j ACCEPT", target)) {
		t.Errorf("unexpected ACCEPT rule for denied CIDR %s", target)
	}
}

// TestRuleGen_AllowInputCIDR checks that an allow-input rule generates FORWARD ACCEPT rules.
func TestRuleGen_AllowInputCIDR(t *testing.T) {
	f := newRuleGenFixture()
	f.addPeerPolicy(f.peer1ID, "g1", 100,
		mustPolicy("pol1", "allow-mgmt",
			mustRule("r1", "input", "allow", "cidr", "172.16.0.0/12"),
		),
	)

	rules, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.jumpPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peerIP := "10.100.0.2"
	target := "172.16.0.0/12"
	if !containsRule(rules, fmt.Sprintf("-s %s -d %s -j ACCEPT", peerIP, target)) {
		t.Errorf("missing ACCEPT rule for input allow in:\n%s", strings.Join(rules, "\n"))
	}
}

// TestRuleGen_DenyInputCIDR checks the deny-input direction: DROP inbound from the CIDR to the peer.
func TestRuleGen_DenyInputCIDR(t *testing.T) {
	f := newRuleGenFixture()
	f.addPeerPolicy(f.peer1ID, "g1", 100,
		mustPolicy("pol1", "deny-inbound",
			mustRule("r1", "input", "deny", "cidr", "10.0.0.0/8"),
		),
	)

	rules, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.jumpPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peerIP := "10.100.0.2"
	// deny-input: block traffic FROM target TO peer
	wantDrop := fmt.Sprintf("iptables -A FORWARD -s 10.0.0.0/8 -d %s -j DROP", peerIP)
	if !containsRule(rules, wantDrop) {
		t.Errorf("missing inbound DROP rule %q in:\n%s", wantDrop, strings.Join(rules, "\n"))
	}
}

// TestRuleGen_DNSRulesPerPeer verifies that each non-jump peer gets DNS INPUT/OUTPUT rules
// pointing to/from the jump peer.
func TestRuleGen_DNSRulesPerPeer(t *testing.T) {
	f := newRuleGenFixture()
	rules, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.jumpPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peers := []struct{ id, addr string }{
		{f.peer1ID, "10.100.0.2"},
		{f.peer2ID, "10.100.0.3"},
	}
	for _, p := range peers {
		inputRule := fmt.Sprintf("iptables -A INPUT -s %s -p udp --dport 53 -j ACCEPT", p.addr)
		if !containsRule(rules, inputRule) {
			t.Errorf("missing DNS INPUT rule for %s", p.addr)
		}
		outputRule := fmt.Sprintf("iptables -A OUTPUT -d %s -p udp --sport 53 -j ACCEPT", p.addr)
		if !containsRule(rules, outputRule) {
			t.Errorf("missing DNS OUTPUT rule for %s", p.addr)
		}
	}
}

// TestRuleGen_WireGuardHandshakeRules verifies WireGuard handshake rules for each peer.
func TestRuleGen_WireGuardHandshakeRules(t *testing.T) {
	f := newRuleGenFixture()
	rules, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.jumpPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peers := []string{"10.100.0.2", "10.100.0.3"}
	for _, addr := range peers {
		outbound := fmt.Sprintf("iptables -A OUTPUT -d %s -p udp --sport 51820 -j ACCEPT", addr)
		inbound := fmt.Sprintf("iptables -A INPUT -s %s -p udp --dport 51820 -j ACCEPT", addr)
		if !containsRule(rules, outbound) {
			t.Errorf("missing WireGuard OUTPUT rule for %s", addr)
		}
		if !containsRule(rules, inbound) {
			t.Errorf("missing WireGuard INPUT rule for %s", addr)
		}
	}
}

// TestRuleGen_DefaultDenyAtEnd verifies the unconditional FORWARD DROP is always last.
func TestRuleGen_DefaultDenyAtEnd(t *testing.T) {
	f := newRuleGenFixture()
	f.addPeerPolicy(f.peer1ID, "g1", 100,
		mustPolicy("pol1", "some-policy",
			mustRule("r1", "output", "allow", "cidr", "192.168.0.0/16"),
		),
	)
	rules, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.jumpPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) < 2 {
		t.Fatalf("expected at least 2 rules, got %d", len(rules))
	}
	// Both per-family FORWARD DROPs at the end (see comment on
	// TestRuleGen_NoPolicies_OnlyDNSAndWireGuardAndDrop for why we now emit
	// the IPv6 one too).
	tail := map[string]bool{
		rules[len(rules)-1]: true,
		rules[len(rules)-2]: true,
	}
	if !tail["iptables -A FORWARD -j DROP"] {
		t.Errorf("expected 'iptables -A FORWARD -j DROP' at the tail, got %v", rules[len(rules)-2:])
	}
	if !tail["ip6tables -A FORWARD -j DROP"] {
		t.Errorf("expected 'ip6tables -A FORWARD -j DROP' at the tail, got %v", rules[len(rules)-2:])
	}
}

// TestRuleGen_GroupPriorityDedup verifies that when a policy appears in two groups,
// only the copy from the highest-priority group (lowest priority number) is effective.
// Since both groups reference the same policy ID, it should be applied exactly once.
func TestRuleGen_GroupPriorityDedup(t *testing.T) {
	f := newRuleGenFixture()
	pol := mustPolicy("shared-pol", "shared",
		mustRule("r1", "output", "allow", "cidr", "192.168.99.0/24"),
	)

	// Group g1 has priority 50 (higher), g2 has priority 100 (lower).
	// Both reference the same policy. The dedup map should include it only once.
	g1 := &network.Group{ID: "g1", NetworkID: f.networkID, Name: "g1", Priority: 50}
	g2 := &network.Group{ID: "g2", NetworkID: f.networkID, Name: "g2", Priority: 100}

	f.groupRepo.groups["g1"] = g1
	f.groupRepo.groups["g2"] = g2
	// Return g1 before g2 (priority order) to match service expectation
	f.groupRepo.peerGroups[f.peer1ID] = []*network.Group{g1, g2}
	f.polRepo.groupPolicies["g1"] = []*network.Policy{pol}
	f.polRepo.groupPolicies["g2"] = []*network.Policy{pol} // same policy

	rules, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.jumpPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count occurrences of the specific ACCEPT rule for the shared policy target
	count := 0
	target := "192.168.99.0/24"
	for _, r := range rules {
		if strings.Contains(r, target) && strings.Contains(r, "-j ACCEPT") && strings.Contains(r, "-s 10.100.0.2") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("shared policy rule should appear exactly once, got %d", count)
	}
}

// TestRuleGen_MultiplePeers_IndependentRules verifies that policies on peer1 do not
// generate rules for peer2's IP.
func TestRuleGen_MultiplePeers_IndependentRules(t *testing.T) {
	f := newRuleGenFixture()
	f.addPeerPolicy(f.peer1ID, "g1", 100,
		mustPolicy("pol1", "peer1-only",
			mustRule("r1", "output", "deny", "cidr", "10.0.0.0/8"),
		),
	)
	// peer2 has no policies

	rules, err := f.svc.GenerateIPTablesRules(context.Background(), f.networkID, f.jumpPeerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// DROP rule should reference peer1's IP
	if !containsRule(rules, fmt.Sprintf("-s 10.100.0.2 -d 10.0.0.0/8 -j DROP")) {
		t.Errorf("missing DROP rule for peer1 in:\n%s", strings.Join(rules, "\n"))
	}

	// The DROP rule must NOT reference peer2's IP as the source for that CIDR
	if containsRule(rules, fmt.Sprintf("-s 10.100.0.3 -d 10.0.0.0/8 -j DROP")) {
		t.Errorf("peer2 should not have a policy-based DROP rule (no policies attached)")
	}
}

// checkCount asserts the number of rules containing substr equals want.
func checkCount(t *testing.T, rules []string, substr string, want int, label string) {
	t.Helper()
	n := 0
	for _, r := range rules {
		if strings.Contains(r, substr) {
			n++
		}
	}
	if n != want {
		t.Errorf("%s: expected %d rules containing %q, got %d\n  rules:\n  %s",
			label, want, substr, n, strings.Join(rules, "\n  "))
	}
}

