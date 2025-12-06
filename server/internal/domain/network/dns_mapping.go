package network

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// DNSMapping represents a domain name to IP address mapping in the internal DNS system
type DNSMapping struct {
	ID        string    `json:"id"`
	RouteID   string    `json:"route_id"`
	Name      string    `json:"name"`       // DNS name (e.g., "server1")
	IPAddress string    `json:"ip_address"` // IP within route's CIDR
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DNSMappingCreateRequest represents the data needed to create a new DNS mapping
type DNSMappingCreateRequest struct {
	Name      string `json:"name" binding:"required"`
	IPAddress string `json:"ip_address" binding:"required"`
}

// DNSMappingUpdateRequest represents the data that can be updated for a DNS mapping
type DNSMappingUpdateRequest struct {
	Name      string `json:"name,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
}

// GetFQDN returns the fully qualified domain name
// Format: name.route_name.domain_suffix
func (d *DNSMapping) GetFQDN(route *Route) string {
	suffix := route.DomainSuffix
	if suffix == "" {
		suffix = "internal"
	}
	return fmt.Sprintf("%s.%s.%s", d.Name, route.Name, suffix)
}

// Validate validates the DNS mapping creation request
func (r *DNSMappingCreateRequest) Validate() error {
	if err := validateDNSName(r.Name); err != nil {
		return err
	}
	if err := ValidateIPAddress(r.IPAddress); err != nil {
		return err
	}
	return nil
}

// Validate validates the DNS mapping update request
func (r *DNSMappingUpdateRequest) Validate() error {
	if r.Name != "" {
		if err := validateDNSName(r.Name); err != nil {
			return err
		}
	}
	if r.IPAddress != "" {
		if err := ValidateIPAddress(r.IPAddress); err != nil {
			return err
		}
	}
	return nil
}

// ValidateIPAddress validates an IP address
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

// validateDNSName validates a DNS name
func validateDNSName(name string) error {
	if name == "" {
		return errors.New("DNS name cannot be empty")
	}
	if len(name) > 255 {
		return errors.New("DNS name cannot exceed 255 characters")
	}
	// Check for invalid characters - DNS names should be alphanumeric with hyphens
	if strings.ContainsAny(name, " \n\r\t") {
		return errors.New("DNS name cannot contain spaces, newlines, or tabs")
	}
	return nil
}
