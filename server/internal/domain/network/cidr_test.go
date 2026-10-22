package network

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateNetworkCIDR(t *testing.T) {
	for _, cidr := range []string{"10.0.0.0/8", "10.0.0.13/32", "0.0.0.0/0", "fd00:10::/64", "fd00::13/128"} {
		if err := ValidateNetworkCIDR(cidr); err != nil {
			t.Errorf("ValidateNetworkCIDR(%q) = %v, want nil", cidr, err)
		}
	}
	for cidr, hint := range map[string]string{
		"10.0.0.1/8":    "use the network address 10.0.0.0/8",
		"fd00:10::1/64": "use the network address fd00:10::/64",
		"10.0.0.0":      "not in CIDR notation",
		"10.0.0.0/33":   "not in CIDR notation",
	} {
		err := ValidateNetworkCIDR(cidr)
		if !errors.Is(err, ErrInvalidCIDR) || !strings.Contains(err.Error(), hint) {
			t.Errorf("ValidateNetworkCIDR(%q) = %v, want an ErrInvalidCIDR saying %q", cidr, err, hint)
		}
	}
}

func TestValidateAllowedIP(t *testing.T) {
	for _, s := range []string{"192.168.1.10", "fd00::1", "192.168.1.0/24"} {
		if err := ValidateAllowedIP(s); err != nil {
			t.Errorf("ValidateAllowedIP(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range []string{"192.168.1.10/24", "not-an-ip", ""} {
		if err := ValidateAllowedIP(s); !errors.Is(err, ErrInvalidCIDR) {
			t.Errorf("ValidateAllowedIP(%q) = %v, want ErrInvalidCIDR", s, err)
		}
	}
}

func TestRouteDestinationsHaveNoHostBits(t *testing.T) {
	req := &RouteCreateRequest{Name: "dc", DestinationCIDR: "10.0.0.1/16", JumpPeerID: "jump"}
	err := req.Validate()
	if err == nil || !strings.Contains(err.Error(), "destination_cidr: invalid CIDR: 10.0.0.1/16 has host bits set, use the network address 10.0.0.0/16") {
		t.Errorf("IPv4 destination with host bits: got %v", err)
	}
	update := &RouteUpdateRequest{DestinationCIDRv6: new("fd00:10::5/64")}
	if err := update.Validate(); err == nil || !strings.Contains(err.Error(), "fd00:10::/64") {
		t.Errorf("IPv6 destination with host bits: got %v", err)
	}
	if err := ValidateCIDRFamily("10.0.0.0/16", false); err != nil {
		t.Errorf("network address refused: %v", err)
	}
}

func TestPolicyRuleCIDRTargetHasNoHostBits(t *testing.T) {
	rule := &PolicyRule{Direction: "output", Action: "allow", TargetType: "cidr", Target: "10.0.1.5/24"}
	if err := rule.Validate(); !errors.Is(err, ErrInvalidCIDR) || !strings.Contains(err.Error(), "10.0.1.0/24") {
		t.Errorf("rule target with host bits: got %v", err)
	}
	rule.Target = "10.0.0.13/32"
	if err := rule.Validate(); err != nil {
		t.Errorf("single-host target refused: %v", err)
	}
}
