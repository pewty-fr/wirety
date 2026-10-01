package network

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"wirety/internal/domain/network"

	"github.com/rs/zerolog/log"
)

// captivePortalEventRetention is how long the captive-portal access history
// is kept.
const captivePortalEventRetention = 30 * 24 * time.Hour

// SetCaptivePortalEventRepository enables the captive-portal access history.
// Without it (e.g. in-memory tests) nothing is recorded.
func (s *Service) SetCaptivePortalEventRepository(events network.CaptivePortalEventRepository) {
	s.events = events
}

// ListCaptivePortalEvents returns a peer's captive-portal access history,
// newest first.
func (s *Service) ListCaptivePortalEvents(ctx context.Context, networkID, peerID string, limit int) ([]*network.CaptivePortalEvent, error) {
	if s.events == nil {
		return []*network.CaptivePortalEvent{}, nil
	}
	events, err := s.events.ListCaptivePortalEvents(ctx, networkID, peerID, limit)
	if err != nil {
		return nil, err
	}
	if events == nil {
		events = []*network.CaptivePortalEvent{}
	}
	return events, nil
}

// lastCaptivePortalSignOut returns the event that ended the peer's last
// captive-portal access, or nil when its latest event is a sign-in.
func (s *Service) lastCaptivePortalSignOut(ctx context.Context, networkID, peerID string) *network.CaptivePortalEvent {
	if latest := s.latestCaptivePortalEvent(ctx, networkID, peerID); latest != nil && latest.EndsAccess() {
		return latest
	}
	return nil
}

// lastSignOutForPreview is lastCaptivePortalSignOut for the captive-portal
// page. A session that just expired may not be recorded yet (the cleanup loop
// runs every 2 min), so expired sessions are processed first.
func (s *Service) lastSignOutForPreview(ctx context.Context, networkID, peerID string) *network.CaptivePortalEvent {
	if err := s.CleanupExpiredCaptivePortalWhitelist(ctx); err != nil {
		log.Warn().Err(err).Msg("captive portal: expired whitelist cleanup failed")
	}
	return s.lastCaptivePortalSignOut(ctx, networkID, peerID)
}

func (s *Service) latestCaptivePortalEvent(ctx context.Context, networkID, peerID string) *network.CaptivePortalEvent {
	if s.events == nil {
		return nil
	}
	events, err := s.events.ListCaptivePortalEvents(ctx, networkID, peerID, 1)
	if err != nil || len(events) == 0 {
		return nil
	}
	return events[0]
}

// recordCaptivePortalEvent stores e, best effort. An end of access that is
// already the peer's latest event is not recorded again: detection repeats on
// every jump heartbeat, and a peer may be whitelisted on several jump peers.
// A public-IP change is a new event when it is to another IP.
func (s *Service) recordCaptivePortalEvent(ctx context.Context, e *network.CaptivePortalEvent) {
	if s.events == nil {
		return
	}
	if e.EndsAccess() {
		if latest := s.latestCaptivePortalEvent(ctx, e.NetworkID, e.PeerID); latest != nil && latest.Event == e.Event &&
			(e.Event != network.CaptivePortalEventEndpointChanged || latest.Detail == e.Detail) {
			return
		}
	}
	if err := s.events.AddCaptivePortalEvent(ctx, e); err != nil {
		log.Warn().Err(err).Str("peer_id", e.PeerID).Str("event", e.Event).Msg("failed to record captive portal event")
		return
	}
	log.Info().
		Str("network_id", e.NetworkID).
		Str("peer_id", e.PeerID).
		Str("peer_ip", e.PeerIP).
		Str("event", e.Event).
		Str("detail", e.Detail).
		Msg("captive portal access event")
}

// recordEndpointChanges records the whitelisted peers whose live public IP
// differs from the one they signed in from: the jump peer stops letting them
// through (see the agent's endpoint check) until they sign in again from the
// new network. The whitelist entry itself is left alone — when two devices
// share one WireGuard config the endpoint oscillates, and the agent's takeover
// defense handles that without signing the legitimate user out.
func (s *Service) recordEndpointChanges(ctx context.Context, networkID, jumpPeerID string, peers []*network.Peer, liveEndpoints map[string]string) {
	if s.events == nil || len(liveEndpoints) == 0 {
		return
	}
	whitelist, err := s.repo.GetCaptivePortalWhitelist(ctx, networkID, jumpPeerID)
	if err != nil {
		return
	}
	for _, entry := range whitelist {
		wgIP, authEndpoint, ok := strings.Cut(entry, "@")
		if !ok || authEndpoint == "" {
			continue
		}
		current, live := liveEndpoints[wgIP]
		if !live || endpointHost(current) == endpointHost(authEndpoint) {
			continue
		}
		p := findPeerByVPNIP(peers, wgIP)
		if p == nil {
			continue
		}
		s.recordCaptivePortalEvent(ctx, &network.CaptivePortalEvent{
			NetworkID: networkID, PeerID: p.ID, PeerIP: wgIP,
			Event:  network.CaptivePortalEventEndpointChanged,
			Detail: fmt.Sprintf("public IP %s → %s", endpointHost(authEndpoint), endpointHost(current)),
		})
	}
}

// CleanupExpiredCaptivePortalWhitelist removes the captive-portal sessions
// whose duration was reached, records their expiry, and prunes the access
// history past its retention.
func (s *Service) CleanupExpiredCaptivePortalWhitelist(ctx context.Context) error {
	if s.events == nil {
		return s.repo.CleanupExpiredCaptivePortalWhitelist(ctx)
	}
	expired, err := s.events.DeleteExpiredCaptivePortalWhitelist(ctx)
	if err != nil {
		return err
	}
	peersByNetwork := make(map[string][]*network.Peer)
	for _, e := range expired {
		peers, ok := peersByNetwork[e.NetworkID]
		if !ok {
			peers, _ = s.repo.ListPeers(ctx, e.NetworkID)
			peersByNetwork[e.NetworkID] = peers
		}
		if p := findPeerByVPNIP(peers, e.PeerIP); p != nil {
			s.recordCaptivePortalEvent(ctx, &network.CaptivePortalEvent{
				NetworkID: e.NetworkID, PeerID: p.ID, PeerIP: e.PeerIP,
				Event: network.CaptivePortalEventExpired, Detail: "session duration reached",
				CreatedAt: e.ExpiresAt,
			})
		}
	}
	return s.events.DeleteCaptivePortalEventsBefore(ctx, time.Now().Add(-captivePortalEventRetention))
}

// signInDetail describes where a sign-in came from, when the jump knew the
// peer's public endpoint.
func signInDetail(endpoint string) string {
	if host := endpointHost(endpoint); host != "" {
		return "from public IP " + host
	}
	return ""
}

// endpointHost returns the IP of an "ip:port" / "[ipv6]:port" endpoint.
func endpointHost(endpoint string) string {
	if host, _, err := net.SplitHostPort(endpoint); err == nil {
		return host
	}
	return endpoint
}

// vpnIP strips the prefix length from a peer address ("10.0.0.2/32").
func vpnIP(addr string) string {
	if ip, _, ok := strings.Cut(addr, "/"); ok {
		return ip
	}
	return addr
}
