package policy

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

// Service implements the business logic for policy management
type Service struct {
	policyRepo network.PolicyRepository
	groupRepo  network.GroupRepository
	peerRepo   network.Repository
	wsNotifier WebSocketNotifier
}

// NewService creates a new policy service
func NewService(policyRepo network.PolicyRepository, groupRepo network.GroupRepository, peerRepo network.Repository) *Service {
	return &Service{
		policyRepo: policyRepo,
		groupRepo:  groupRepo,
		peerRepo:   peerRepo,
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

// PolicyTemplate represents a predefined policy template
type PolicyTemplate struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Rules       []network.PolicyRule `json:"rules"`
}

// GetDefaultTemplates returns predefined policy templates
func (s *Service) GetDefaultTemplates() []PolicyTemplate {
	return []PolicyTemplate{
		{
			Name:        "Fully Encapsulated",
			Description: "Allows all outbound traffic and denies all inbound traffic",
			Rules: []network.PolicyRule{
				{
					Direction:   "output",
					Action:      "allow",
					Target:      "0.0.0.0/0",
					TargetType:  "cidr",
					Description: "Allow all outbound traffic",
				},
				{
					Direction:   "input",
					Action:      "deny",
					Target:      "0.0.0.0/0",
					TargetType:  "cidr",
					Description: "Deny all inbound traffic",
				},
			},
		},
		{
			Name:        "Isolated",
			Description: "Denies all inbound and outbound traffic",
			Rules: []network.PolicyRule{
				{
					Direction:   "input",
					Action:      "deny",
					Target:      "0.0.0.0/0",
					TargetType:  "cidr",
					Description: "Deny all inbound traffic",
				},
				{
					Direction:   "output",
					Action:      "deny",
					Target:      "0.0.0.0/0",
					TargetType:  "cidr",
					Description: "Deny all outbound traffic",
				},
			},
		},
		{
			Name:        "Default Network",
			Description: "Allows all traffic within the network CIDR",
			Rules: []network.PolicyRule{
				{
					Direction:   "input",
					Action:      "allow",
					Target:      "{{NETWORK_CIDR}}",
					TargetType:  "cidr",
					Description: "Allow inbound traffic from network",
				},
				{
					Direction:   "output",
					Action:      "allow",
					Target:      "{{NETWORK_CIDR}}",
					TargetType:  "cidr",
					Description: "Allow outbound traffic to network",
				},
			},
		},
	}
}

// GenerateIPTablesRules generates iptables rules for a jump peer based on all policies affecting it
func (s *Service) GenerateIPTablesRules(ctx context.Context, networkID, jumpPeerID string) ([]string, error) {
	// Verify jump peer exists
	peer, err := s.peerRepo.GetPeer(ctx, networkID, jumpPeerID)
	if err != nil {
		return nil, fmt.Errorf("jump peer not found: %w", err)
	}

	if !peer.IsJump {
		return nil, fmt.Errorf("peer is not a jump peer")
	}

	// Get all groups in the network
	groups, err := s.groupRepo.ListGroups(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	// Collect all policies from all groups
	policyMap := make(map[string]*network.Policy)
	var orderedPolicies []*network.Policy

	for _, group := range groups {
		policies, err := s.policyRepo.GetPoliciesForGroup(ctx, networkID, group.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get policies for group %s: %w", group.ID, err)
		}

		for _, policy := range policies {
			// Avoid duplicates
			if _, exists := policyMap[policy.ID]; !exists {
				policyMap[policy.ID] = policy
				orderedPolicies = append(orderedPolicies, policy)
			}
		}
	}

	// Generate iptables rules
	var rules []string

	// Add rules from each policy
	for _, policy := range orderedPolicies {
		for _, rule := range policy.Rules {
			iptablesRule := s.generateIPTablesRule(rule, networkID)
			if iptablesRule != "" {
				rules = append(rules, iptablesRule)
			}
		}
	}

	// Add default deny rules at the end
	rules = append(rules, "iptables -A INPUT -j DROP")
	rules = append(rules, "iptables -A OUTPUT -j DROP")

	return rules, nil
}

// generateIPTablesRule converts a policy rule to an iptables command
func (s *Service) generateIPTablesRule(rule network.PolicyRule, networkID string) string {
	// Determine the chain based on direction
	chain := "INPUT"
	if rule.Direction == "output" {
		chain = "OUTPUT"
	}

	// Determine the target based on action
	target := "ACCEPT"
	if rule.Action == "deny" {
		target = "DROP"
	}

	// Build the iptables rule based on target type
	switch rule.TargetType {
	case "cidr":
		// For CIDR targets, use source/destination based on direction
		if rule.Direction == "input" {
			return fmt.Sprintf("iptables -A %s -s %s -j %s", chain, rule.Target, target)
		}
		return fmt.Sprintf("iptables -A %s -d %s -j %s", chain, rule.Target, target)
	case "peer":
		// For peer targets, we would need to resolve the peer IP
		// This is a simplified version - in production, you'd look up the peer's IP
		return fmt.Sprintf("# Peer-based rule for peer %s (requires IP resolution)", rule.Target)
	case "group":
		// For group targets, we would need to resolve all peer IPs in the group
		// This is a simplified version - in production, you'd look up all group member IPs
		return fmt.Sprintf("# Group-based rule for group %s (requires IP resolution)", rule.Target)
	default:
		return ""
	}
}
