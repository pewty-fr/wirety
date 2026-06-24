package network

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// DNSMapping represents a domain name to IP address mapping in the internal
// DNS system.  May carry an IPv4 address, an IPv6 address, or both — when both
// are set, the agent's DNS server returns the IPv4 for A queries and the IPv6
// for AAAA queries on the same hostname.  Migration 027 enforces at the DB
// level that at least one of IPAddress / IPv6Address is populated.
type DNSMapping struct {
	ID          string    `json:"id"`
	RouteID     string    `json:"route_id"`
	Name        string    `json:"name"`                    // DNS name (e.g., "server1")
	IPAddress   string    `json:"ip_address,omitempty"`    // IPv4 address (optional if v6 set)
	IPv6Address string    `json:"ip_address_v6,omitempty"` // IPv6 address (optional if v4 set)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DNSMappingCreateRequest represents the data needed to create a new DNS
// mapping.  At least one of IPAddress / IPv6Address must be provided.
type DNSMappingCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	IPAddress   string `json:"ip_address,omitempty"`
	IPv6Address string `json:"ip_address_v6,omitempty"`
}

// DNSMappingUpdateRequest represents the data that can be updated for a DNS
// mapping.  Empty strings are interpreted as "leave unchanged".
type DNSMappingUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	IPAddress   string `json:"ip_address,omitempty"`
	IPv6Address string `json:"ip_address_v6,omitempty"`
}

// GetFQDN returns the fully qualified domain name for this DNS mapping.
//
// Format: <record-name>.<network-name>.<network-domain-suffix>
// (the suffix defaults to "internal" when empty).
//
// The route that hosts the mapping is intentionally NOT part of the FQDN:
// DNS records live in the network's namespace regardless of which route they
// belong to.  Routes are an internal grouping/gating concept — renaming a
// route, or moving a record between routes, must not change the name peers
// resolve to.
func (d *DNSMapping) GetFQDN(network *Network) string {
	suffix := network.DomainSuffix
	if suffix == "" {
		suffix = "internal"
	}
	return fmt.Sprintf("%s.%s.%s", d.Name, network.Name, suffix)
}

// Validate validates the DNS mapping creation request.  Requires at least one
// of IPAddress / IPv6Address to be set, with each given address matching its
// claimed family.
func (r *DNSMappingCreateRequest) Validate() error {
	if err := validateDNSName(r.Name); err != nil {
		return err
	}
	if r.IPAddress == "" && r.IPv6Address == "" {
		return errors.New("at least one of ip_address or ip_address_v6 must be set")
	}
	if r.IPAddress != "" {
		if err := ValidateIPAddressFamily(r.IPAddress, false); err != nil {
			return fmt.Errorf("ip_address: %w", err)
		}
	}
	if r.IPv6Address != "" {
		if err := ValidateIPAddressFamily(r.IPv6Address, true); err != nil {
			return fmt.Errorf("ip_address_v6: %w", err)
		}
	}
	return nil
}

// Validate validates the DNS mapping update request.  Note: this does NOT
// enforce "at least one address must remain set" — that's only meaningful
// in the context of the merged record after applying the update, which the
// service layer checks before persisting.
func (r *DNSMappingUpdateRequest) Validate() error {
	if r.Name != "" {
		if err := validateDNSName(r.Name); err != nil {
			return err
		}
	}
	if r.IPAddress != "" {
		if err := ValidateIPAddressFamily(r.IPAddress, false); err != nil {
			return fmt.Errorf("ip_address: %w", err)
		}
	}
	if r.IPv6Address != "" {
		if err := ValidateIPAddressFamily(r.IPv6Address, true); err != nil {
			return fmt.Errorf("ip_address_v6: %w", err)
		}
	}
	return nil
}

// ValidateIPAddress validates an IP address (any family).  Kept for backwards
// compatibility with callers outside the dns_mapping package.
func ValidateIPAddress(ip string) error {
	if ip == "" {
		return errors.New("IP address cannot be empty")
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return errors.New("invalid IP address format")
	}
	return nil
}

// ValidateIPAddressFamily validates an IP address and asserts it belongs to
// the given family.  wantV6=true → IPv6 expected, wantV6=false → IPv4 expected.
func ValidateIPAddressFamily(ip string, wantV6 bool) error {
	if ip == "" {
		return errors.New("IP address cannot be empty")
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return errors.New("invalid IP address format")
	}
	isV6 := parsedIP.To4() == nil
	if wantV6 && !isV6 {
		return errors.New("expected an IPv6 address")
	}
	if !wantV6 && isV6 {
		return errors.New("expected an IPv4 address")
	}
	return nil
}

// ValidateIPInCIDR validates that an IP address is within a CIDR range
func ValidateIPInCIDR(ip, cidr string) error {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return errors.New("invalid IP address format")
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return errors.New("invalid CIDR format")
	}

	if !ipNet.Contains(parsedIP) {
		return fmt.Errorf("IP address %s is not within CIDR %s", ip, cidr)
	}

	return nil
}

// validateDNSName validates a DNS name.
//
// Supported forms:
//   - A single DNS label: alphanumeric + hyphens, 1–63 chars, no leading/trailing hyphen.
//   - A bare wildcard:    "*" — becomes *.network.suffix when resolved.
//   - A wildcard prefix:  "*.label[.label…]" — matches one additional label deep.
//     e.g. "*.api" → "*.api.network.suffix" matches "v1.api.network.suffix".
//
// Wildcard records follow RFC 4592: the "*" covers exactly one label, so
// "*.api.internal" matches "v1.api.internal" but NOT "x.v1.api.internal".
// Fully-specified records always take priority over wildcard matches.
func validateDNSName(name string) error {
	if name == "" {
		return errors.New("DNS name cannot be empty")
	}

	// Wildcard name: "*" or "*.one-or-more-labels"
	if strings.HasPrefix(name, "*") {
		if name == "*" {
			return nil // bare wildcard — valid
		}
		if !strings.HasPrefix(name, "*.") {
			return errors.New("DNS wildcard must be exactly '*' or start with '*.'")
		}
		rest := name[2:] // labels after "*."
		if rest == "" {
			return errors.New("wildcard DNS name must have at least one label after '*.'")
		}
		for _, label := range strings.Split(rest, ".") {
			if label == "" {
				return errors.New("wildcard DNS name cannot have empty labels")
			}
			if len(label) > 63 {
				return errors.New("DNS label cannot exceed 63 characters")
			}
			for _, ch := range label {
				if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') &&
					(ch < '0' || ch > '9') && ch != '-' {
					return errors.New("DNS name can only contain alphanumeric characters and hyphens")
				}
			}
			if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return errors.New("DNS label cannot start or end with a hyphen")
			}
		}
		return nil
	}

	// Non-wildcard: single label (dots are separators in the FQDN, not allowed here)
	if len(name) > 63 {
		return errors.New("DNS name cannot exceed 63 characters")
	}
	if strings.ContainsAny(name, " \n\r\t") {
		return errors.New("DNS name cannot contain spaces, newlines, or tabs")
	}
	for _, char := range name {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '-' {
			return errors.New("DNS name can only contain alphanumeric characters and hyphens")
		}
	}
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		return errors.New("DNS name cannot start or end with a hyphen")
	}
	return nil
}
