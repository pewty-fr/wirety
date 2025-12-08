package agent

import (
	"os"
	"testing"
)

func TestCollectSystemInfo(t *testing.T) {
	sysInfo, err := CollectSystemInfo("wg0")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if sysInfo == nil {
		t.Fatal("Expected SystemInfo to be non-nil")
	}

	// Hostname should not be empty (fallback to "unknown" if needed)
	if sysInfo.Hostname == "" {
		t.Error("Expected hostname to be set")
	}

	// System uptime should be non-negative
	if sysInfo.SystemUptime < 0 {
		t.Errorf("Expected non-negative system uptime, got %d", sysInfo.SystemUptime)
	}

	// WireGuard uptime should be non-negative
	if sysInfo.WireGuardUptime < 0 {
		t.Errorf("Expected non-negative WireGuard uptime, got %d", sysInfo.WireGuardUptime)
	}

	// PeerEndpoints should be initialized (can be empty)
	if sysInfo.PeerEndpoints == nil {
		t.Error("Expected PeerEndpoints map to be initialized")
	}
}

func TestCollectSystemInfoWithNonExistentInterface(t *testing.T) {
	sysInfo, err := CollectSystemInfo("nonexistent-interface")
	if err != nil {
		t.Errorf("Expected no error even with non-existent interface, got: %v", err)
	}

	if sysInfo == nil {
		t.Fatal("Expected SystemInfo to be non-nil")
	}

	// Should still collect basic system info
	if sysInfo.Hostname == "" {
		t.Error("Expected hostname to be set even with non-existent interface")
	}

	// WireGuard uptime should be 0 for non-existent interface
	if sysInfo.WireGuardUptime != 0 {
		t.Errorf("Expected WireGuard uptime 0 for non-existent interface, got %d", sysInfo.WireGuardUptime)
	}

	// PeerEndpoints should be empty for non-existent interface
	if len(sysInfo.PeerEndpoints) != 0 {
		t.Errorf("Expected 0 peer endpoints for non-existent interface, got %d", len(sysInfo.PeerEndpoints))
	}
}

func TestGetSystemUptime(t *testing.T) {
	uptime := getSystemUptime()

	// Uptime should be non-negative
	if uptime < 0 {
		t.Errorf("Expected non-negative uptime, got %d", uptime)
	}

	// On most systems, uptime should be greater than 0
	// (unless the system just booted, but that's unlikely in tests)
	// We'll just check it's reasonable (less than 10 years in seconds)
	maxReasonableUptime := int64(10 * 365 * 24 * 60 * 60) // 10 years in seconds
	if uptime > maxReasonableUptime {
		t.Errorf("Uptime seems unreasonably high: %d seconds", uptime)
	}
}

func TestGetWireGuardUptime(t *testing.T) {
	// Test with non-existent interface
	uptime := getWireGuardUptime("nonexistent-interface")
	if uptime != 0 {
		t.Errorf("Expected 0 uptime for non-existent interface, got %d", uptime)
	}

	// Test with common interface names that might exist
	commonInterfaces := []string{"lo", "eth0", "wlan0", "en0"}
	for _, iface := range commonInterfaces {
		uptime := getWireGuardUptime(iface)
		// Should not panic and should return non-negative value
		if uptime < 0 {
			t.Errorf("Expected non-negative uptime for interface %s, got %d", iface, uptime)
		}
	}
}

func TestGetWireGuardEndpoints(t *testing.T) {
	// Test with non-existent interface
	endpoints := getWireGuardEndpoints("nonexistent-interface")
	if endpoints == nil {
		t.Error("Expected endpoints map to be initialized")
	}

	if len(endpoints) != 0 {
		t.Errorf("Expected 0 endpoints for non-existent interface, got %d", len(endpoints))
	}

	// Test with empty interface name
	endpoints = getWireGuardEndpoints("")
	if endpoints == nil {
		t.Error("Expected endpoints map to be initialized for empty interface")
	}

	// Test with common WireGuard interface names
	wgInterfaces := []string{"wg0", "wg1", "wireguard"}
	for _, iface := range wgInterfaces {
		endpoints := getWireGuardEndpoints(iface)
		if endpoints == nil {
			t.Errorf("Expected endpoints map to be initialized for interface %s", iface)
		}

		// Should not contain (none) endpoints
		for pubKey, endpoint := range endpoints {
			if endpoint == "(none)" {
				t.Errorf("Expected (none) endpoints to be filtered out, found for key %s", pubKey)
			}

			// Public keys should be non-empty
			if pubKey == "" {
				t.Error("Expected public key to be non-empty")
			}

			// Endpoints should be non-empty
			if endpoint == "" {
				t.Error("Expected endpoint to be non-empty")
			}
		}
	}
}

func TestSystemInfoFields(t *testing.T) {
	// Test SystemInfo struct fields
	sysInfo := &SystemInfo{
		Hostname:        "test-host",
		SystemUptime:    12345,
		WireGuardUptime: 6789,
		PeerEndpoints: map[string]string{
			"pubkey1": "192.168.1.1:51820",
			"pubkey2": "10.0.0.1:51820",
		},
	}

	if sysInfo.Hostname != "test-host" {
		t.Errorf("Expected hostname 'test-host', got '%s'", sysInfo.Hostname)
	}

	if sysInfo.SystemUptime != 12345 {
		t.Errorf("Expected system uptime 12345, got %d", sysInfo.SystemUptime)
	}

	if sysInfo.WireGuardUptime != 6789 {
		t.Errorf("Expected WireGuard uptime 6789, got %d", sysInfo.WireGuardUptime)
	}

	if len(sysInfo.PeerEndpoints) != 2 {
		t.Errorf("Expected 2 peer endpoints, got %d", len(sysInfo.PeerEndpoints))
	}

	expectedEndpoints := map[string]string{
		"pubkey1": "192.168.1.1:51820",
		"pubkey2": "10.0.0.1:51820",
	}

	for key, expectedEndpoint := range expectedEndpoints {
		if actualEndpoint, exists := sysInfo.PeerEndpoints[key]; !exists {
			t.Errorf("Expected peer endpoint for key %s to exist", key)
		} else if actualEndpoint != expectedEndpoint {
			t.Errorf("Expected endpoint '%s' for key %s, got '%s'", expectedEndpoint, key, actualEndpoint)
		}
	}
}

func TestHostnameHandling(t *testing.T) {
	// Save original hostname
	originalHostname, _ := os.Hostname()

	// Test with actual hostname
	sysInfo, err := CollectSystemInfo("wg0")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Hostname should either be the actual hostname or "unknown"
	if sysInfo.Hostname != originalHostname && sysInfo.Hostname != "unknown" {
		t.Errorf("Expected hostname to be '%s' or 'unknown', got '%s'", originalHostname, sysInfo.Hostname)
	}

	// Hostname should never be empty
	if sysInfo.Hostname == "" {
		t.Error("Expected hostname to never be empty")
	}
}

func TestUptimeReasonableness(t *testing.T) {
	sysInfo, err := CollectSystemInfo("wg0")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// System uptime should be reasonable (less than 1 year for most test systems)
	maxReasonableUptime := int64(365 * 24 * 60 * 60) // 1 year in seconds
	if sysInfo.SystemUptime > maxReasonableUptime {
		t.Logf("Warning: System uptime seems very high: %d seconds", sysInfo.SystemUptime)
	}

	// WireGuard uptime should not exceed system uptime
	if sysInfo.WireGuardUptime > sysInfo.SystemUptime {
		t.Logf("Warning: WireGuard uptime (%d) exceeds system uptime (%d)", sysInfo.WireGuardUptime, sysInfo.SystemUptime)
	}
}

func TestPeerEndpointsFormat(t *testing.T) {
	sysInfo, err := CollectSystemInfo("wg0")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Check that peer endpoints have reasonable format
	for pubKey, endpoint := range sysInfo.PeerEndpoints {
		// Public key should be reasonable length (WireGuard keys are base64 encoded, ~44 chars)
		if len(pubKey) < 10 {
			t.Errorf("Public key seems too short: '%s'", pubKey)
		}

		// Endpoint should contain a colon (IP:port format)
		if !containsChar(endpoint, ':') {
			t.Errorf("Endpoint should contain port separator ':': '%s'", endpoint)
		}

		// Endpoint should not be empty or "(none)"
		if endpoint == "" || endpoint == "(none)" {
			t.Errorf("Endpoint should not be empty or '(none)': '%s'", endpoint)
		}
	}
}

// Helper function to check if string contains character
func containsChar(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}
