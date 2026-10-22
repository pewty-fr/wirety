package route

import (
	"context"
	"strings"
	"testing"

	"wirety/internal/domain/network"
)

func newUpdateTestService() (*Service, *mockRouteRepository) {
	routeRepo := newMockRouteRepository()
	netGetter := newMockNetworkGetter()
	netGetter.networks["net"] = &network.Network{ID: "net", Name: "corp"}
	netGetter.peers["jump"] = &network.Peer{ID: "jump", Name: "jump-paris", IsJump: true}
	routeRepo.routes["rt"] = &network.Route{
		ID: "rt", NetworkID: "net", Name: "datacenter", Description: "Paris DC",
		DestinationCIDR: "10.0.0.0/16", DestinationCIDRv6: "fd00:10::/64", JumpPeerID: "jump",
	}
	return NewService(routeRepo, newMockGroupRepository(), &networkGetterAdapter{getter: netGetter}), routeRepo
}

func TestUpdateRouteClearsTheDescriptionAndADestination(t *testing.T) {
	service, _ := newUpdateTestService()
	r, err := service.UpdateRoute(context.Background(), "net", "rt", &network.RouteUpdateRequest{
		Description:       new(""),
		DestinationCIDRv6: new(""),
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.Description != "" || r.DestinationCIDRv6 != "" || r.DestinationCIDR != "10.0.0.0/16" {
		t.Errorf("got description %q, v4 %q, v6 %q", r.Description, r.DestinationCIDR, r.DestinationCIDRv6)
	}
}

func TestUpdateRouteKeepsWhatIsLeftOut(t *testing.T) {
	service, _ := newUpdateTestService()
	r, err := service.UpdateRoute(context.Background(), "net", "rt", &network.RouteUpdateRequest{Name: "dc-paris"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Description != "Paris DC" || r.DestinationCIDRv6 != "fd00:10::/64" {
		t.Errorf("got description %q, v6 %q; want them unchanged", r.Description, r.DestinationCIDRv6)
	}
}

func TestUpdateRouteKeepsOneDestination(t *testing.T) {
	service, routeRepo := newUpdateTestService()
	_, err := service.UpdateRoute(context.Background(), "net", "rt", &network.RouteUpdateRequest{
		DestinationCIDR:   new(""),
		DestinationCIDRv6: new(""),
	})
	if err == nil || !strings.Contains(err.Error(), "must remain set") {
		t.Fatalf("clearing both destinations: want an error, got %v", err)
	}
	if routeRepo.routes["rt"].DestinationCIDR != "10.0.0.0/16" {
		t.Errorf("the stored route changed: %+v", routeRepo.routes["rt"])
	}
}
