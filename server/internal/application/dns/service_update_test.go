package dns

import (
	"context"
	"strings"
	"testing"

	"wirety/internal/domain/network"
)

func newDNSUpdateTestService(t *testing.T) (*Service, *mockDNSRepository) {
	t.Helper()
	dnsRepo := newMockDNSRepository()
	routeRepo := newMockRouteRepository()
	route := &network.Route{
		ID: "rt", NetworkID: "net", Name: "datacenter",
		DestinationCIDR: "10.0.0.0/16", DestinationCIDRv6: "fd00:10::/64",
	}
	if err := routeRepo.CreateRoute(context.Background(), "net", route); err != nil {
		t.Fatal(err)
	}
	dnsRepo.mappings["nas"] = &network.DNSMapping{ID: "nas", RouteID: "rt", Name: "nas", IPAddress: "10.0.0.50", IPv6Address: "fd00:10::50"}
	return NewService(dnsRepo, routeRepo, newMockPeerRepository()), dnsRepo
}

func TestUpdateDNSMappingClearsAnAddress(t *testing.T) {
	service, _ := newDNSUpdateTestService(t)
	m, err := service.UpdateDNSMapping(context.Background(), "net", "rt", "nas", &network.DNSMappingUpdateRequest{IPv6Address: new("")})
	if err != nil {
		t.Fatal(err)
	}
	if m.IPv6Address != "" || m.IPAddress != "10.0.0.50" {
		t.Errorf("got v4 %q, v6 %q; want 10.0.0.50 and no IPv6", m.IPAddress, m.IPv6Address)
	}
}

func TestUpdateDNSMappingKeepsOneAddress(t *testing.T) {
	service, dnsRepo := newDNSUpdateTestService(t)
	_, err := service.UpdateDNSMapping(context.Background(), "net", "rt", "nas", &network.DNSMappingUpdateRequest{
		IPAddress:   new(""),
		IPv6Address: new(""),
	})
	if err == nil || !strings.Contains(err.Error(), "must remain set") {
		t.Fatalf("clearing both addresses: want an error, got %v", err)
	}
	if dnsRepo.mappings["nas"].IPAddress != "10.0.0.50" {
		t.Errorf("the stored record changed: %+v", dnsRepo.mappings["nas"])
	}
}
