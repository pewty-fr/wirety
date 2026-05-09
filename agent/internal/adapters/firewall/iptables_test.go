package firewall

import (
	"os/exec"
	"strings"
	"testing"
	dom "wirety/agent/internal/domain/policy"
	"wirety/agent/internal/ports"
)

func TestNewAdapter(t *testing.T) {
	adapter := NewAdapter("wg0", []string{"eth0"})

	if adapter.iface != "wg0" {
		t.Errorf("Expected interface 'wg0', got '%s'", adapter.iface)
	}

	if len(adapter.natInterfaces) == 0 || adapter.natInterfaces[0] != "eth0" {
		t.Errorf("Expected NAT interface 'eth0', got %v", adapter.natInterfaces)
	}

	if adapter.httpPort != 3128 {
		t.Errorf("Expected HTTP port 3128, got %d", adapter.httpPort)
	}

	if adapter.httpsPort != 3129 {
		t.Errorf("Expected HTTPS port 3129, got %d", adapter.httpsPort)
	}
}

func TestNewAdapterWithEmptyNATInterface(t *testing.T) {
	adapter := NewAdapter("wg0", nil)

	if len(adapter.natInterfaces) != 0 {
		t.Errorf("Expected empty NAT interfaces, got %v", adapter.natInterfaces)
	}
}

func TestSetProxyPorts(t *testing.T) {
	adapter := NewAdapter("wg0", []string{"eth0"})

	adapter.SetProxyPorts(8080, 8443)

	if adapter.httpPort != 8080 {
		t.Errorf("Expected HTTP port 8080, got %d", adapter.httpPort)
	}

	if adapter.httpsPort != 8443 {
		t.Errorf("Expected HTTPS port 8443, got %d", adapter.httpsPort)
	}
}

func TestFallbackNATInterface(t *testing.T) {
	adapter := NewAdapter("wg0", nil)

	// This test will try to find a fallback interface
	// The result depends on the system, so we just test that it doesn't panic
	result := adapter.fallbackNATInterfaces()

	// Result can be empty if no suitable interface is found
	t.Logf("Fallback NAT interfaces: %v", result)
}

func TestGetNATInterfaces(t *testing.T) {
	// Test with configured NAT interface
	adapter := NewAdapter("wg0", []string{"eth0"})
	result := adapter.getNATInterfaces()

	if len(result) == 0 || result[0] != "eth0" {
		t.Errorf("Expected configured NAT interfaces ['eth0'], got %v", result)
	}

	// Test with auto-detection (empty NAT interface)
	adapter2 := NewAdapter("wg0", nil)
	result2 := adapter2.getNATInterfaces()

	// Result depends on system, just test that it doesn't panic
	t.Logf("Auto-detected NAT interfaces: %v", result2)
}

func TestToCheckArgs(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{[]string{"-A", "INPUT", "-j", "ACCEPT"}, []string{"-C", "INPUT", "-j", "ACCEPT"}},
		{[]string{"-I", "CHAIN", "1", "-j", "DROP"}, []string{"-C", "CHAIN", "-j", "DROP"}},
		{[]string{"-t", "nat", "-A", "POSTROUTING", "-j", "MASQUERADE"}, []string{"-t", "nat", "-C", "POSTROUTING", "-j", "MASQUERADE"}},
	}

	for _, tt := range tests {
		got := toCheckArgs(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("toCheckArgs(%v) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("toCheckArgs(%v)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestApplyIPTablesRule(t *testing.T) {
	adapter := NewAdapter("wg0", []string{"eth0"})

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
	adapter := NewAdapter("wg0", []string{"eth0"})

	// Test with nil policy - should not panic
	err := adapter.Sync(ports.SyncRequest{Policy: nil, SelfIP: "10.0.0.1"})
	if err != nil {
		t.Errorf("Sync with nil policy should not error, got: %v", err)
	}
}

func TestSyncWithEmptyPolicy(t *testing.T) {
	adapter := NewAdapter("wg0", []string{"eth0"})

	policy := &dom.JumpPolicy{
		IP:            "10.0.0.1",
		IPTablesRules: []string{},
	}

	// This will likely fail due to permissions, but we test that it doesn't panic
	err := adapter.Sync(ports.SyncRequest{Policy: policy, SelfIP: "10.0.0.1"})

	// We expect this to fail in test environment due to permissions
	// The important thing is that it doesn't panic
	t.Logf("Sync with empty policy returned: %v", err)
}

func TestSyncWithPolicyRules(t *testing.T) {
	adapter := NewAdapter("wg0", []string{"eth0"})

	policy := &dom.JumpPolicy{
		IP: "10.0.0.1",
		IPTablesRules: []string{
			"-A INPUT -j ACCEPT",
			"-A OUTPUT -j DROP",
		},
	}

	whitelistedIPs := []string{"10.0.0.2", "10.0.0.3"}

	// This will likely fail due to permissions, but we test that it doesn't panic
	err := adapter.Sync(ports.SyncRequest{Policy: policy, SelfIP: "10.0.0.1", AuthenticatedIPs: whitelistedIPs})

	// We expect this to fail in test environment due to permissions
	// The important thing is that it doesn't panic and handles the rules
	t.Logf("Sync with policy rules returned: %v", err)
}

func TestRun(t *testing.T) {
	adapter := NewAdapter("wg0", []string{"eth0"})

	// Test with invalid command - should fail
	err := adapter.run("--invalid-flag")
	if err == nil {
		t.Error("Expected error for invalid iptables command")
	}

	if !strings.Contains(err.Error(), "iptables") {
		t.Errorf("Expected error to mention iptables, got: %v", err)
	}
}

func TestRunIfNotExistsDoesNotPanic(t *testing.T) {
	adapter := NewAdapter("wg0", []string{"eth0"})

	// Test rule parsing does not panic with various argument forms
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "simple rule",
			args: []string{"-A", "INPUT", "-j", "ACCEPT"},
		},
		{
			name: "rule with table",
			args: []string{"-t", "nat", "-A", "POSTROUTING", "-j", "MASQUERADE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// In test environment this will fail due to permissions; we test it doesn't panic
			err := adapter.runIfNotExists(tt.args...)
			t.Logf("runIfNotExists(%v) = %v", tt.args, err)
		})
	}
}

func TestRunIfNotExists(t *testing.T) {
	adapter := NewAdapter("wg0", []string{"eth0"})

	// Test with a rule that doesn't exist (and will fail to create due to permissions)
	err := adapter.runIfNotExists("-A", "INPUT", "-j", "ACCEPT")

	// We expect this to fail in test environment due to permissions
	// The important thing is that it doesn't panic
	t.Logf("runIfNotExists returned: %v", err)
}

// Additional integration-style tests

func TestSyncIntegration(t *testing.T) {
	adapter := NewAdapter("wg0", []string{"eth0"})

	policy := &dom.JumpPolicy{
		IP: "10.0.0.1",
		IPTablesRules: []string{
			"-A INPUT -j ACCEPT",
			"-A OUTPUT -j DROP",
		},
	}

	// This will likely fail due to permissions, but we test the integration
	err := adapter.Sync(ports.SyncRequest{Policy: policy, SelfIP: "10.0.0.1", AuthenticatedIPs: []string{"10.0.0.2"}})

	// We expect this to fail in test environment due to permissions
	// The important thing is that it doesn't panic and handles the rules properly
	t.Logf("Sync integration test returned: %v", err)
}

func TestDetectNATInterfaces(t *testing.T) {
	adapter := NewAdapter("wg0", nil)

	// This test depends on the system configuration
	// We mainly test that it doesn't panic
	result := adapter.detectNATInterfaces()

	t.Logf("Detected NAT interfaces: %v", result)

	// The result can be empty if no default route is found
	// or if the parsing fails, which is acceptable in test environment
}

func TestEnableDebugLogging(t *testing.T) {
	adapter := NewAdapter("wg0", []string{"eth0"})

	// This will likely fail due to permissions, but should not panic
	err := adapter.EnableDebugLogging()

	// We expect this to fail in test environment due to permissions
	t.Logf("EnableDebugLogging returned: %v", err)
}
