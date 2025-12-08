package network

import (
	"errors"
	"net"
	"strings"
	"time"
)

// Policy represents a set of iptables rules applied on jump peers to filter traffic between peers
type Policy struct {
	ID          string       `json:"id"`
	NetworkID   string       `json:"network_id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Rules       []PolicyRule `json:"rules"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// PolicyRule represents a specific allow or deny iptables rule for IP ranges or peer traffic
type PolicyRule struct {
	ID          string `json:"id"`
	Direction   string `json:"direction"`   // "input" or "output"
	Action      string `json:"action"`      // "allow" or "deny"
	Target      string `json:"target"`      // IP/CIDR, peer ID, or group ID
	TargetType  string `json:"target_type"` // "cidr", "peer", "group"
	Description string `json:"description"`
}

// PolicyCreateRequest represents the data needed to create a new policy
type PolicyCreateRequest struct {
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description"`
	Rules       []PolicyRule `json:"rules"`
}

// PolicyUpdateRequest represents the data that can be updated for a policy
type PolicyUpdateRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// Validate validates the policy creation request
func (r *PolicyCreateRequest) Validate() error {
	if err := validatePolicyName(r.Name); err != nil {
		return err
	}
	for _, rule := range r.Rules {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates the policy update request
func (r *PolicyUpdateRequest) Validate() error {
	if r.Name != "" {
		if err := validatePolicyName(r.Name); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates a policy rule
func (r *PolicyRule) Validate() error {
	// Validate direction
	if r.Direction != "input" && r.Direction != "output" {
		return errors.New("policy rule direction must be 'input' or 'output'")
	}

	// Validate action
	if r.Action != "allow" && r.Action != "deny" {
		return errors.New("policy rule action must be 'allow' or 'deny'")
	}

	// Validate target type
	if r.TargetType != "cidr" && r.TargetType != "peer" && r.TargetType != "group" {
		return errors.New("policy rule target_type must be 'cidr', 'peer', or 'group'")
	}

	// Validate target based on type
	if r.Target == "" {
		return errors.New("policy rule target cannot be empty")
	}

	// If target type is CIDR, validate the CIDR format
	if r.TargetType == "cidr" {
		if _, _, err := net.ParseCIDR(r.Target); err != nil {
			return errors.New("policy rule target must be a valid CIDR when target_type is 'cidr'")
		}
	}

	return nil
}

// validatePolicyName validates a policy name
func validatePolicyName(name string) error {
	if name == "" {
		return errors.New("policy name cannot be empty")
	}
	if len(name) > 255 {
		return errors.New("policy name cannot exceed 255 characters")
	}
	// Check for invalid characters
	if strings.ContainsAny(name, "\n\r\t") {
		return errors.New("policy name cannot contain newlines or tabs")
	}
	return nil
}
