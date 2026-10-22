package network

import (
	"context"
	"testing"
	"time"

	"wirety/internal/adapters/db/memory"
	"wirety/internal/domain/network"
)

// newEventsTestService returns a service over in-memory repositories with a
// network holding a jump peer and peer-a (10.0.0.2), and the access history
// enabled.
func newEventsTestService(t *testing.T) (*Service, *memory.Repository) {
	t.Helper()
	ctx := context.Background()
	repo := memory.NewRepository()
	if err := repo.CreateNetwork(ctx, &network.Network{ID: "net", Name: "net", CIDR: "10.0.0.0/24"}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []*network.Peer{
		{ID: "jump", Name: "jump", Address: "10.0.0.1", IsJump: true},
		{ID: "peer-a", Name: "peer-a", Address: "10.0.0.2"},
	} {
		if err := repo.CreatePeer(ctx, "net", p); err != nil {
			t.Fatal(err)
		}
	}
	s := NewService(repo, memory.NewIPAMRepository(ctx), nil, nil, nil, nil, nil)
	s.SetCaptivePortalEventRepository(repo)
	return s, repo
}

func events(t *testing.T, s *Service) []*network.CaptivePortalEvent {
	t.Helper()
	evs, err := s.ListCaptivePortalEvents(context.Background(), "net", "peer-a", 10)
	if err != nil {
		t.Fatal(err)
	}
	return evs
}

func TestDisconnectedPeerSignOutIsRecorded(t *testing.T) {
	ctx := context.Background()
	s, repo := newEventsTestService(t)
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump", "10.0.0.2", "1.2.3.4:51000")

	lastHandshake := map[string]time.Time{"10.0.0.2": time.Now().Add(-5 * time.Minute)}
	if err := s.CleanupWhitelistForDisconnectedPeers(ctx, "net", "jump", map[string]bool{}, lastHandshake); err != nil {
		t.Fatal(err)
	}

	wl, _ := repo.GetCaptivePortalWhitelist(ctx, "net", "jump")
	if len(wl) != 0 {
		t.Fatalf("disconnected peer still whitelisted: %v", wl)
	}
	evs := events(t, s)
	if len(evs) != 1 || evs[0].Event != network.CaptivePortalEventTunnelInactive {
		t.Fatalf("want one tunnel_inactive event, got %+v", evs)
	}
	if evs[0].Detail != "last WireGuard handshake 5m0s ago (idle limit 3m0s)" {
		t.Errorf("detail = %q", evs[0].Detail)
	}

	// Also whitelisted on a second jump peer: the same sign-out is not
	// recorded twice.
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump-2", "10.0.0.2", "1.2.3.4:51000")
	_ = s.CleanupWhitelistForDisconnectedPeers(ctx, "net", "jump-2", map[string]bool{}, nil)
	if n := len(events(t, s)); n != 1 {
		t.Errorf("sign-out recorded %d times", n)
	}
}

func TestLivePeerKeepsItsAccess(t *testing.T) {
	ctx := context.Background()
	s, repo := newEventsTestService(t)
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump", "10.0.0.2", "1.2.3.4:51000")

	_ = s.CleanupWhitelistForDisconnectedPeers(ctx, "net", "jump", map[string]bool{"10.0.0.2": true}, nil)

	if wl, _ := repo.GetCaptivePortalWhitelist(ctx, "net", "jump"); len(wl) != 1 {
		t.Fatalf("live peer lost its access: %v", wl)
	}
	if evs := events(t, s); len(evs) != 0 {
		t.Errorf("unexpected events: %+v", evs)
	}
}

func TestAsleepPeerKeepsItsSessionUpToTheIdleTimeout(t *testing.T) {
	ctx := context.Background()
	s, repo := newEventsTestService(t)
	s.SetCaptivePortalIdleTimeout(time.Hour)
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump", "10.0.0.2", "1.2.3.4:51000")

	// Laptop asleep for 20 min: no handshake, the session is kept.
	asleep := map[string]time.Time{"10.0.0.2": time.Now().Add(-20 * time.Minute)}
	_ = s.CleanupWhitelistForDisconnectedPeers(ctx, "net", "jump", map[string]bool{}, asleep)
	if wl, _ := repo.GetCaptivePortalWhitelist(ctx, "net", "jump"); len(wl) != 1 {
		t.Fatalf("asleep peer lost its session within the idle timeout: %v", wl)
	}
	if evs := events(t, s); len(evs) != 0 {
		t.Fatalf("unexpected events: %+v", evs)
	}

	// Past the idle timeout: the session ends.
	gone := map[string]time.Time{"10.0.0.2": time.Now().Add(-61 * time.Minute)}
	_ = s.CleanupWhitelistForDisconnectedPeers(ctx, "net", "jump", map[string]bool{}, gone)
	if wl, _ := repo.GetCaptivePortalWhitelist(ctx, "net", "jump"); len(wl) != 0 {
		t.Fatalf("peer still whitelisted past the idle timeout: %v", wl)
	}
	if evs := events(t, s); len(evs) != 1 || evs[0].Detail != "last WireGuard handshake 1h1m0s ago (idle limit 1h0m0s)" {
		t.Fatalf("want one tunnel_inactive with the idle limit, got %+v", evs)
	}
}

func TestPeerWithoutKnownHandshakeLosesItsSession(t *testing.T) {
	ctx := context.Background()
	s, repo := newEventsTestService(t)
	s.SetCaptivePortalIdleTimeout(time.Hour)
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump", "10.0.0.2", "1.2.3.4:51000")

	// The jump knows no handshake (its interface was recreated): nothing
	// proves the peer was recently there, the session ends.
	_ = s.CleanupWhitelistForDisconnectedPeers(ctx, "net", "jump", map[string]bool{}, nil)
	if wl, _ := repo.GetCaptivePortalWhitelist(ctx, "net", "jump"); len(wl) != 0 {
		t.Fatalf("session kept without a known handshake: %v", wl)
	}
}

func TestEndpointChangeIsRecordedOncePerNewIP(t *testing.T) {
	ctx := context.Background()
	s, repo := newEventsTestService(t)
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump", "10.0.0.2", "1.2.3.4:51000")
	peers, _ := repo.ListPeers(ctx, "net")

	// Same public IP, NAT port rebind: not a change.
	s.recordEndpointChanges(ctx, "net", "jump", peers, map[string]string{"10.0.0.2": "1.2.3.4:62000"})
	if evs := events(t, s); len(evs) != 0 {
		t.Fatalf("port rebind recorded: %+v", evs)
	}

	// New public IP, seen on every heartbeat: recorded once.
	for i := 0; i < 3; i++ {
		s.recordEndpointChanges(ctx, "net", "jump", peers, map[string]string{"10.0.0.2": "5.6.7.8:40000"})
	}
	evs := events(t, s)
	if len(evs) != 1 || evs[0].Event != network.CaptivePortalEventEndpointChanged || evs[0].Detail != "public IP 1.2.3.4 → 5.6.7.8" {
		t.Fatalf("want one endpoint_changed 1.2.3.4 → 5.6.7.8, got %+v", evs)
	}

	// Another network again: a new event.
	s.recordEndpointChanges(ctx, "net", "jump", peers, map[string]string{"10.0.0.2": "9.9.9.9:40000"})
	if evs := events(t, s); len(evs) != 2 || evs[0].Detail != "public IP 1.2.3.4 → 9.9.9.9" {
		t.Fatalf("second change not recorded: %+v", evs)
	}

	// The whitelist entry is kept (the agent enforces the endpoint check).
	if wl, _ := repo.GetCaptivePortalWhitelist(ctx, "net", "jump"); len(wl) != 1 {
		t.Errorf("whitelist entry removed: %v", wl)
	}
}

func TestConcurrentUseBehindTheSameIPEndsTheSession(t *testing.T) {
	ctx := context.Background()
	s, repo := newEventsTestService(t)
	_ = repo.AddCaptivePortalWhitelist(ctx, "net", "jump", "10.0.0.2", "1.2.3.4:51000")

	err := s.processEndpointTakeovers(ctx, "net", "jump", []network.EndpointTakeoverReport{{
		WgIP: "10.0.0.2", AuthenticatedAt: "1.2.3.4:51000", EndSession: true,
		Endpoints: []string{"1.2.3.4:52000", "1.2.3.4:53000"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if wl, _ := repo.GetCaptivePortalWhitelist(ctx, "net", "jump"); len(wl) != 0 {
		t.Fatalf("session kept: %v", wl)
	}
	evs := events(t, s)
	if len(evs) != 1 || evs[0].Event != network.CaptivePortalEventConcurrentUse || evs[0].Detail != "two devices at 1.2.3.4:52000 and 1.2.3.4:53000" {
		t.Fatalf("want one concurrent_use event, got %+v", evs)
	}
	// Nothing is denylisted: the legitimate device cannot be told apart.
	if dl, _ := repo.GetEndpointDenylist(ctx, "net", "jump"); len(dl) != 0 {
		t.Errorf("denylisted: %+v", dl)
	}
}
