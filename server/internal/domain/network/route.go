package network

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// Route represents a network destination (IP range) that is added to AllowedIPs
// in regular peers' WireGuard configurations when attached to a group, reachable
// through a specific jump peer.
//
// Dual-stack: a route may declare DestinationCIDR (IPv4), DestinationCIDRv6
// (IPv6), or BOTH.  When both are set, attached peers get BOTH CIDRs in their
// AllowedIPs — useful for a single "full tunnel" or "internet" route that
// covers both address families with one entity instead of two parallel rows.
// Migration 027 enforces at the DB level that at least one is set.
type Route struct {
	ID                string    `json:"id"`
	NetworkID         string    `json:"network_id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	DestinationCIDR   string    `json:"destination_cidr,omitempty"`    // IPv4 CIDR (optional if v6 is set)
	DestinationCIDRv6 string    `json:"destination_cidr_v6,omitempty"` // IPv6 CIDR (optional if v4 is set)
	JumpPeerID        string    `json:"jump_peer_id"`                  // Gateway jump peer
	DomainSuffix      string    `json:"domain_suffix"`                 // Custom domain (default: .internal)
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// RouteCreateRequest represents the data needed to create a new route.  At
// least one of DestinationCIDR / DestinationCIDRv6 must be provided.
type RouteCreateRequest struct {
	Name              string `json:"name" binding:"required"`
	Description       string `json:"description"`
	DestinationCIDR   string `json:"destination_cidr,omitempty"`
	DestinationCIDRv6 string `json:"destination_cidr_v6,omitempty"`
	JumpPeerID        string `json:"jump_peer_id" binding:"required"`
	DomainSuffix      string `json:"domain_suffix"`
}

// RouteUpdateRequest represents the data that can be updated for a route.
// Empty strings are interpreted as "leave unchanged" (use a sentinel like
// "-" if you ever need an explicit "clear this field").
type RouteUpdateRequest struct {
	Name              string `json:"name,omitempty"`
	Description       string `json:"description,omitempty"`
	DestinationCIDR   string `json:"destination_cidr,omitempty"`
	DestinationCIDRv6 string `json:"destination_cidr_v6,omitempty"`
	JumpPeerID        string `json:"jump_peer_id,omitempty"`
	DomainSuffix      string `json:"domain_suffix,omitempty"`
}

// Validate validates the route creation request
func (r *RouteCreateRequest) Validate() error {
	if err := validateRouteName(r.Name); err != nil {
		return err
	}
	// Dual-stack: at least one family, with each given CIDR matching its claimed family.
	if r.DestinationCIDR == "" && r.DestinationCIDRv6 == "" {
		return errors.New("at least one of destination_cidr or destination_cidr_v6 must be set")
	}
	if r.DestinationCIDR != "" {
		if err := ValidateCIDRFamily(r.DestinationCIDR, false); err != nil {
			return fmt.Errorf("destination_cidr: %w", err)
		}
	}
	if r.DestinationCIDRv6 != "" {
		if err := ValidateCIDRFamily(r.DestinationCIDRv6, true); err != nil {
			return fmt.Errorf("destination_cidr_v6: %w", err)
		}
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

// Validate validates the route update request.  Note: this checks the SHAPE
// of any non-empty fields but does NOT enforce the "at least one family"
// invariant — that's only meaningful in the context of the merged record,
// which the service layer applies before persisting.
func (r *RouteUpdateRequest) Validate() error {
	if r.Name != "" {
		if err := validateRouteName(r.Name); err != nil {
			return err
		}
	}
	if r.DestinationCIDR != "" {
		if err := ValidateCIDRFamily(r.DestinationCIDR, false); err != nil {
			return fmt.Errorf("destination_cidr: %w", err)
		}
	}
	if r.DestinationCIDRv6 != "" {
		if err := ValidateCIDRFamily(r.DestinationCIDRv6, true); err != nil {
			return fmt.Errorf("destination_cidr_v6: %w", err)
		}
	}
	if r.DomainSuffix != "" {
		if err := validateDomainSuffix(r.DomainSuffix); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCIDR validates a CIDR notation (any family).  Kept for backwards
// compatibility with callers outside the route package.
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

// ValidateCIDRFamily validates a CIDR and asserts it belongs to the given
// address family — wantV6=false → IPv4 expected, wantV6=true → IPv6 expected.
// Used by route validation to reject "IPv6 CIDR submitted in destination_cidr"
// and the symmetric mistake.
func ValidateCIDRFamily(cidr string, wantV6 bool) error {
	if cidr == "" {
		return errors.New("CIDR cannot be empty")
	}
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return errors.New("invalid CIDR format")
	}
	isV6 := ip.To4() == nil
	if wantV6 && !isV6 {
		return errors.New("expected an IPv6 CIDR")
	}
	if !wantV6 && isV6 {
		return errors.New("expected an IPv4 CIDR")
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
	if len(suffix) > 253 {
		return errors.New("domain suffix cannot exceed 253 characters")
	}
	// Basic domain validation - should not contain spaces or invalid characters
	if strings.ContainsAny(suffix, " \n\r\t_") {
		return errors.New("domain suffix cannot contain spaces, newlines, tabs, or underscores")
	}
	// Must be a valid domain format (letters, numbers, dots, hyphens)
	for _, char := range suffix {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '.' && char != '-' {
			return errors.New("domain suffix can only contain alphanumeric characters, dots, and hyphens")
		}
	}
	// Cannot start or end with dot or hyphen
	if strings.HasPrefix(suffix, ".") || strings.HasSuffix(suffix, ".") ||
		strings.HasPrefix(suffix, "-") || strings.HasSuffix(suffix, "-") {
		return errors.New("domain suffix cannot start or end with a dot or hyphen")
	}
	return nil
}
