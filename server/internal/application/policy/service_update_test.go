package policy

import (
	"context"
	"testing"

	"wirety/internal/domain/network"
)

func TestUpdatePolicyClearsOrKeepsTheDescription(t *testing.T) {
	ctx := context.Background()
	policyRepo := newMockPolicyRepository()
	netGetter := newMockNetworkGetter()
	netGetter.networks["net"] = &network.Network{ID: "net", Name: "corp"}
	policyRepo.policies["pol"] = &network.Policy{ID: "pol", NetworkID: "net", Name: "web", Description: "HTTP only"}
	policyRepo.policyRules["pol"] = []network.PolicyRule{}
	service := NewService(policyRepo, newMockGroupRepository(), &networkGetterAdapter{getter: netGetter}, newMockRouteRepository())

	p, err := service.UpdatePolicy(ctx, "net", "pol", &network.PolicyUpdateRequest{Name: "web-2"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Description != "HTTP only" {
		t.Errorf("description changed to %q by an update that left it out", p.Description)
	}

	p, err = service.UpdatePolicy(ctx, "net", "pol", &network.PolicyUpdateRequest{Description: new("")})
	if err != nil {
		t.Fatal(err)
	}
	if p.Description != "" || p.Name != "web-2" {
		t.Errorf("got name %q, description %q; want web-2 and an empty description", p.Name, p.Description)
	}
}
