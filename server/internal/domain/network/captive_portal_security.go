package network

import "time"

// EndpointDenylistEntry represents a public source endpoint (IP:port) that is
// blocked at the jump peer's physical-interface level for a specific WireGuard
// peer slot.  When the agent observes the WireGuard endpoint of an authenticated
// peer change to a foreign source (config theft / config sharing), the agent
// reports that source to the server, which persists it here and pushes it to
// the jump peer's WebSocket.  The jump peer then drops UDP packets from that
// source to its WireGuard listen port — preventing the rogue device from ever
// completing a fresh WireGuard handshake and stealing the peer slot back.
//
// Entries live for DenylistTTL by default (configurable, see
// EndpointDenylistDefaultTTL).  Authenticating from a new endpoint via the
// captive portal clears any denylist entries for that peer (the user proved
// ownership through SSO, so their previous "rogue" source was actually a
// legitimate roam).
type EndpointDenylistEntry struct {
	NetworkID   string    `json:"network_id"`
	JumpPeerID  string    `json:"jump_peer_id"`
	WgIP        string    `json:"wg_ip"`        // targeted peer's WireGuard private IP
	BlockedIP   string    `json:"blocked_ip"`   // rogue source IP (may be 0.0.0.0 if port-only block)
	BlockedPort int       `json:"blocked_port"` // rogue source port (0 = any port for that IP)
	Reason      string    `json:"reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// EndpointDenylistDefaultTTL is the default lifetime of a denylist entry.
// 24 h matches the captive-portal whitelist TTL — after a full day the rogue
// source is forgotten, and any future attempts will go through the regular
// capture-portal flow again.
const EndpointDenylistDefaultTTL = 24 * time.Hour

// CaptivePortalQuarantine represents the strike state for a peer's captive
// portal authentication attempts.  After QuarantineStrikeThreshold consecutive
// failed/abandoned auth attempts (token issued but never converted into a
// successful AuthenticateCaptivePortal call before token expiry), the peer is
// quarantined: no new tokens are issued and no temporary HTTPS access is granted
// until QuarantinedUntil passes or an admin clears the strikes.
//
// Strikes reset on a successful auth.  This protects against:
//   - Brute-force attempts using a stolen WireGuard config
//   - Misbehaving clients that keep retrying without completing OIDC
type CaptivePortalQuarantine struct {
	NetworkID        string     `json:"network_id"`
	PeerID           string     `json:"peer_id"`
	Strikes          int        `json:"strikes"`
	LastStrikeAt     *time.Time `json:"last_strike_at,omitempty"`
	QuarantinedUntil *time.Time `json:"quarantined_until,omitempty"` // nil = not quarantined
}

// IsQuarantined reports whether the peer is currently in quarantine.
func (q *CaptivePortalQuarantine) IsQuarantined(now time.Time) bool {
	return q != nil && q.QuarantinedUntil != nil && now.Before(*q.QuarantinedUntil)
}

// QuarantineStrikeThreshold is the number of consecutive failed auth attempts
// after which a peer is automatically quarantined.
const QuarantineStrikeThreshold = 3

// QuarantineDuration is how long a peer remains quarantined after hitting the
// strike threshold.  1 hour gives the legitimate user a clear "wait and retry"
// path while preventing rapid brute-force.  An admin can clear this manually
// from the dashboard.
const QuarantineDuration = 1 * time.Hour
