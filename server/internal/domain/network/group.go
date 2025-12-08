package network

import (
	"errors"
	"strings"
	"time"
)

// Group represents a collection of peers that share common characteristics or policies
type Group struct {
	ID          string    `json:"id"`
	NetworkID   string    `json:"network_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Priority    int       `json:"priority"`   // Priority for policy application order (0-999, lower = higher priority)
	PeerIDs     []string  `json:"peer_ids"`   // Member peer identifiers
	PolicyIDs   []string  `json:"policy_ids"` // Attached policy identifiers
	RouteIDs    []string  `json:"route_ids"`  // Attached route identifiers
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GroupCreateRequest represents the data needed to create a new group
type GroupCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Priority    *int   `json:"priority,omitempty"` // Optional priority (1-999), defaults to 100
}

// GroupUpdateRequest represents the data that can be updated for a group
type GroupUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Priority    *int   `json:"priority,omitempty"` // Optional priority (1-999)
}

// Validate validates the group name and priority
func (r *GroupCreateRequest) Validate() error {
	if err := validateGroupName(r.Name); err != nil {
		return err
	}
	if r.Priority != nil {
		if *r.Priority < 1 || *r.Priority > 999 {
			return errors.New("priority must be between 1 and 999")
		}
	}
	return nil
}

// Validate validates the group update request
func (r *GroupUpdateRequest) Validate() error {
	if r.Name != "" {
		if err := validateGroupName(r.Name); err != nil {
			return err
		}
	}
	if r.Priority != nil {
		if *r.Priority < 1 || *r.Priority > 999 {
			return errors.New("priority must be between 1 and 999")
		}
	}
	return nil
}

// validateGroupName validates a group name
func validateGroupName(name string) error {
	if name == "" {
		return errors.New("group name cannot be empty")
	}
	if len(name) > 64 {
		return errors.New("group name cannot exceed 64 characters")
	}
	// Check for invalid characters
	if strings.ContainsAny(name, "\n\r\t") {
		return errors.New("group name cannot contain newlines or tabs")
	}
	return nil
}
