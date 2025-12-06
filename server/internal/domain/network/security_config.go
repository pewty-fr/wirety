package network

import (
	"errors"
	"time"
)

// SecurityConfig represents the configurable security settings for a network
type SecurityConfig struct {
	ID                       string        `json:"id"`
	NetworkID                string        `json:"network_id"`
	Enabled                  bool          `json:"enabled"`                      // Master toggle for security incident detection
	SessionConflictThreshold time.Duration `json:"session_conflict_threshold"`   // Time window to consider sessions as conflicting
	EndpointChangeThreshold  time.Duration `json:"endpoint_change_threshold"`    // Minimum time between endpoint changes to not be suspicious
	MaxEndpointChangesPerDay int           `json:"max_endpoint_changes_per_day"` // Maximum number of endpoint changes per day before flagging as suspicious
	CreatedAt                time.Time     `json:"created_at"`
	UpdatedAt                time.Time     `json:"updated_at"`
}

// SecurityConfigUpdateRequest represents the data that can be updated for security config
type SecurityConfigUpdateRequest struct {
	Enabled                  *bool          `json:"enabled,omitempty"`
	SessionConflictThreshold *time.Duration `json:"session_conflict_threshold,omitempty"`
	EndpointChangeThreshold  *time.Duration `json:"endpoint_change_threshold,omitempty"`
	MaxEndpointChangesPerDay *int           `json:"max_endpoint_changes_per_day,omitempty"`
}

// DefaultSecurityConfig returns the default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled:                  true,
		SessionConflictThreshold: 5 * time.Minute,
		EndpointChangeThreshold:  30 * time.Minute,
		MaxEndpointChangesPerDay: 10,
	}
}

// Validate validates the security configuration
func (c *SecurityConfig) Validate() error {
	if c.SessionConflictThreshold < time.Minute {
		return errors.New("session conflict threshold must be at least 1 minute")
	}
	if c.EndpointChangeThreshold < time.Minute {
		return errors.New("endpoint change threshold must be at least 1 minute")
	}
	if c.MaxEndpointChangesPerDay < 1 || c.MaxEndpointChangesPerDay > 1000 {
		return errors.New("max endpoint changes per day must be between 1 and 1000")
	}
	return nil
}

// Validate validates the security config update request
func (r *SecurityConfigUpdateRequest) Validate() error {
	if r.SessionConflictThreshold != nil && *r.SessionConflictThreshold < time.Minute {
		return errors.New("session conflict threshold must be at least 1 minute")
	}
	if r.EndpointChangeThreshold != nil && *r.EndpointChangeThreshold < time.Minute {
		return errors.New("endpoint change threshold must be at least 1 minute")
	}
	if r.MaxEndpointChangesPerDay != nil && (*r.MaxEndpointChangesPerDay < 1 || *r.MaxEndpointChangesPerDay > 1000) {
		return errors.New("max endpoint changes per day must be between 1 and 1000")
	}
	return nil
}
