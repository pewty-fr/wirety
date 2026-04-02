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
	EndpointChangeThreshold  time.Duration `json:"endpoint_change_threshold"`    // Window to detect IP-level changes (shared config detection)
	MaxEndpointChangesPerDay int           `json:"max_endpoint_changes_per_day"` // Maximum number of IP-level endpoint changes per day
	PortChangeThreshold      time.Duration `json:"port_change_threshold"`        // Window to detect port-only changes (NAT rebinding)
	MaxPortChangesPerWindow  int           `json:"max_port_changes_per_window"`  // Max port-only changes within window before flagging
	CreatedAt                time.Time     `json:"created_at"`
	UpdatedAt                time.Time     `json:"updated_at"`
}

// SecurityConfigUpdateRequest represents the data that can be updated for security config
type SecurityConfigUpdateRequest struct {
	Enabled                  *bool          `json:"enabled,omitempty"`
	SessionConflictThreshold *time.Duration `json:"session_conflict_threshold,omitempty"`
	EndpointChangeThreshold  *time.Duration `json:"endpoint_change_threshold,omitempty"`
	MaxEndpointChangesPerDay *int           `json:"max_endpoint_changes_per_day,omitempty"`
	PortChangeThreshold      *time.Duration `json:"port_change_threshold,omitempty"`
	MaxPortChangesPerWindow  *int           `json:"max_port_changes_per_window,omitempty"`
}

// DefaultSecurityConfig returns the default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		Enabled:                  true,
		SessionConflictThreshold: 5 * time.Minute,
		EndpointChangeThreshold:  5 * time.Minute,
		MaxEndpointChangesPerDay: 10,
		PortChangeThreshold:      5 * time.Minute,
		MaxPortChangesPerWindow:  5,
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
	if c.PortChangeThreshold < time.Minute {
		return errors.New("port change threshold must be at least 1 minute")
	}
	if c.MaxPortChangesPerWindow < 1 || c.MaxPortChangesPerWindow > 1000 {
		return errors.New("max port changes per window must be between 1 and 1000")
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
	if r.PortChangeThreshold != nil && *r.PortChangeThreshold < time.Minute {
		return errors.New("port change threshold must be at least 1 minute")
	}
	if r.MaxPortChangesPerWindow != nil && (*r.MaxPortChangesPerWindow < 1 || *r.MaxPortChangesPerWindow > 1000) {
		return errors.New("max port changes per window must be between 1 and 1000")
	}
	return nil
}
