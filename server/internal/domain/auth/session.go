package auth

import "time"

// Session represents a user session with refresh token
type Session struct {
	SessionHash            string    `json:"session_hash"`
	UserID                 string    `json:"user_id"`
	AccessToken            string    `json:"-"` // Never expose in JSON
	RefreshToken           string    `json:"-"` // Never expose in JSON
	AccessTokenExpiresAt   time.Time `json:"access_token_expires_at"`
	RefreshTokenExpiresAt  time.Time `json:"refresh_token_expires_at"`
	CreatedAt              time.Time `json:"created_at"`
	LastUsedAt             time.Time `json:"last_used_at"`
}

// IsAccessTokenExpired checks if the access token has expired
func (s *Session) IsAccessTokenExpired() bool {
	return time.Now().After(s.AccessTokenExpiresAt)
}

// IsRefreshTokenExpired checks if the refresh token has expired
func (s *Session) IsRefreshTokenExpired() bool {
	return time.Now().After(s.RefreshTokenExpiresAt)
}

// IsValid checks if the session is still valid (refresh token not expired)
func (s *Session) IsValid() bool {
	return !s.IsRefreshTokenExpired()
}
