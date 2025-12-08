package network

import (
	"testing"
	"time"
)

func TestDNSMapping_GetFQDN(t *testing.T) {
	tests := []struct {
		name         string
		dnsMapping   *DNSMapping
		route        *Route
		expectedFQDN string
	}{
		{
			name: "with custom domain suffix",
			dnsMapping: &DNSMapping{
				ID:        "mapping1",
				RouteID:   "route1",
				Name:      "api",
				IPAddress: "192.168.1.10",
			},
			route: &Route{
				ID:           "route1",
				Name:         "backend",
				DomainSuffix: "example.com",
			},
			expectedFQDN: "api.backend.example.com",
		},
		{
			name: "with empty domain suffix (should use default)",
			dnsMapping: &DNSMapping{
				ID:        "mapping1",
				RouteID:   "route1",
				Name:      "database",
				IPAddress: "192.168.1.20",
			},
			route: &Route{
				ID:           "route1",
				Name:         "storage",
				DomainSuffix: "",
			},
			expectedFQDN: "database.storage.internal",
		},
		{
			name: "with complex names",
			dnsMapping: &DNSMapping{
				ID:        "mapping1",
				RouteID:   "route1",
				Name:      "web-server-01",
				IPAddress: "10.0.0.100",
			},
			route: &Route{
				ID:           "route1",
				Name:         "production-web",
				DomainSuffix: "corp.local",
			},
			expectedFQDN: "web-server-01.production-web.corp.local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fqdn := tt.dnsMapping.GetFQDN(tt.route)
			if fqdn != tt.expectedFQDN {
				t.Errorf("Expected FQDN %s, got %s", tt.expectedFQDN, fqdn)
			}
		})
	}
}

func TestValidateDNSName(t *testing.T) {
	tests := []struct {
		name        string
		dnsName     string
		expectError bool
	}{
		{
			name:        "valid simple name",
			dnsName:     "server1",
			expectError: false,
		},
		{
			name:        "valid name with hyphens",
			dnsName:     "web-server-01",
			expectError: false,
		},
		{
			name:        "valid name with numbers",
			dnsName:     "api2",
			expectError: false,
		},
		{
			name:        "empty name",
			dnsName:     "",
			expectError: true,
		},
		{
			name:        "name with spaces",
			dnsName:     "web server",
			expectError: true,
		},
		{
			name:        "name with newline",
			dnsName:     "server\n1",
			expectError: true,
		},
		{
			name:        "name with tab",
			dnsName:     "server\t1",
			expectError: true,
		},
		{
			name:        "name with carriage return",
			dnsName:     "server\r1",
			expectError: true,
		},
		{
			name:        "very long name",
			dnsName:     "this-is-a-very-long-dns-name-that-exceeds-the-maximum-allowed-length-for-dns-names-in-the-system-and-should-be-rejected-because-it-is-way-too-long-for-any-reasonable-use-case-and-would-cause-problems-in-the-dns-resolution-process",
			expectError: true,
		},
		{
			name:        "name at maximum length boundary",
			dnsName:     "a" + string(make([]byte, 254)), // 255 characters total
			expectError: true,                            // Should be rejected as it's exactly 255 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDNSName(tt.dnsName)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// Test the domain errors
func TestDomainErrors(t *testing.T) {
	// Test that our domain errors are properly defined
	if ErrNetworkNotFound == nil {
		t.Error("ErrNetworkNotFound should be defined")
	}
	if ErrPeerNotFound == nil {
		t.Error("ErrPeerNotFound should be defined")
	}
	if ErrRouteNotFound == nil {
		t.Error("ErrRouteNotFound should be defined")
	}
	if ErrGroupNotFound == nil {
		t.Error("ErrGroupNotFound should be defined")
	}
	if ErrPolicyNotFound == nil {
		t.Error("ErrPolicyNotFound should be defined")
	}
	if ErrDNSMappingNotFound == nil {
		t.Error("ErrDNSMappingNotFound should be defined")
	}
}

// Test DNS mapping struct creation and basic properties
func TestDNSMapping_BasicProperties(t *testing.T) {
	now := time.Now()
	mapping := &DNSMapping{
		ID:        "test-id",
		RouteID:   "route-id",
		Name:      "test-server",
		IPAddress: "192.168.1.100",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if mapping.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", mapping.ID)
	}
	if mapping.RouteID != "route-id" {
		t.Errorf("Expected RouteID 'route-id', got %s", mapping.RouteID)
	}
	if mapping.Name != "test-server" {
		t.Errorf("Expected Name 'test-server', got %s", mapping.Name)
	}
	if mapping.IPAddress != "192.168.1.100" {
		t.Errorf("Expected IPAddress '192.168.1.100', got %s", mapping.IPAddress)
	}
	if !mapping.CreatedAt.Equal(now) {
		t.Errorf("Expected CreatedAt to be %v, got %v", now, mapping.CreatedAt)
	}
	if !mapping.UpdatedAt.Equal(now) {
		t.Errorf("Expected UpdatedAt to be %v, got %v", now, mapping.UpdatedAt)
	}
}

// Test request structs
func TestDNSMappingRequests(t *testing.T) {
	// Test create request
	createReq := &DNSMappingCreateRequest{
		Name:      "api-server",
		IPAddress: "10.0.0.50",
	}

	if createReq.Name != "api-server" {
		t.Errorf("Expected Name 'api-server', got %s", createReq.Name)
	}
	if createReq.IPAddress != "10.0.0.50" {
		t.Errorf("Expected IPAddress '10.0.0.50', got %s", createReq.IPAddress)
	}

	// Test update request
	updateReq := &DNSMappingUpdateRequest{
		Name:      "updated-server",
		IPAddress: "10.0.0.51",
	}

	if updateReq.Name != "updated-server" {
		t.Errorf("Expected Name 'updated-server', got %s", updateReq.Name)
	}
	if updateReq.IPAddress != "10.0.0.51" {
		t.Errorf("Expected IPAddress '10.0.0.51', got %s", updateReq.IPAddress)
	}

	// Test partial update request
	partialReq := &DNSMappingUpdateRequest{
		Name: "only-name-updated",
	}

	if partialReq.Name != "only-name-updated" {
		t.Errorf("Expected Name 'only-name-updated', got %s", partialReq.Name)
	}
	if partialReq.IPAddress != "" {
		t.Errorf("Expected empty IPAddress, got %s", partialReq.IPAddress)
	}
}
