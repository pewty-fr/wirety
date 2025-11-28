package network

import "context"

// PolicyRepository defines the interface for policy data persistence
type PolicyRepository interface {
	// Policy CRUD operations
	CreatePolicy(ctx context.Context, networkID string, policy *Policy) error
	GetPolicy(ctx context.Context, networkID, policyID string) (*Policy, error)
	UpdatePolicy(ctx context.Context, networkID string, policy *Policy) error
	DeletePolicy(ctx context.Context, networkID, policyID string) error
	ListPolicies(ctx context.Context, networkID string) ([]*Policy, error)

	// Rule operations
	AddRuleToPolicy(ctx context.Context, networkID, policyID string, rule *PolicyRule) error
	RemoveRuleFromPolicy(ctx context.Context, networkID, policyID, ruleID string) error
	UpdateRule(ctx context.Context, networkID, policyID string, rule *PolicyRule) error

	// Get policies for a specific group
	GetPoliciesForGroup(ctx context.Context, networkID, groupID string) ([]*Policy, error)
}
