package network

import (
	"testing"
	"time"
)

func TestValidateIPAddress(t *testing.T) {
	tests := []struct {
		name        string
		ip          string
		expectError bool
	}{
		{
			name:        "valid IPv4",
			ip:          "192.168.1.1",
			expectError: false,
		},
		{
			name:        "valid IPv6",
			ip:          "2001:db8::1",
			expectError: false,
		},
		{
			name:        "empty IP",
			ip:          "",
			expectError: true,
		},
		{
			name:        "invalid IP format",
			ip:          "256.256.256.256",
			expectError: true,
		},
		{
			name:        "invalid IP string",
			ip:          "not-an-ip",
			expectError: true,
		},
		{
			name:        "IP with CIDR notation",
			ip:          "192.168.1.1/24",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIPAddress(tt.ip)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestValidateIPInCIDR(t *testing.T) {
	tests := []struct {
		name        string
		ip          string
		cidr        string
		expectError bool
	}{
		{
			name:        "IP in CIDR range",
			ip:          "192.168.1.10",
			cidr:        "192.168.1.0/24",
			expectError: false,
		},
		{
			name:        "IP at network boundary",
			ip:          "192.168.1.1",
			cidr:        "192.168.1.0/24",
			expectError: false,
		},
		{
			name:        "IP at broadcast boundary",
			ip:          "192.168.1.254",
			cidr:        "192.168.1.0/24",
			expectError: false,
		},
		{
			name:        "IP outside CIDR range",
			ip:          "192.168.2.10",
			cidr:        "192.168.1.0/24",
			expectError: true,
		},
		{
			name:        "invalid IP",
			ip:          "not-an-ip",
			cidr:        "192.168.1.0/24",
			expectError: true,
		},
		{
			name:        "invalid CIDR",
			ip:          "192.168.1.10",
			cidr:        "not-a-cidr",
			expectError: true,
		},
		{
			name:        "empty IP",
			ip:          "",
			cidr:        "192.168.1.0/24",
			expectError: true,
		},
		{
			name:        "empty CIDR",
			ip:          "192.168.1.10",
			cidr:        "",
			expectError: true,
		},
		{
			name:        "IPv6 in range",
			ip:          "2001:db8::10",
			cidr:        "2001:db8::/32",
			expectError: false,
		},
		{
			name:        "IPv6 out of range",
			ip:          "2001:db9::10",
			cidr:        "2001:db8::/32",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIPInCIDR(tt.ip, tt.cidr)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestValidateCIDR(t *testing.T) {
	tests := []struct {
		name        string
		cidr        string
		expectError bool
	}{
		{
			name:        "valid IPv4 CIDR",
			cidr:        "192.168.1.0/24",
			expectError: false,
		},
		{
			name:        "valid IPv6 CIDR",
			cidr:        "2001:db8::/32",
			expectError: false,
		},
		{
			name:        "valid /32 CIDR",
			cidr:        "192.168.1.1/32",
			expectError: false,
		},
		{
			name:        "valid /0 CIDR",
			cidr:        "0.0.0.0/0",
			expectError: false,
		},
		{
			name:        "empty CIDR",
			cidr:        "",
			expectError: true,
		},
		{
			name:        "invalid CIDR format",
			cidr:        "192.168.1.0",
			expectError: true,
		},
		{
			name:        "invalid IP in CIDR",
			cidr:        "256.256.256.256/24",
			expectError: true,
		},
		{
			name:        "invalid prefix length",
			cidr:        "192.168.1.0/33",
			expectError: true,
		},
		{
			name:        "negative prefix length",
			cidr:        "192.168.1.0/-1",
			expectError: true,
		},
		{
			name:        "non-numeric prefix",
			cidr:        "192.168.1.0/abc",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCIDR(tt.cidr)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDNSMappingCreateRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     *DNSMappingCreateRequest
		expectError bool
	}{
		{
			name: "valid request",
			request: &DNSMappingCreateRequest{
				Name:      "server1",
				IPAddress: "192.168.1.10",
			},
			expectError: false,
		},
		{
			name: "empty name",
			request: &DNSMappingCreateRequest{
				Name:      "",
				IPAddress: "192.168.1.10",
			},
			expectError: true,
		},
		{
			name: "invalid IP",
			request: &DNSMappingCreateRequest{
				Name:      "server1",
				IPAddress: "not-an-ip",
			},
			expectError: true,
		},
		{
			name: "name too long",
			request: &DNSMappingCreateRequest{
				Name:      "this-is-a-very-long-dns-name-that-exceeds-the-maximum-allowed-length-for-dns-labels",
				IPAddress: "192.168.1.10",
			},
			expectError: true,
		},
		{
			name: "name with invalid characters",
			request: &DNSMappingCreateRequest{
				Name:      "server_1",
				IPAddress: "192.168.1.10",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDNSMappingUpdateRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     *DNSMappingUpdateRequest
		expectError bool
	}{
		{
			name: "valid request - name only",
			request: &DNSMappingUpdateRequest{
				Name: "server1",
			},
			expectError: false,
		},
		{
			name: "valid request - IP only",
			request: &DNSMappingUpdateRequest{
				IPAddress: "192.168.1.10",
			},
			expectError: false,
		},
		{
			name: "valid request - both fields",
			request: &DNSMappingUpdateRequest{
				Name:      "server1",
				IPAddress: "192.168.1.10",
			},
			expectError: false,
		},
		{
			name:        "empty request",
			request:     &DNSMappingUpdateRequest{},
			expectError: false,
		},
		{
			name: "invalid name",
			request: &DNSMappingUpdateRequest{
				Name: "server_1",
			},
			expectError: true,
		},
		{
			name: "invalid IP",
			request: &DNSMappingUpdateRequest{
				IPAddress: "not-an-ip",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestSecurityConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *SecurityConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &SecurityConfig{
				SessionConflictThreshold: 5 * time.Minute,
				EndpointChangeThreshold:  30 * time.Minute,
				MaxEndpointChangesPerDay: 10,
			},
			expectError: false,
		},
		{
			name: "session conflict threshold too low",
			config: &SecurityConfig{
				SessionConflictThreshold: 30 * time.Second,
				EndpointChangeThreshold:  30 * time.Minute,
				MaxEndpointChangesPerDay: 10,
			},
			expectError: true,
		},
		{
			name: "endpoint change threshold too low",
			config: &SecurityConfig{
				SessionConflictThreshold: 5 * time.Minute,
				EndpointChangeThreshold:  30 * time.Second,
				MaxEndpointChangesPerDay: 10,
			},
			expectError: true,
		},
		{
			name: "max endpoint changes too low",
			config: &SecurityConfig{
				SessionConflictThreshold: 5 * time.Minute,
				EndpointChangeThreshold:  30 * time.Minute,
				MaxEndpointChangesPerDay: 0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestPolicyRule_Validate(t *testing.T) {
	tests := []struct {
		name        string
		rule        *PolicyRule
		expectError bool
	}{
		{
			name: "valid input allow rule",
			rule: &PolicyRule{
				Direction:   "input",
				Action:      "allow",
				TargetType:  "cidr",
				Target:      "192.168.1.0/24",
				Description: "Allow internal network",
			},
			expectError: false,
		},
		{
			name: "valid output deny rule",
			rule: &PolicyRule{
				Direction:  "output",
				Action:     "deny",
				TargetType: "peer",
				Target:     "peer-123",
			},
			expectError: false,
		},
		{
			name: "invalid direction",
			rule: &PolicyRule{
				Direction:  "invalid",
				Action:     "allow",
				TargetType: "cidr",
				Target:     "192.168.1.0/24",
			},
			expectError: true,
		},
		{
			name: "invalid action",
			rule: &PolicyRule{
				Direction:  "input",
				Action:     "invalid",
				TargetType: "cidr",
				Target:     "192.168.1.0/24",
			},
			expectError: true,
		},
		{
			name: "invalid target type",
			rule: &PolicyRule{
				Direction:  "input",
				Action:     "allow",
				TargetType: "invalid",
				Target:     "192.168.1.0/24",
			},
			expectError: true,
		},
		{
			name: "empty target",
			rule: &PolicyRule{
				Direction:  "input",
				Action:     "allow",
				TargetType: "cidr",
				Target:     "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestPolicyCreateRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     *PolicyCreateRequest
		expectError bool
	}{
		{
			name: "valid request",
			request: &PolicyCreateRequest{
				Name:        "test-policy",
				Description: "Test policy",
				Rules: []PolicyRule{
					{
						Direction:  "input",
						Action:     "allow",
						TargetType: "cidr",
						Target:     "192.168.1.0/24",
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty name",
			request: &PolicyCreateRequest{
				Name: "",
				Rules: []PolicyRule{
					{
						Direction:  "input",
						Action:     "allow",
						TargetType: "cidr",
						Target:     "192.168.1.0/24",
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid rule",
			request: &PolicyCreateRequest{
				Name: "test-policy",
				Rules: []PolicyRule{
					{
						Direction:  "invalid",
						Action:     "allow",
						TargetType: "cidr",
						Target:     "192.168.1.0/24",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestRouteCreateRequest_Validate(t *testing.T) {
	tests := []struct {
		name        string
		request     *RouteCreateRequest
		expectError bool
	}{
		{
			name: "valid request",
			request: &RouteCreateRequest{
				Name:            "test-route",
				Description:     "Test route",
				DestinationCIDR: "192.168.1.0/24",
				JumpPeerID:      "jump-peer-1",
				DomainSuffix:    "example.com",
			},
			expectError: false,
		},
		{
			name: "empty name",
			request: &RouteCreateRequest{
				Name:            "",
				DestinationCIDR: "192.168.1.0/24",
				JumpPeerID:      "jump-peer-1",
			},
			expectError: true,
		},
		{
			name: "invalid CIDR",
			request: &RouteCreateRequest{
				Name:            "test-route",
				DestinationCIDR: "not-a-cidr",
				JumpPeerID:      "jump-peer-1",
			},
			expectError: true,
		},
		{
			name: "empty jump peer ID",
			request: &RouteCreateRequest{
				Name:            "test-route",
				DestinationCIDR: "192.168.1.0/24",
				JumpPeerID:      "",
			},
			expectError: true,
		},
		{
			name: "invalid domain suffix",
			request: &RouteCreateRequest{
				Name:            "test-route",
				DestinationCIDR: "192.168.1.0/24",
				JumpPeerID:      "jump-peer-1",
				DomainSuffix:    "invalid_domain",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestGroupCreateRequest_Validate(t *testing.T) {
	validPriority := 1
	invalidPriority := -1

	tests := []struct {
		name        string
		request     *GroupCreateRequest
		expectError bool
	}{
		{
			name: "valid request",
			request: &GroupCreateRequest{
				Name:        "test-group",
				Description: "Test group",
				Priority:    &validPriority,
			},
			expectError: false,
		},
		{
			name: "empty name",
			request: &GroupCreateRequest{
				Name:     "",
				Priority: &validPriority,
			},
			expectError: true,
		},
		{
			name: "negative priority",
			request: &GroupCreateRequest{
				Name:     "test-group",
				Priority: &invalidPriority,
			},
			expectError: true,
		},
		{
			name: "name too long",
			request: &GroupCreateRequest{
				Name:     "this-is-a-very-long-group-name-that-exceeds-the-maximum-allowed-length-for-group-names-in-the-system",
				Priority: &validPriority,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
func TestSecurityConfigUpdateRequest_Validate(t *testing.T) {
	trueVal := true
	validThreshold := 5 * time.Minute
	validEndpointThreshold := 30 * time.Minute
	validMaxChanges := 10
	invalidThreshold := 30 * time.Second
	invalidMaxChanges := 0

	tests := []struct {
		name        string
		request     *SecurityConfigUpdateRequest
		expectError bool
	}{
		{
			name: "valid request - enabled only",
			request: &SecurityConfigUpdateRequest{
				Enabled: &trueVal,
			},
			expectError: false,
		},
		{
			name: "valid request - thresholds",
			request: &SecurityConfigUpdateRequest{
				SessionConflictThreshold: &validThreshold,
				EndpointChangeThreshold:  &validEndpointThreshold,
				MaxEndpointChangesPerDay: &validMaxChanges,
			},
			expectError: false,
		},
		{
			name: "session conflict threshold too low",
			request: &SecurityConfigUpdateRequest{
				SessionConflictThreshold: &invalidThreshold,
			},
			expectError: true,
		},
		{
			name: "endpoint change threshold too low",
			request: &SecurityConfigUpdateRequest{
				EndpointChangeThreshold: &invalidThreshold,
			},
			expectError: true,
		},
		{
			name: "max endpoint changes too low",
			request: &SecurityConfigUpdateRequest{
				MaxEndpointChangesPerDay: &invalidMaxChanges,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
