package network

import (
	"errors"
	"net"
	"strings"
	"time"
)

// Route represents a network destination (IP range) that is added to AllowedIPs in regular peer
// WireGuard configurations when attached to a group, accessible through a specific jump peer
type Route struct {
	ID              string    `json:"id"`
	NetworkID       string    `json:"network_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	DestinationCIDR string    `json:"destination_cidr"` // External network range
	JumpPeerID      string    `json:"jump_peer_id"`     // Gateway jump peer
	DomainSuffix    string    `json:"domain_suffix"`    // Custom domain (default: .internal)
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RouteCreateRequest represents the data needed to create a new route
type RouteCreateRequest struct {
	Name            string `json:"name" binding:"required"`
	Description     string `json:"description"`
	DestinationCIDR string `json:"destination_cidr" binding:"required"`
	JumpPeerID      string `json:"jump_peer_id" binding:"required"`
	DomainSuffix    string `json:"domain_suffix"`
}

// RouteUpdateRequest represents the data that can be updated for a route
type RouteUpdateRequest struct {
	Name            string `json:"name,omitempty"`
	Description     string `json:"description,omitempty"`
	DestinationCIDR string `json:"destination_cidr,omitempty"`
	JumpPeerID      string `json:"jump_peer_id,omitempty"`
	DomainSuffix    string `json:"domain_suffix,omitempty"`
}

// Validate validates the route creation request
func (r *RouteCreateRequest) Validate() error {
	if err := validateRouteName(r.Name); err != nil {
		return err
	}
	if err := ValidateCIDR(r.DestinationCIDR); err != nil {
		return err
	}
	if r.JumpPeerID == "" {
		return errors.New("jump peer ID cannot be empty")
	}
	if r.DomainSuffix != "" {
		if err := validateDomainSuffix(r.DomainSuffix); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates the route update request
func (r *RouteUpdateRequest) Validate() error {
	if r.Name != "" {
		if err := validateRouteName(r.Name); err != nil {
			return err
		}
	}
	if r.DestinationCIDR != "" {
		if err := ValidateCIDR(r.DestinationCIDR); err != nil {
			return err
		}
	}
	if r.DomainSuffix != "" {
		if err := validateDomainSuffix(r.DomainSuffix); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCIDR validates a CIDR notation
func ValidateCIDR(cidr string) error {
	if cidr == "" {
		return errors.New("CIDR cannot be empty")
	}
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return errors.New("invalid CIDR format")
	}
	return nil
}

// validateRouteName validates a route name
func validateRouteName(name string) error {
	if name == "" {
		return errors.New("route name cannot be empty")
	}
	if len(name) > 255 {
		return errors.New("route name cannot exceed 255 characters")
	}
	// Check for invalid characters
	if strings.ContainsAny(name, "\n\r\t") {
		return errors.New("route name cannot contain newlines or tabs")
	}
	return nil
}

// validateDomainSuffix validates a domain suffix
func validateDomainSuffix(suffix string) error {
	if suffix == "" {
		return nil // Empty is allowed, will use default
	}
	if len(suffix) > 255 {
		return errors.New("domain suffix cannot exceed 255 characters")
	}
	// Basic domain validation - should not contain spaces or invalid characters
	if strings.ContainsAny(suffix, " \n\r\t") {
		return errors.New("domain suffix cannot contain spaces, newlines, or tabs")
	}
	return nil
}
