package network

import "time"

// CaptivePortalToken represents a temporary token for captive portal authentication
type CaptivePortalToken struct {
	Token        string    `json:"token"`
	NetworkID    string    `json:"network_id"`
	JumpPeerID   string    `json:"jump_peer_id"`
	PeerIP       string    `json:"peer_ip"`                 // WireGuard private IP of the connecting peer
	PeerEndpoint string    `json:"peer_endpoint,omitempty"` // full public endpoint "ip:port" at connect time
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`

	// ConsumeState is set by the /captive-portal/start endpoint when the agent's
	// redirect first delivers the URL to a browser.  It is also stored as a
	// same-origin cookie (cp_state) on that browser's session.  The /authenticate
	// endpoint requires the cookie value to equal this field — phishing
	// resistance: a URL alone cannot consume the token, the consuming browser
	// must be the one that received it via the captive-portal-bouncer.
	//
	// Empty string = no state set yet (token freshly created by the agent);
	// the auth endpoint MUST refuse to consume tokens without a state to
	// prevent the legacy bypass path.
	ConsumeState string `json:"consume_state,omitempty"`
}

// IsExpired checks if the token has expired
func (t *CaptivePortalToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid checks if the token is still valid
func (t *CaptivePortalToken) IsValid() bool {
	return !t.IsExpired()
}
