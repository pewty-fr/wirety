package network

import "time"

// AgentSession represents an active agent session with system information
type AgentSession struct {
	PeerID           string    `json:"peer_id"`           // Peer ID this session belongs to
	Hostname         string    `json:"hostname"`          // Agent hostname
	SystemUptime     int64     `json:"system_uptime"`     // Host uptime in seconds
	WireGuardUptime  int64     `json:"wireguard_uptime"`  // WireGuard interface uptime in seconds
	ReportedEndpoint string    `json:"reported_endpoint"` // Endpoint as reported by other agents
	LastSeen         time.Time `json:"last_seen"`         // Last heartbeat timestamp
	FirstSeen        time.Time `json:"first_seen"`        // First connection timestamp
	SessionID        string    `json:"session_id"`        // Unique session identifier
}

// AgentHeartbeat represents a heartbeat message from an agent
type AgentHeartbeat struct {
	Hostname        string            `json:"hostname"`
	SystemUptime    int64             `json:"system_uptime"`    // seconds
	WireGuardUptime int64             `json:"wireguard_uptime"` // seconds
	PeerEndpoints   map[string]string `json:"peer_endpoints"`   // Map of peer public key to endpoint

	// PeerHandshakes holds the Unix timestamp of the most-recent WireGuard
	// handshake for each peer, keyed by peer public key.  Reported by jump-peer
	// agents (via `wg show <iface> latest-handshakes`).  The server uses these
	// timestamps instead of endpoint presence to update wgLastSeen: a handshake
	// is the ground-truth liveness indicator because WireGuard re-handshakes
	// every ~180 s — a stale handshake (> 185 s ago) means the tunnel is down
	// even though `wg show endpoints` still shows the last known endpoint.
	//
	// When this field is absent (older agents), the server falls back to the
	// previous endpoint-presence logic.
	PeerHandshakes map[string]int64 `json:"peer_handshakes,omitempty"` // pubkey → Unix timestamp

	// LocalAllowedIPs is the list of CIDRs configured in this peer's WireGuard
	// AllowedIPs (i.e. what THIS peer routes through the VPN).  Reported by every
	// agent on every heartbeat.  Consumed by the jump peer's DNS server to decide
	// whether to redirect external DNS for unauthenticated peers (full-tunnel
	// peers get aggressive redirection; split-tunnel peers don't).
	//
	// Empty / unset means "unknown" — the jump peer falls back to the previous
	// behaviour (only intercept probe domains and internal VPN names).
	LocalAllowedIPs []string `json:"local_allowed_ips,omitempty"`

	// EndpointTakeovers reports cases where, since the previous heartbeat, this
	// agent observed the WireGuard endpoint of an authenticated peer flip from
	// the IP:port that was authenticated to a different IP:port.  Each entry
	// asks the server to add a denylist rule blocking the rogue source at the
	// physical interface, preventing the rogue source from completing further
	// WireGuard handshakes and stealing the peer slot.
	//
	// Only jump-peer agents populate this field (they are the only agents whose
	// `wg show endpoints` lists other peers).
	EndpointTakeovers []EndpointTakeoverReport `json:"endpoint_takeovers,omitempty"`
}

// EndpointTakeoverReport is a single rogue-source observation reported by the
// jump-peer agent to the server.  See AgentHeartbeat.EndpointTakeovers.
type EndpointTakeoverReport struct {
	WgIP            string `json:"wg_ip"`             // targeted peer's WireGuard private IP
	AuthenticatedAt string `json:"authenticated_at"`  // the IP:port stored at last successful auth
	ObservedAt      string `json:"observed_at"`       // the rogue IP:port now seen on `wg show endpoints`
}

// PeerConnectivityStatus describes whether a peer is currently reachable.
// Replaces the old PeerSessionStatus which mixed in security-incident concerns.
type PeerConnectivityStatus struct {
	PeerID         string        `json:"peer_id"`
	HasActiveAgent bool          `json:"has_active_agent"`
	CurrentSession *AgentSession `json:"current_session,omitempty"`
	LastChecked    time.Time     `json:"last_checked"`

	// CaptivePortalState is the peer's current captive-portal authentication
	// state, computed server-side from the whitelist, pending-token, and
	// quarantine tables.  Possible values:
	//   "authenticated"  — peer completed OIDC auth and is in a jump peer's whitelist
	//   "pending_auth"   — peer has an active (unused) captive-portal token
	//   "quarantined"    — peer exceeded auth-failure threshold; access blocked
	//   ""               — no auth record (new / un-authenticated peer)
	CaptivePortalState string `json:"captive_portal_state,omitempty"`
}
