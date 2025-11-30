package api

import (
	"context"

	"wirety/internal/application/policy"
	"wirety/internal/domain/network"
)

// policyServiceAdapter adapts the policy service to the API interface
type policyServiceAdapter struct {
	service *policy.Service
}

// NewPolicyServiceAdapter creates a new policy service adapter
func NewPolicyServiceAdapter(service *policy.Service) PolicyService {
	return &policyServiceAdapter{service: service}
}

func (a *policyServiceAdapter) CreatePolicy(ctx context.Context, networkID string, req *network.PolicyCreateRequest) (*network.Policy, error) {
	return a.service.CreatePolicy(ctx, networkID, req)
}

func (a *policyServiceAdapter) GetPolicy(ctx context.Context, networkID, policyID string) (*network.Policy, error) {
	return a.service.GetPolicy(ctx, networkID, policyID)
}

func (a *policyServiceAdapter) UpdatePolicy(ctx context.Context, networkID, policyID string, req *network.PolicyUpdateRequest) (*network.Policy, error) {
	return a.service.UpdatePolicy(ctx, networkID, policyID, req)
}

func (a *policyServiceAdapter) DeletePolicy(ctx context.Context, networkID, policyID string) error {
	return a.service.DeletePolicy(ctx, networkID, policyID)
}

func (a *policyServiceAdapter) ListPolicies(ctx context.Context, networkID string) ([]*network.Policy, error) {
	return a.service.ListPolicies(ctx, networkID)
}

func (a *policyServiceAdapter) AddRuleToPolicy(ctx context.Context, networkID, policyID string, rule *network.PolicyRule) error {
	return a.service.AddRuleToPolicy(ctx, networkID, policyID, rule)
}

func (a *policyServiceAdapter) RemoveRuleFromPolicy(ctx context.Context, networkID, policyID, ruleID string) error {
	return a.service.RemoveRuleFromPolicy(ctx, networkID, policyID, ruleID)
}

func (a *policyServiceAdapter) GetDefaultTemplates() []PolicyTemplate {
	templates := a.service.GetDefaultTemplates()
	result := make([]PolicyTemplate, len(templates))
	for i, t := range templates {
		result[i] = PolicyTemplate{
			Name:        t.Name,
			Description: t.Description,
			Rules:       t.Rules,
		}
	}
	return result
}
