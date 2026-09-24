//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

// TestStackSmoke validates the non-WireGuard spine of the harness: the server
// and Dex images build, Postgres comes up, the server connects to the DB and
// validates Dex's issuer, the admin Bearer is obtained via the OIDC password
// grant (first user => administrator), and the REST API accepts an
// authenticated write. It creates no agent/peer containers, so it runs anywhere
// Docker runs — including hosts without the WireGuard kernel module.
//
// Run: go test -tags e2e -run TestStackSmoke -v ./...
func TestStackSmoke(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	st := setupStack(ctx, t)

	net, err := st.admin.createNetwork(ctx, network{
		Name:         "smoke",
		CIDR:         "10.91.0.0/24",
		DomainSuffix: "smoke.internal",
	})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	if net.ID == "" {
		t.Fatal("created network has no ID")
	}

	// A jump peer create returns an enrollment token — proves peer + IPAM work.
	jump, err := st.admin.createPeer(ctx, net.ID, createPeerReq{
		Name: "jump-1", IsJump: true, UseAgent: true, Endpoint: "10.255.255.254", ListenPort: 51820,
	})
	if err != nil {
		t.Fatalf("create jump peer: %v", err)
	}
	if jump.Token == "" {
		t.Fatal("jump peer create returned no enrollment token")
	}
	if jump.Address == "" {
		t.Fatal("jump peer has no allocated address")
	}
	t.Logf("smoke OK: network=%s jump=%s addr=%s", net.ID, jump.ID, jump.Address)
}
