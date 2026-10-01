package network

import (
	"context"
	"time"
)

// Captive-portal access events. Every sign-in and every end of access is
// recorded with its cause, so it is visible why a peer had to go back
// through the captive portal.
const (
	// CaptivePortalEventAuthenticated: the peer signed in through the portal.
	CaptivePortalEventAuthenticated = "authenticated"
	// CaptivePortalEventExpired: the session duration (CAPTIVE_PORTAL_SESSION_TTL) was reached.
	CaptivePortalEventExpired = "expired"
	// CaptivePortalEventTunnelInactive: no WireGuard handshake for too long (disconnected).
	CaptivePortalEventTunnelInactive = "tunnel_inactive"
	// CaptivePortalEventEndpointChanged: the peer's public IP differs from the
	// one it signed in from (network change); the jump no longer lets it through.
	CaptivePortalEventEndpointChanged = "endpoint_changed"
	// CaptivePortalEventRevoked: authentication revoked from the dashboard.
	CaptivePortalEventRevoked = "revoked"
)

// CaptivePortalEvent is one entry of a peer's captive-portal access history.
type CaptivePortalEvent struct {
	NetworkID string    `json:"network_id"`
	PeerID    string    `json:"peer_id"`
	PeerIP    string    `json:"peer_ip"`
	Event     string    `json:"event"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// EndsAccess reports whether the event ended the peer's access.
func (e *CaptivePortalEvent) EndsAccess() bool {
	return e.Event != CaptivePortalEventAuthenticated
}

// ExpiredWhitelistEntry is a captive-portal whitelist entry removed because
// its session duration was reached.
type ExpiredWhitelistEntry struct {
	NetworkID string
	PeerIP    string
	ExpiresAt time.Time
}

// CaptivePortalEventRepository stores the captive-portal access history.
type CaptivePortalEventRepository interface {
	AddCaptivePortalEvent(ctx context.Context, e *CaptivePortalEvent) error
	// ListCaptivePortalEvents returns a peer's events, newest first.
	ListCaptivePortalEvents(ctx context.Context, networkID, peerID string, limit int) ([]*CaptivePortalEvent, error)
	DeleteCaptivePortalEventsBefore(ctx context.Context, before time.Time) error
	// DeleteExpiredCaptivePortalWhitelist removes the expired whitelist
	// entries and returns them, so their expiry can be recorded.
	DeleteExpiredCaptivePortalWhitelist(ctx context.Context) ([]ExpiredWhitelistEntry, error)
}
