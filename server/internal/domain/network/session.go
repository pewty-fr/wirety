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

// EndpointChange represents a detected endpoint change for a peer
// This tracks changes as observed by the jump server from WireGuard handshakes.
// The PeerID refers to the peer whose endpoint changed (identified by public key),
// NOT the jump server that observed the change.
type EndpointChange struct {
	PeerID      string    `json:"peer_id"`      // ID of the peer whose endpoint changed
	OldEndpoint string    `json:"old_endpoint"` // Previous endpoint (IP:port)
	NewEndpoint string    `json:"new_endpoint"` // New endpoint (IP:port)
	ChangedAt   time.Time `json:"changed_at"`   // When the change was detected
	Source      string    `json:"source"`       // "agent" (self-reported) or "wireguard" (observed from handshakes)
}

// PeerSessionStatus represents the security status of a peer
type PeerSessionStatus struct {
	PeerID                string           `json:"peer_id"`
	HasActiveAgent        bool             `json:"has_active_agent"`
	CurrentSession        *AgentSession    `json:"current_session,omitempty"`
	ConflictingSessions   []AgentSession   `json:"conflicting_sessions,omitempty"` // Multiple agents detected
	RecentEndpointChanges []EndpointChange `json:"recent_endpoint_changes,omitempty"`
	SuspiciousActivity    bool             `json:"suspicious_activity"` // Rapid endpoint changes
	LastChecked           time.Time        `json:"last_checked"`
}

// SessionConflictThreshold is the time window to consider sessions as conflicting
const SessionConflictThreshold = 5 * time.Minute

// EndpointChangeThreshold is the minimum time between endpoint changes to not be suspicious
const EndpointChangeThreshold = 30 * time.Minute

// MaxEndpointChangesPerDay is the maximum number of endpoint changes per day before flagging as suspicious
const MaxEndpointChangesPerDay = 10

// SecurityIncident represents a security incident (shared config, session conflict, etc.)
type SecurityIncident struct {
	ID           string    `json:"id"`
	PeerID       string    `json:"peer_id"`
	PeerName     string    `json:"peer_name"`
	NetworkID    string    `json:"network_id"`
	NetworkName  string    `json:"network_name"`
	IncidentType string    `json:"incident_type"` // "shared_config", "session_conflict", "suspicious_activity"
	DetectedAt   time.Time `json:"detected_at"`
	PublicKey    string    `json:"public_key"` // The public key involved in the incident
	Endpoints    []string  `json:"endpoints"`  // List of endpoints involved
	Details      string    `json:"details"`    // Human-readable description
	Resolved     bool      `json:"resolved"`
	ResolvedAt   time.Time `json:"resolved_at,omitempty"`
	ResolvedBy   string    `json:"resolved_by,omitempty"`
}

// IncidentType constants
const (
	IncidentTypeSharedConfig       = "shared_config"
	IncidentTypeSessionConflict    = "session_conflict"
	IncidentTypeSuspiciousActivity = "suspicious_activity"
)
