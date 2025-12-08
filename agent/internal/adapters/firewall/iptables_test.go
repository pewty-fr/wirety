package firewall

import (
	"os/exec"
	"strings"
	"testing"
	dom "wirety/agent/internal/domain/policy"
)

func TestNewAdapter(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	if adapter.iface != "wg0" {
		t.Errorf("Expected interface 'wg0', got '%s'", adapter.iface)
	}

	if adapter.natInterface != "eth0" {
		t.Errorf("Expected NAT interface 'eth0', got '%s'", adapter.natInterface)
	}

	if adapter.httpPort != 3128 {
		t.Errorf("Expected HTTP port 3128, got %d", adapter.httpPort)
	}

	if adapter.httpsPort != 3129 {
		t.Errorf("Expected HTTPS port 3129, got %d", adapter.httpsPort)
	}
}

func TestNewAdapterWithEmptyNATInterface(t *testing.T) {
	adapter := NewAdapter("wg0", "")

	if adapter.natInterface != "" {
		t.Errorf("Expected empty NAT interface, got '%s'", adapter.natInterface)
	}
}

func TestSetProxyPorts(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	adapter.SetProxyPorts(8080, 8443)

	if adapter.httpPort != 8080 {
		t.Errorf("Expected HTTP port 8080, got %d", adapter.httpPort)
	}

	if adapter.httpsPort != 8443 {
		t.Errorf("Expected HTTPS port 8443, got %d", adapter.httpsPort)
	}
}

func TestFallbackNATInterface(t *testing.T) {
	adapter := NewAdapter("wg0", "")

	// This test will try to find a fallback interface
	// The result depends on the system, so we just test that it doesn't panic
	result := adapter.fallbackNATInterface()

	// Result can be empty string if no suitable interface is found
	t.Logf("Fallback NAT interface: '%s'", result)
}

func TestGetNATInterface(t *testing.T) {
	// Test with configured NAT interface
	adapter := NewAdapter("wg0", "eth0")
	result := adapter.getNATInterface()

	if result != "eth0" {
		t.Errorf("Expected configured NAT interface 'eth0', got '%s'", result)
	}

	// Test with auto-detection (empty NAT interface)
	adapter2 := NewAdapter("wg0", "")
	result2 := adapter2.getNATInterface()

	// Result depends on system, just test that it doesn't panic
	t.Logf("Auto-detected NAT interface: '%s'", result2)
}

func TestContainsSubstring(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "hello", true},
		{"hello world", "world", true},
		{"hello world", "lo wo", true},
		{"hello world", "xyz", false},
		{"hello", "hello world", false},
		{"", "test", false},
		{"test", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		got := containsSubstring(tt.s, tt.substr)
		if got != tt.want {
			t.Errorf("containsSubstring(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
		}
	}
}

func TestApplyIPTablesRule(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	tests := []struct {
		name     string
		chain    string
		rule     string
		wantErr  bool
		skipTest bool // Skip test if iptables not available
	}{
		{
			name:    "simple rule",
			chain:   "TEST_CHAIN",
			rule:    "-j ACCEPT",
			wantErr: true, // Will fail because we don't have permissions
		},
		{
			name:    "rule with iptables prefix",
			chain:   "TEST_CHAIN",
			rule:    "iptables -A INPUT -j ACCEPT",
			wantErr: true, // Will fail because we don't have permissions
		},
		{
			name:    "empty rule",
			chain:   "TEST_CHAIN",
			rule:    "",
			wantErr: true,
		},
	}

	// Check if iptables is available
	if _, err := exec.LookPath("iptables"); err != nil {
		t.Skip("iptables not available, skipping iptables rule tests")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTest {
				t.Skip("Test skipped")
			}

			err := adapter.applyIPTablesRule(tt.chain, tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("applyIPTablesRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSyncWithNilPolicy(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	// Test with nil policy - should not panic
	err := adapter.Sync(nil, "10.0.0.1", []string{})
	if err != nil {
		t.Errorf("Sync with nil policy should not error, got: %v", err)
	}
}

func TestSyncWithEmptyPolicy(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	policy := &dom.JumpPolicy{
		IP:            "10.0.0.1",
		IPTablesRules: []string{},
	}

	// This will likely fail due to permissions, but we test that it doesn't panic
	err := adapter.Sync(policy, "10.0.0.1", []string{})

	// We expect this to fail in test environment due to permissions
	// The important thing is that it doesn't panic
	t.Logf("Sync with empty policy returned: %v", err)
}

func TestSyncWithPolicyRules(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	policy := &dom.JumpPolicy{
		IP: "10.0.0.1",
		IPTablesRules: []string{
			"-A INPUT -j ACCEPT",
			"-A OUTPUT -j DROP",
		},
	}

	whitelistedIPs := []string{"10.0.0.2", "10.0.0.3"}

	// This will likely fail due to permissions, but we test that it doesn't panic
	err := adapter.Sync(policy, "10.0.0.1", whitelistedIPs)

	// We expect this to fail in test environment due to permissions
	// The important thing is that it doesn't panic and handles the rules
	t.Logf("Sync with policy rules returned: %v", err)
}

func TestRun(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	// Test with invalid command - should fail
	err := adapter.run("--invalid-flag")
	if err == nil {
		t.Error("Expected error for invalid iptables command")
	}

	if !strings.Contains(err.Error(), "iptables") {
		t.Errorf("Expected error to mention iptables, got: %v", err)
	}
}

func TestRuleExists(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	// Test rule parsing
	tests := []struct {
		name string
		args []string
		want bool // We can't really test the actual existence without root
	}{
		{
			name: "simple rule",
			args: []string{"-A", "INPUT", "-j", "ACCEPT"},
			want: false, // Will be false in test environment
		},
		{
			name: "rule with table",
			args: []string{"-t", "nat", "-A", "POSTROUTING", "-j", "MASQUERADE"},
			want: false, // Will be false in test environment
		},
		{
			name: "invalid rule",
			args: []string{"-X", "INVALID"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.ruleExists(tt.args...)
			// In test environment, this will always be false
			// We're mainly testing that it doesn't panic
			t.Logf("ruleExists(%v) = %v", tt.args, got)
		})
	}
}

func TestRunIfNotExists(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	// Test with a rule that doesn't exist (and will fail to create due to permissions)
	err := adapter.runIfNotExists("-A", "INPUT", "-j", "ACCEPT")

	// We expect this to fail in test environment due to permissions
	// The important thing is that it doesn't panic
	t.Logf("runIfNotExists returned: %v", err)
}

// Additional integration-style tests

func TestSyncIntegration(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	policy := &dom.JumpPolicy{
		IP: "10.0.0.1",
		IPTablesRules: []string{
			"-A INPUT -j ACCEPT",
			"-A OUTPUT -j DROP",
		},
	}

	// This will likely fail due to permissions, but we test the integration
	err := adapter.Sync(policy, "10.0.0.1", []string{"10.0.0.2"})

	// We expect this to fail in test environment due to permissions
	// The important thing is that it doesn't panic and handles the rules properly
	t.Logf("Sync integration test returned: %v", err)
}

func TestDetectDefaultNATInterface(t *testing.T) {
	adapter := NewAdapter("wg0", "")

	// This test depends on the system configuration
	// We mainly test that it doesn't panic
	result := adapter.detectDefaultNATInterface()

	t.Logf("Detected default NAT interface: '%s'", result)

	// The result can be empty if no default route is found
	// or if the parsing fails, which is acceptable in test environment
}

func TestEnableDebugLogging(t *testing.T) {
	adapter := NewAdapter("wg0", "eth0")

	// This will likely fail due to permissions, but should not panic
	err := adapter.EnableDebugLogging()

	// We expect this to fail in test environment due to permissions
	t.Logf("EnableDebugLogging returned: %v", err)
}
