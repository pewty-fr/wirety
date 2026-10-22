package network

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"wirety/internal/adapters/db/memory"
	"wirety/internal/domain/network"
)

func newRangesTestService(t *testing.T, repo network.Repository) (*Service, *memory.IPAMRepository) {
	t.Helper()
	if repo == nil {
		repo = memory.NewRepository()
	}
	ipamRepo := memory.NewIPAMRepository(context.Background())
	return NewService(repo, ipamRepo, nil, nil, nil, nil, nil), ipamRepo
}

func mustCreateNetwork(t *testing.T, s *Service, name, cidr, cidrV6 string) *network.Network {
	t.Helper()
	n, err := s.CreateNetwork(context.Background(), &network.NetworkCreateRequest{Name: name, CIDR: cidr, CIDRv6: cidrV6})
	if err != nil {
		t.Fatalf("create network %s: %v", name, err)
	}
	return n
}

func mustAddPeer(t *testing.T, s *Service, networkID, name string) *network.Peer {
	t.Helper()
	p, err := s.AddPeer(context.Background(), networkID, &network.PeerCreateRequest{Name: name, UseAgent: true}, "")
	if err != nil {
		t.Fatalf("add peer %s: %v", name, err)
	}
	return p
}

// addresses maps peer names to their IPv4 (or IPv6) address.
func addresses(t *testing.T, s *Service, networkID string, v6 bool) map[string]string {
	t.Helper()
	peers, err := s.repo.ListPeers(context.Background(), networkID)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, p := range peers {
		if v6 {
			out[p.Name] = p.AddressV6
		} else {
			out[p.Name] = p.Address
		}
	}
	return out
}

func TestCreateNetworkRefusesRangeOfAnotherNetwork(t *testing.T) {
	ctx := context.Background()
	s, _ := newRangesTestService(t, nil)
	mustCreateNetwork(t, s, "prod", "10.10.0.0/24", "fd10::/64")

	for _, req := range []network.NetworkCreateRequest{
		{Name: "same", CIDR: "10.10.0.0/24"},
		{Name: "wider", CIDR: "10.10.0.0/16"},
		{Name: "inside", CIDR: "10.10.0.128/25"},
		{Name: "v6", CIDR: "10.11.0.0/24", CIDRv6: "fd10::/48"},
	} {
		_, err := s.CreateNetwork(ctx, &req)
		if !errors.Is(err, network.ErrCIDROverlap) || !strings.Contains(err.Error(), "“prod”") {
			t.Errorf("%s: err = %v, want an overlap with “prod”", req.Name, err)
		}
	}
	if networks, _ := s.ListNetworks(ctx); len(networks) != 1 {
		t.Errorf("%d networks, want only prod", len(networks))
	}
	// A range of its own is fine, and was not reserved by the refused attempts.
	mustCreateNetwork(t, s, "dev", "10.11.0.0/24", "fd11::/64")
}

func TestCreateNetworkValidatesRanges(t *testing.T) {
	s, _ := newRangesTestService(t, nil)
	for _, req := range []network.NetworkCreateRequest{
		{Name: "a", CIDR: "fd00::/64"},
		{Name: "b", CIDRv6: "10.0.0.0/24"},
		{Name: "c", CIDR: "10.0.0.0/31"},
		{Name: "d", CIDR: "10.0.0.1/24"},
		{Name: "e", CIDR: "nope"},
	} {
		if _, err := s.CreateNetwork(context.Background(), &req); !errors.Is(err, network.ErrInvalidCIDR) {
			t.Errorf("%+v: err = %v, want ErrInvalidCIDR", req, err)
		}
	}
}

// failingCreateRepo refuses to store networks.
type failingCreateRepo struct{ *memory.Repository }

func (failingCreateRepo) CreateNetwork(context.Context, *network.Network) error {
	return errors.New("database is down")
}

func TestCreateNetworkFreesRangesWhenStoringFails(t *testing.T) {
	ctx := context.Background()
	s, ipamRepo := newRangesTestService(t, failingCreateRepo{memory.NewRepository()})
	if _, err := s.CreateNetwork(ctx, &network.NetworkCreateRequest{Name: "a", CIDR: "10.12.0.0/24", CIDRv6: "fd12::/64"}); err == nil {
		t.Fatal("network created")
	}
	// IPAM would refuse an overlapping range if the attempt had kept its ranges.
	for _, cidr := range []string{"10.12.0.0/16", "fd12::/48"} {
		if _, err := ipamRepo.EnsureRootPrefix(ctx, cidr); err != nil {
			t.Errorf("%s: %v", cidr, err)
		}
	}
}

func TestDeleteNetworkFreesItsRangeWithItsPeers(t *testing.T) {
	ctx := context.Background()
	s, _ := newRangesTestService(t, nil)
	old := mustCreateNetwork(t, s, "old", "10.20.0.0/24", "")
	mustAddPeer(t, s, old.ID, "jump")
	mustAddPeer(t, s, old.ID, "laptop")
	if err := s.DeleteNetwork(ctx, old.ID); err != nil {
		t.Fatal(err)
	}

	// The range and its addresses are free again: a wider network over it
	// starts from the first address.
	n := mustCreateNetwork(t, s, "new", "10.20.0.0/16", "")
	if p := mustAddPeer(t, s, n.ID, "jump"); p.Address != "10.20.0.1" {
		t.Errorf("address %s, want 10.20.0.1", p.Address)
	}
}

func TestAddPeerToFullNetwork(t *testing.T) {
	ctx := context.Background()
	s, ipamRepo := newRangesTestService(t, nil)
	n := mustCreateNetwork(t, s, "tiny", "10.30.0.0/30", "")
	mustAddPeer(t, s, n.ID, "jump")
	laptop := mustAddPeer(t, s, n.ID, "laptop")

	_, err := s.AddPeer(ctx, n.ID, &network.PeerCreateRequest{Name: "phone", UseAgent: true}, "")
	if !errors.Is(err, network.ErrNetworkFull) || !strings.Contains(err.Error(), "10.30.0.0/30") {
		t.Fatalf("err = %v, want ErrNetworkFull naming the range", err)
	}
	if err := s.DeletePeer(ctx, n.ID, laptop.ID); err != nil {
		t.Fatal(err)
	}
	mustAddPeer(t, s, n.ID, "phone")

	// Dual stack: when IPv6 runs out, the IPv4 address taken first is given back.
	d := mustCreateNetwork(t, s, "dual", "10.31.0.0/24", "fd31::/126") // 3 IPv6 addresses
	for _, name := range []string{"a", "b", "c"} {
		mustAddPeer(t, s, d.ID, name)
	}
	if _, err := s.AddPeer(ctx, d.ID, &network.PeerCreateRequest{Name: "d", UseAgent: true}, ""); !errors.Is(err, network.ErrNetworkFull) {
		t.Fatalf("err = %v, want ErrNetworkFull", err)
	}
	if ip, _ := ipamRepo.AcquireIP(ctx, "10.31.0.0/24"); ip != "10.31.0.4" {
		t.Errorf("next IPv4 address %s, want 10.31.0.4 (released)", ip)
	}
}

func TestUpdateNetworkGrowsRangeKeepingAddresses(t *testing.T) {
	ctx := context.Background()
	s, _ := newRangesTestService(t, nil)
	n := mustCreateNetwork(t, s, "grow", "10.40.0.0/30", "")
	mustAddPeer(t, s, n.ID, "jump")
	mustAddPeer(t, s, n.ID, "laptop")
	before := addresses(t, s, n.ID, false)

	updated, err := s.UpdateNetwork(ctx, n.ID, &network.NetworkUpdateRequest{CIDR: "10.40.0.0/29"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.CIDR != "10.40.0.0/29" {
		t.Errorf("CIDR %s", updated.CIDR)
	}
	if after := addresses(t, s, n.ID, false); !maps.Equal(before, after) {
		t.Errorf("addresses changed: %v → %v", before, after)
	}
	if p := mustAddPeer(t, s, n.ID, "phone"); p.Address != "10.40.0.3" {
		t.Errorf("new peer got %s, want 10.40.0.3", p.Address)
	}
}

func TestUpdateNetworkShiftsRangeRenumberingPeers(t *testing.T) {
	ctx := context.Background()
	s, _ := newRangesTestService(t, nil)
	n := mustCreateNetwork(t, s, "shift", "10.41.0.0/24", "")
	mustAddPeer(t, s, n.ID, "jump")
	mustAddPeer(t, s, n.ID, "laptop")

	if _, err := s.UpdateNetwork(ctx, n.ID, &network.NetworkUpdateRequest{CIDR: "10.42.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	got := slices.Sorted(maps.Values(addresses(t, s, n.ID, false)))
	if !slices.Equal(got, []string{"10.42.0.1", "10.42.0.2"}) {
		t.Errorf("addresses %v", got)
	}
	// The old range is free.
	mustCreateNetwork(t, s, "other", "10.41.0.0/16", "")
}

func TestUpdateNetworkShrinkKeepsAddressesThatStillFit(t *testing.T) {
	ctx := context.Background()
	s, _ := newRangesTestService(t, nil)
	n := mustCreateNetwork(t, s, "shrink", "10.43.0.0/28", "")
	var peers []*network.Peer
	for _, name := range []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8"} {
		peers = append(peers, mustAddPeer(t, s, n.ID, name))
	}
	for _, p := range peers[1:6] {
		if err := s.DeletePeer(ctx, n.ID, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	// p1 .1 fits the /29; p7 .7 is its broadcast address and p8 .8 is outside.
	if _, err := s.UpdateNetwork(ctx, n.ID, &network.NetworkUpdateRequest{CIDR: "10.43.0.0/29"}); err != nil {
		t.Fatal(err)
	}
	got := addresses(t, s, n.ID, false)
	if got["p1"] != "10.43.0.1" {
		t.Errorf("p1 = %s, want its address kept", got["p1"])
	}
	moved := []string{got["p7"], got["p8"]}
	slices.Sort(moved)
	if !slices.Equal(moved, []string{"10.43.0.2", "10.43.0.3"}) {
		t.Errorf("p7, p8 = %v, want .2 and .3", moved)
	}
}

func TestUpdateNetworkIPv6(t *testing.T) {
	ctx := context.Background()
	s, _ := newRangesTestService(t, nil)
	n := mustCreateNetwork(t, s, "v6", "10.44.0.0/24", "")
	mustAddPeer(t, s, n.ID, "jump")
	mustAddPeer(t, s, n.ID, "laptop")

	// Adding IPv6 gives every peer an address.
	if _, err := s.UpdateNetwork(ctx, n.ID, &network.NetworkUpdateRequest{CIDRv6: "fd44::/64"}); err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Values(addresses(t, s, n.ID, true))); !slices.Equal(got, []string{"fd44::1", "fd44::2"}) {
		t.Errorf("IPv6 addresses %v", got)
	}
	// Widening it keeps them.
	if _, err := s.UpdateNetwork(ctx, n.ID, &network.NetworkUpdateRequest{CIDRv6: "fd44::/48"}); err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Values(addresses(t, s, n.ID, true))); !slices.Equal(got, []string{"fd44::1", "fd44::2"}) {
		t.Errorf("IPv6 addresses after widening %v", got)
	}
}

func TestUpdateNetworkRefusalsChangeNothing(t *testing.T) {
	ctx := context.Background()
	s, _ := newRangesTestService(t, nil)
	mustCreateNetwork(t, s, "prod", "10.50.0.0/24", "fd50::/64")
	n := mustCreateNetwork(t, s, "dev", "10.51.0.0/29", "")
	for _, name := range []string{"jump", "a", "b", "c"} {
		mustAddPeer(t, s, n.ID, name)
	}
	before := addresses(t, s, n.ID, false)

	for _, tt := range []struct {
		req  network.NetworkUpdateRequest
		want error
	}{
		{network.NetworkUpdateRequest{Name: "renamed", CIDR: "10.50.0.0/16"}, network.ErrCIDROverlap},
		{network.NetworkUpdateRequest{CIDRv6: "fd50::/48"}, network.ErrCIDROverlap},
		{network.NetworkUpdateRequest{CIDR: "10.52.0.0/30"}, network.ErrInvalidCIDR}, // room for 2, 4 peers
		{network.NetworkUpdateRequest{CIDR: "10.52.0.1/24"}, network.ErrInvalidCIDR},
		{network.NetworkUpdateRequest{CIDR: "fd52::/64"}, network.ErrInvalidCIDR},
	} {
		if _, err := s.UpdateNetwork(ctx, n.ID, &tt.req); !errors.Is(err, tt.want) {
			t.Errorf("%+v: err = %v, want %v", tt.req, err, tt.want)
		}
	}
	got, err := s.GetNetwork(ctx, n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "dev" || got.CIDR != "10.51.0.0/29" || got.CIDRv6 != "" {
		t.Errorf("network changed: %s %s %s", got.Name, got.CIDR, got.CIDRv6)
	}
	if after := addresses(t, s, n.ID, false); !maps.Equal(before, after) {
		t.Errorf("addresses changed: %v → %v", before, after)
	}
	// Its range still works.
	if p := mustAddPeer(t, s, n.ID, "d"); p.Address != "10.51.0.5" {
		t.Errorf("new peer got %s, want 10.51.0.5", p.Address)
	}

	// A static peer would keep a config with its old address.
	if _, err := s.AddPeer(ctx, n.ID, &network.PeerCreateRequest{Name: "static"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateNetwork(ctx, n.ID, &network.NetworkUpdateRequest{CIDR: "10.51.0.0/24"}); err == nil || !strings.Contains(err.Error(), "static regular peer 'static'") {
		t.Errorf("err = %v, want the static peer refusal", err)
	}
}
