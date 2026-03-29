package auth

import "time"

// APIToken represents a long-lived API token for programmatic access.
// The raw token is shown exactly once at creation; only its SHA-256 hash is stored.
type APIToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`          // SHA-256 hex — never serialised
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"` // nil = never expires
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// APITokenCreateRequest carries the parameters for creating a new API token.
type APITokenCreateRequest struct {
	Name      string     `json:"name" binding:"required"`
	ExpiresAt *time.Time `json:"expires_at"` // nil = never expires
}

// APITokenResponse is returned to the client.
// RawToken is only populated on creation — it is never stored or returned again.
type APITokenResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	RawToken   string     `json:"token,omitempty"` // non-empty only on first creation
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}
