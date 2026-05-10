package policy

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"wirety/internal/domain/network"

	"github.com/google/uuid"
)

// stripCIDR drops the optional "/prefix" suffix from a CIDR / address string,
// returning just the host part.  Used so the rule generator can pass plain IPs
// (e.g. "10.255.236.5") to iptables when the peer.Address field carries a /32.
func stripCIDR(s string) string {
	if idx := strings.IndexByte(s, '/'); idx != -1 {
		return s[:idx]
	}
	return s
}

// isIPv6CIDR reports whether the given CIDR or IP literal is IPv6.  Unparseable
// inputs default to IPv4 (so the rule still goes through the iptables path and
// the legacy behaviour is preserved).
func isIPv6CIDR(s string) bool {
	if strings.Contains(s, ":") {
		// Quick path: anything with a colon is IPv6 (CIDR or bare).
		return true
	}
	// Fallback for unusual inputs.
	if ip, _, err := net.ParseCIDR(s); err == nil && ip.To4() == nil {
		return true
	}
	if ip := net.ParseIP(s); ip != nil && ip.To4() == nil {
		return true
	}
	return false
}

// WebSocketNotifier is an interface for notifying peers about config updates
type WebSocketNotifier interface {
	NotifyNetworkPeers(networkID string)
}

// Service implements the business logic for policy management
type Service struct {
	policyRepo network.PolicyRepository
	groupRepo  network.GroupRepository
	peerRepo   network.Repository
	routeRepo  network.RouteRepository
	wsNotifier WebSocketNotifier
}

// NewService creates a new policy service
func NewService(policyRepo network.PolicyRepository, groupRepo network.GroupRepository, peerRepo network.Repository, routeRepo network.RouteRepository) *Service {
	return &Service{
		policyRepo: policyRepo,
		groupRepo:  groupRepo,
		peerRepo:   peerRepo,
		routeRepo:  routeRepo,
	}
}

// SetWebSocketNotifier sets the WebSocket notifier for the service
func (s *Service) SetWebSocketNotifier(notifier WebSocketNotifier) {
	s.wsNotifier = notifier
}

// CreatePolicy creates a new policy with name validation
func (s *Service) CreatePolicy(ctx context.Context, networkID string, req *network.PolicyCreateRequest) (*network.Policy, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Verify network exists
	_, err := s.peerRepo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	now := time.Now()

	// Generate IDs for rules
	rules := make([]network.PolicyRule, len(req.Rules))
	for i, rule := range req.Rules {
		rules[i] = network.PolicyRule{
			ID:          uuid.New().String(),
			Direction:   rule.Direction,
			Action:      rule.Action,
			Target:      rule.Target,
			TargetType:  rule.TargetType,
			Description: rule.Description,
		}
	}

	policy := &network.Policy{
		ID:          uuid.New().String(),
		NetworkID:   networkID,
		Name:        req.Name,
		Description: req.Description,
		Rules:       rules,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.policyRepo.CreatePolicy(ctx, networkID, policy); err != nil {
		return nil, fmt.Errorf("failed to create policy: %w", err)
	}

	return policy, nil
}

// GetPolicy retrieves a policy by ID
func (s *Service) GetPolicy(ctx context.Context, networkID, policyID string) (*network.Policy, error) {
	policy, err := s.policyRepo.GetPolicy(ctx, networkID, policyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}
	return policy, nil
}

// UpdatePolicy updates an existing policy
func (s *Service) UpdatePolicy(ctx context.Context, networkID, policyID string, req *network.PolicyUpdateRequest) (*network.Policy, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing policy
	policy, err := s.policyRepo.GetPolicy(ctx, networkID, policyID)
	if err != nil {
		return nil, fmt.Errorf("policy not found: %w", err)
	}

	// Update fields
	if req.Name != "" {
		policy.Name = req.Name
	}
	if req.Description != "" {
		policy.Description = req.Description
	}
	policy.UpdatedAt = time.Now()

	if err := s.policyRepo.UpdatePolicy(ctx, networkID, policy); err != nil {
		return nil, fmt.Errorf("failed to update policy: %w", err)
	}

	// Notify all peers in the network about the policy change
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return policy, nil
}

// DeletePolicy deletes a policy
func (s *Service) DeletePolicy(ctx context.Context, networkID, policyID string) error {
	// Verify policy exists
	_, err := s.policyRepo.GetPolicy(ctx, networkID, policyID)
	if err != nil {
		return fmt.Errorf("policy not found: %w", err)
	}

	if err := s.policyRepo.DeletePolicy(ctx, networkID, policyID); err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}

	// Notify all peers in the network about the policy deletion
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// ListPolicies lists all policies in a network
func (s *Service) ListPolicies(ctx context.Context, networkID string) ([]*network.Policy, error) {
	// Verify network exists
	_, err := s.peerRepo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	policies, err := s.policyRepo.ListPolicies(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}

	return policies, nil
}

// AddRuleToPolicy adds a rule to a policy with validation
func (s *Service) AddRuleToPolicy(ctx context.Context, networkID, policyID string, rule *network.PolicyRule) error {
	// Validate rule
	if err := rule.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Generate ID for the rule if not provided
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	// Add rule to policy
	if err := s.policyRepo.AddRuleToPolicy(ctx, networkID, policyID, rule); err != nil {
		return fmt.Errorf("failed to add rule to policy: %w", err)
	}

	// Notify all peers in the network about the policy change
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// RemoveRuleFromPolicy removes a rule from a policy
func (s *Service) RemoveRuleFromPolicy(ctx context.Context, networkID, policyID, ruleID string) error {
	// Remove rule from policy
	if err := s.policyRepo.RemoveRuleFromPolicy(ctx, networkID, policyID, ruleID); err != nil {
		return fmt.Errorf("failed to remove rule from policy: %w", err)
	}

	// Notify all peers in the network about the policy change
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// GenerateIPTablesRules generates iptables rules for a jump peer based on all policies affecting it
// Rules are generated per-peer for the FORWARD chain since the jump peer routes traffic
func (s *Service) GenerateIPTablesRules(ctx context.Context, networkID, jumpPeerID string) ([]string, error) {
	// Verify jump peer exists
	jumpPeer, err := s.peerRepo.GetPeer(ctx, networkID, jumpPeerID)
	if err != nil {
		return nil, fmt.Errorf("jump peer not found: %w", err)
	}

	if !jumpPeer.IsJump {
		return nil, fmt.Errorf("peer is not a jump peer")
	}

	// Get all peers in the network
	allPeers, err := s.peerRepo.ListPeers(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list peers: %w", err)
	}

	// Generate iptables rules
	var rules []string

	// Generate rules for ALL regular peers (non-jump peers)
	// Jump peers enforce policies for all regular peers regardless of routes
	// This prevents peers from bypassing policies by modifying their WireGuard config
	for _, peer := range allPeers {
		if peer.IsJump {
			continue // Skip jump peers
		}

		// Get groups this peer belongs to
		groups, err := s.groupRepo.GetPeerGroups(ctx, networkID, peer.ID)
		if err != nil {
			// If we can't get groups, skip this peer
			continue
		}

		// Collect all policies from peer's groups (groups are ordered by priority)
		// Lower priority number = higher priority (applied first)
		// Quarantine groups have priority 0, user groups default to 100
		policyMap := make(map[string]*network.Policy)
		for _, group := range groups {
			policies, err := s.policyRepo.GetPoliciesForGroup(ctx, networkID, group.ID)
			if err != nil {
				continue
			}

			for _, policy := range policies {
				// Avoid duplicates - first occurrence wins (highest priority group)
				if _, exists := policyMap[policy.ID]; !exists {
					policyMap[policy.ID] = policy
				}
			}
		}

		// Generate rules for this peer based on their policies.
		//
		// We pass BOTH the peer's IPv4 and IPv6 addresses (when present) — the
		// generator picks the right one based on the rule target's family.
		// Mixing families (IPv4 source / IPv6 dest) produces invalid rules that
		// iptables rejects with "invalid mask 64" or similar.
		peerV4 := stripCIDR(peer.Address)
		peerV6 := stripCIDR(peer.AddressV6)
		for _, policy := range policyMap {
			for _, rule := range policy.Rules {
				peerRules := s.generateIPTablesRulesForPeer(peerV4, peerV6, rule)
				rules = append(rules, peerRules...)
			}
		}
	}

	// Add DNS rules to allow DNS queries/responses between jump server and all peers.
	// The jump server runs a DNS server, so we need to allow DNS traffic.
	// Emit one pair per address family the peer has configured.
	for _, peer := range allPeers {
		if peer.IsJump {
			continue // Skip jump peers
		}

		if v4 := stripCIDR(peer.Address); v4 != "" {
			rules = append(rules, fmt.Sprintf("iptables -A INPUT -s %s -p udp --dport 53 -j ACCEPT", v4))
			rules = append(rules, fmt.Sprintf("iptables -A OUTPUT -d %s -p udp --sport 53 -j ACCEPT", v4))
		}
		if v6 := stripCIDR(peer.AddressV6); v6 != "" {
			rules = append(rules, fmt.Sprintf("ip6tables -A INPUT -s %s -p udp --dport 53 -j ACCEPT", v6))
			rules = append(rules, fmt.Sprintf("ip6tables -A OUTPUT -d %s -p udp --sport 53 -j ACCEPT", v6))
		}
	}

	// Add WireGuard handshake rules to allow tunnel establishment.
	// WireGuard uses UDP for handshakes and keepalives, which must be allowed in
	// both directions, per address family.
	jumpPeer, err = s.peerRepo.GetPeer(ctx, networkID, jumpPeerID)
	if err == nil && jumpPeer.ListenPort > 0 {
		for _, peer := range allPeers {
			if peer.IsJump {
				continue // Skip jump peers
			}

			if v4 := stripCIDR(peer.Address); v4 != "" {
				rules = append(rules, fmt.Sprintf("iptables -A OUTPUT -d %s -p udp --sport %d -j ACCEPT", v4, jumpPeer.ListenPort))
				rules = append(rules, fmt.Sprintf("iptables -A INPUT -s %s -p udp --dport %d -j ACCEPT", v4, jumpPeer.ListenPort))
			}
			if v6 := stripCIDR(peer.AddressV6); v6 != "" {
				rules = append(rules, fmt.Sprintf("ip6tables -A OUTPUT -d %s -p udp --sport %d -j ACCEPT", v6, jumpPeer.ListenPort))
				rules = append(rules, fmt.Sprintf("ip6tables -A INPUT -s %s -p udp --dport %d -j ACCEPT", v6, jumpPeer.ListenPort))
			}
		}
	}

	// Add default deny rule at the end of the policy chain — one per family.
	// The agent rewrites these rules' chain references to WIRETY_POLICY /
	// WIRETY6_POLICY, so an authenticated peer's traffic that doesn't match any
	// allow rule in its policy gets dropped HERE, instead of falling back into
	// WIRETY_JUMP / WIRETY6_JUMP and tripping the unauthenticated-peer REJECTs.
	rules = append(rules, "iptables -A FORWARD -j DROP")
	rules = append(rules, "ip6tables -A FORWARD -j DROP")

	return rules, nil
}

// generateIPTablesRulesForPeer converts a policy rule to iptables (or ip6tables)
// commands for a specific peer.  Since the jump peer routes traffic, we use
// FORWARD chain rules with the peer's IP.
//
// The rule's target CIDR family decides which command prefix the rule carries:
//   - IPv4 CIDR target → "iptables …" with the peer's IPv4 source address
//   - IPv6 CIDR target → "ip6tables …" with the peer's IPv6 source address
//
// If the target is IPv6 but the peer has no IPv6 address configured, the rule
// is silently skipped — there is no way to express "this IPv4 peer can talk to
// this IPv6 destination" because the kernel routes by family.  Likewise an IPv4
// target with an empty peer.Address (shouldn't happen) is skipped.
//
// The agent's firewall adapter (applyIPTablesRule) detects the prefix and
// dispatches to the correct table.
func (s *Service) generateIPTablesRulesForPeer(peerV4, peerV6 string, rule network.PolicyRule) []string {
	var rules []string

	// Build the iptables rules based on target type
	switch rule.TargetType {
	case "cidr":
		isV6 := isIPv6CIDR(rule.Target)
		cmd := "iptables"
		peerIP := peerV4
		if isV6 {
			cmd = "ip6tables"
			peerIP = peerV6
		}
		if peerIP == "" {
			// Peer has no address in the target's family — rule is unrepresentable.
			return rules
		}

		// For CIDR targets, generate FORWARD rules
		switch rule.Direction {
		case "input":
			// "input" means traffic coming TO the peer (peer is receiving)
			// This translates to:
			// 1. Allow traffic FROM peer TO destination (outbound from peer's perspective)
			// 2. Allow return traffic FROM destination TO peer (established connections)

			if rule.Action == "allow" {
				// Outbound: peer → destination
				rules = append(rules, fmt.Sprintf("%s -A FORWARD -s %s -d %s -j ACCEPT", cmd, peerIP, rule.Target))

				// Return traffic: destination → peer (established connections only)
				rules = append(rules, fmt.Sprintf("%s -A FORWARD -d %s -s %s -m state --state RELATED,ESTABLISHED -j ACCEPT", cmd, peerIP, rule.Target))
			} else {
				// Deny inbound from destination to peer
				rules = append(rules, fmt.Sprintf("%s -A FORWARD -s %s -d %s -j DROP", cmd, rule.Target, peerIP))
			}
		case "output":
			// "output" means traffic going FROM the peer (peer is sending)
			// This translates to:
			// 1. Control traffic FROM peer TO destination

			if rule.Action == "allow" {
				// Allow outbound: peer → destination
				rules = append(rules, fmt.Sprintf("%s -A FORWARD -s %s -d %s -j ACCEPT", cmd, peerIP, rule.Target))

				// Allow return traffic: destination → peer (established connections only)
				rules = append(rules, fmt.Sprintf("%s -A FORWARD -d %s -s %s -m state --state RELATED,ESTABLISHED -j ACCEPT", cmd, peerIP, rule.Target))
			} else {
				// Deny outbound: peer → destination
				rules = append(rules, fmt.Sprintf("%s -A FORWARD -s %s -d %s -j DROP", cmd, peerIP, rule.Target))
			}
		}
	case "peer":
		// For peer targets, we would need to resolve the peer IP
		// TODO: Implement peer IP resolution
		rules = append(rules, fmt.Sprintf("# Peer-based rule for peer %s (requires IP resolution)", rule.Target))
	case "group":
		// For group targets, we would need to resolve all peer IPs in the group
		// TODO: Implement group member IP resolution
		rules = append(rules, fmt.Sprintf("# Group-based rule for group %s (requires IP resolution)", rule.Target))
	}

	return rules
}
