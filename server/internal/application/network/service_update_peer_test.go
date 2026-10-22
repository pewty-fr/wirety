package network

import (
	"context"
	"errors"
	"strings"
	"testing"

	"wirety/internal/domain/network"
)

func TestUpdatePeerClearsOrKeepsOwnerAndEndpoint(t *testing.T) {
	ctx := context.Background()
	s, repo := newEventsTestService(t)
	if err := repo.UpdatePeer(ctx, "net", &network.Peer{ID: "peer-a", Name: "peer-a", Address: "10.0.0.2", OwnerID: "bob", Endpoint: "203.0.113.7:51820"}); err != nil {
		t.Fatal(err)
	}

	p, err := s.UpdatePeer(ctx, "net", "peer-a", &network.PeerUpdateRequest{Name: "peer-b"})
	if err != nil {
		t.Fatal(err)
	}
	if p.OwnerID != "bob" || p.Endpoint != "203.0.113.7:51820" {
		t.Errorf("got owner %q, endpoint %q; want them unchanged", p.OwnerID, p.Endpoint)
	}

	p, err = s.UpdatePeer(ctx, "net", "peer-a", &network.PeerUpdateRequest{OwnerID: new(""), Endpoint: new("")})
	if err != nil {
		t.Fatal(err)
	}
	if p.OwnerID != "" || p.Endpoint != "" {
		t.Errorf("got owner %q, endpoint %q; want both cleared", p.OwnerID, p.Endpoint)
	}
}

func TestPeerAdditionalAllowedIPsAreChecked(t *testing.T) {
	ctx := context.Background()
	s, _ := newEventsTestService(t)

	_, err := s.AddPeer(ctx, "net", &network.PeerCreateRequest{Name: "laptop", AdditionalAllowedIPs: []string{"192.168.1.10/24"}}, "")
	if !errors.Is(err, network.ErrInvalidCIDR) || !strings.Contains(err.Error(), "additional_allowed_ips") {
		t.Fatalf("create with a host-bits CIDR: got %v", err)
	}

	_, err = s.UpdatePeer(ctx, "net", "peer-a", &network.PeerUpdateRequest{AdditionalAllowedIPs: []string{"not-an-ip"}})
	if !errors.Is(err, network.ErrInvalidCIDR) {
		t.Fatalf("update with an invalid entry: got %v", err)
	}
	p, err := s.UpdatePeer(ctx, "net", "peer-a", &network.PeerUpdateRequest{AdditionalAllowedIPs: []string{"192.168.1.10", "192.168.2.0/24"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.AdditionalAllowedIPs) != 2 {
		t.Errorf("allowed IPs = %v", p.AdditionalAllowedIPs)
	}
}
