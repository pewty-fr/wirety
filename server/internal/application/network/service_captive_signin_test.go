package network

import (
	"context"
	"testing"
	"time"

	"wirety/internal/adapters/db/memory"
	"wirety/internal/domain/network"
)

// jumpSees sends the jump peer's heartbeat: peer-a (10.0.0.2) is live at the
// given public endpoint.
func jumpSees(t *testing.T, s *Service, endpoint string) {
	t.Helper()
	err := s.ProcessAgentHeartbeat(context.Background(), "net", "jump", &network.AgentHeartbeat{
		PeerEndpoints:  map[string]string{"peer-a-key": endpoint},
		PeerHandshakes: map[string]int64{"peer-a-key": time.Now().Unix()},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func newSignInTestService(t *testing.T) (*Service, *memory.Repository) {
	t.Helper()
	s, repo := newEventsTestService(t)
	if err := repo.UpdatePeer(context.Background(), "net", &network.Peer{ID: "peer-a", Name: "peer-a", Address: "10.0.0.2", PublicKey: "peer-a-key"}); err != nil {
		t.Fatal(err)
	}
	return s, repo
}

func portalState(t *testing.T, s *Service) string {
	t.Helper()
	status, err := s.GetPeerConnectivityStatus(context.Background(), "net", "peer-a")
	if err != nil {
		t.Fatal(err)
	}
	return status.CaptivePortalState
}

// The dashboard shows a peer as authenticated only while the jump peer lets it
// through: from the public IP it signed in from.
func TestPortalStateFollowsTheJumpPeersEndpointCheck(t *testing.T) {
	ctx := context.Background()
	s, repo := newSignInTestService(t)
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump", "10.0.0.2", "1.2.3.4:51000")

	if got := portalState(t, s); got != "authenticated" {
		t.Errorf("no heartbeat yet: state %q, want authenticated", got)
	}
	jumpSees(t, s, "1.2.3.4:62000") // NAT port rebind, same IP
	if got := portalState(t, s); got != "authenticated" {
		t.Errorf("same public IP: state %q, want authenticated", got)
	}

	jumpSees(t, s, "5.6.7.8:40000") // another network
	if got := portalState(t, s); got != "" {
		t.Errorf("another public IP: state %q, want not authenticated", got)
	}
	if _, err := s.CreateCaptivePortalToken(ctx, "net", "jump", "10.0.0.2", "5.6.7.8:40000"); err != nil {
		t.Fatal(err)
	}
	if got := portalState(t, s); got != "pending_auth" {
		t.Errorf("signing in from the new network: state %q, want pending_auth", got)
	}

	// Signing in again rebinds the entry to the new network.
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump", "10.0.0.2", "5.6.7.8:40000")
	if got := portalState(t, s); got != "authenticated" {
		t.Errorf("after signing in again: state %q, want authenticated", got)
	}
}

// A whitelisted peer the jump peer does not let through (its entry is bound to
// another network's public IP) must be able to sign in again: the jump peer
// gets its token as pending, so it keeps it for the browser and opens the
// HTTPS access the sign-in needs.
func TestTokenOfAWhitelistedPeerIsPending(t *testing.T) {
	ctx := context.Background()
	s, repo := newSignInTestService(t)
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump", "10.0.0.2", "1.2.3.4:51000")
	jumpSees(t, s, "5.6.7.8:40000")

	token, err := s.CreateCaptivePortalToken(ctx, "net", "jump", "10.0.0.2", "5.6.7.8:40000")
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.GetCaptivePortalSecurityState(ctx, "net", "jump")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.PendingAuth) != 1 || state.PendingAuth[0].WgIP != "10.0.0.2" || state.PendingAuth[0].Endpoint != "5.6.7.8:40000" {
		t.Fatalf("pending auth = %+v, want 10.0.0.2 from 5.6.7.8:40000", state.PendingAuth)
	}
	if len(state.Whitelist) != 1 {
		t.Errorf("whitelist = %v, want the entry kept", state.Whitelist)
	}

	// The browser's token stays valid for the /start bouncer.
	if _, err := s.BindCaptivePortalTokenToBrowser(ctx, token.Token); err != nil {
		t.Errorf("bind token: %v", err)
	}
}

// A token requested from the public IP the peer signed in from is redundant:
// the peer is authenticated, it is not pending.
func TestTokenFromTheSignedInIPIsNotPending(t *testing.T) {
	ctx := context.Background()
	s, repo := newSignInTestService(t)
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump", "10.0.0.2", "1.2.3.4:51000")
	if _, err := s.CreateCaptivePortalToken(ctx, "net", "jump", "10.0.0.2", "1.2.3.4:62000"); err != nil {
		t.Fatal(err)
	}
	state, err := s.GetCaptivePortalSecurityState(ctx, "net", "jump")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.PendingAuth) != 0 {
		t.Errorf("pending auth = %+v, want none", state.PendingAuth)
	}
}
