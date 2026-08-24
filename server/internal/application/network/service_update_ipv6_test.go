package network

import (
	"context"
	"testing"

	network "wirety/internal/domain/network"
)

// TestUpdateNetwork_AddIPv6CIDR locks in the fix for the bug where a PUT adding
// an IPv6 CIDR to an existing IPv4-only network was silently dropped: the
// UpdateNetwork service never read req.CIDRv6. The correct behaviour is to
// persist the new v6 CIDR AND make the network dual-stack by allocating an IPv6
// address to every existing peer (registering the IPAM root prefix first).
func TestUpdateNetwork_AddIPv6CIDR(t *testing.T) {
	ctx := context.Background()

	const networkID = "net-1"
	fullRepo := newMockFullRepository()
	fullRepo.networks[networkID] = &network.Network{
		ID:   networkID,
		Name: "dev",
		CIDR: "10.255.0.0/22", // IPv4-only to start
	}
	// Two existing peers with no IPv6 yet.
	fullRepo.peers["p1"] = &network.Peer{ID: "p1", Address: "10.255.0.1/22"}
	fullRepo.peers["p2"] = &network.Peer{ID: "p2", Address: "10.255.0.2/22"}

	svc := &Service{repo: fullRepo}

	net, err := svc.UpdateNetwork(ctx, networkID, &network.NetworkUpdateRequest{
		CIDRv6: "fd00::/64",
	})
	if err != nil {
		t.Fatalf("UpdateNetwork returned error: %v", err)
	}

	if net.CIDRv6 != "fd00::/64" {
		t.Errorf("expected CIDRv6 to be persisted as fd00::/64, got %q", net.CIDRv6)
	}
	if net.CIDR != "10.255.0.0/22" {
		t.Errorf("IPv4 CIDR must be untouched, got %q", net.CIDR)
	}

	// Every existing peer must have received an IPv6 address (mutated in place
	// on the objects ListPeers returns).
	for _, id := range []string{"p1", "p2"} {
		if got := fullRepo.peers[id].AddressV6; got == "" {
			t.Errorf("peer %s did not receive an IPv6 address", id)
		}
		if got := fullRepo.peers[id].Address; got == "" {
			t.Errorf("peer %s lost its IPv4 address", id)
		}
	}
}

// TestUpdateNetwork_EmptyIPv6IsNoChange verifies the "empty means leave
// unchanged" semantics: a PUT that omits cidr_v6 must never wipe an existing
// one (mirrors the IPv4 CIDR handling).
func TestUpdateNetwork_EmptyIPv6IsNoChange(t *testing.T) {
	ctx := context.Background()

	const networkID = "net-2"
	fullRepo := newMockFullRepository()
	fullRepo.networks[networkID] = &network.Network{
		ID:     networkID,
		Name:   "dev",
		CIDR:   "10.255.0.0/22",
		CIDRv6: "fd00::/64", // already dual-stack
	}

	svc := &Service{repo: fullRepo}

	net, err := svc.UpdateNetwork(ctx, networkID, &network.NetworkUpdateRequest{
		Name: "dev-renamed", // unrelated change, no cidr_v6
	})
	if err != nil {
		t.Fatalf("UpdateNetwork returned error: %v", err)
	}
	if net.CIDRv6 != "fd00::/64" {
		t.Errorf("existing CIDRv6 must be preserved when req omits it, got %q", net.CIDRv6)
	}
}
