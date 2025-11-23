package network

import "time"

// CaptivePortalToken represents a temporary token for captive portal authentication
type CaptivePortalToken struct {
	Token      string    `json:"token"`
	NetworkID  string    `json:"network_id"`
	JumpPeerID string    `json:"jump_peer_id"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// IsExpired checks if the token has expired
func (t *CaptivePortalToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsValid checks if the token is still valid
func (t *CaptivePortalToken) IsValid() bool {
	return !t.IsExpired()
}
