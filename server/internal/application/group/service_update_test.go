package group

import (
	"context"
	"testing"

	"wirety/internal/domain/network"
)

func TestUpdateGroupClearsOrKeepsTheDescription(t *testing.T) {
	ctx := context.Background()
	groupRepo := newMockGroupRepository()
	netGetter := newMockNetworkGetter()
	netGetter.networks["net"] = &network.Network{ID: "net", Name: "corp"}
	groupRepo.groups["grp"] = &network.Group{ID: "grp", NetworkID: "net", Name: "engineering", Description: "Developers", Priority: 100}
	service := NewService(groupRepo, &networkGetterAdapter{getter: netGetter}, newMockRouteRepository())

	g, err := service.UpdateGroup(ctx, "net", "grp", &network.GroupUpdateRequest{Priority: new(50)})
	if err != nil {
		t.Fatal(err)
	}
	if g.Description != "Developers" {
		t.Errorf("description changed to %q by an update that left it out", g.Description)
	}

	g, err = service.UpdateGroup(ctx, "net", "grp", &network.GroupUpdateRequest{Description: new("")})
	if err != nil {
		t.Fatal(err)
	}
	if g.Description != "" || g.Priority != 50 {
		t.Errorf("got description %q, priority %d; want it cleared and 50", g.Description, g.Priority)
	}
}
