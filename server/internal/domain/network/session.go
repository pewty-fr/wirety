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
}

// PeerConnectivityStatus describes whether a peer is currently reachable.
// Replaces the old PeerSessionStatus which mixed in security-incident concerns.
type PeerConnectivityStatus struct {
	PeerID         string        `json:"peer_id"`
	HasActiveAgent bool          `json:"has_active_agent"`
	CurrentSession *AgentSession `json:"current_session,omitempty"`
	LastChecked    time.Time     `json:"last_checked"`
}
