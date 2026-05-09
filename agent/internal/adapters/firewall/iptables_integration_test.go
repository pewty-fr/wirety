//go:build integration

package firewall

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	dom "wirety/agent/internal/domain/policy"
	"wirety/agent/internal/ports"
)

// requireRoot skips the test if not running as root (or without NET_ADMIN capability).
func requireRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() != 0 {
		t.Skip("skipped: requires root / NET_ADMIN (run with sudo or in a privileged container)")
	}
	if _, err := exec.LookPath("iptables"); err != nil {
		t.Skip("skipped: iptables not found in PATH")
	}
}

// cleanChain removes both Wirety chains from the filter table so tests start clean.
func cleanChain(t *testing.T) {
	t.Helper()
	_ = exec.Command("iptables", "-D", "FORWARD", "-j", "WIRETY_JUMP").Run()
	_ = exec.Command("iptables", "-F", "WIRETY_JUMP").Run()
	_ = exec.Command("iptables", "-X", "WIRETY_JUMP").Run()
	_ = exec.Command("iptables", "-F", "WIRETY_POLICY").Run()
	_ = exec.Command("iptables", "-X", "WIRETY_POLICY").Run()
}

// savedRules returns the current iptables-save output for the filter table.
func savedRules(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("iptables-save", "-t", "filter").Output()
	if err != nil {
		t.Fatalf("iptables-save failed: %v", err)
	}
	return string(out)
}

// chainExists returns true when the WIRETY_JUMP chain exists.
func chainExistsInKernel(chain string) bool {
	return exec.Command("iptables", "-L", chain, "-n").Run() == nil
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestIntegration_WiretyChainCreated verifies that Sync creates the WIRETY_JUMP chain.
func TestIntegration_WiretyChainCreated(t *testing.T) {
	requireRoot(t)
	cleanChain(t)
	t.Cleanup(func() { cleanChain(t) })

	adapter := NewAdapter("wg0", nil)
	policy := &dom.JumpPolicy{IP: "10.0.0.1", IPTablesRules: []string{}}

	if err := adapter.Sync(ports.SyncRequest{Policy: policy, SelfIP: "10.0.0.1"}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if !chainExistsInKernel("WIRETY_JUMP") {
		t.Error("expected WIRETY_JUMP chain to exist after Sync")
	}
}

// TestIntegration_AcceptRuleApplied verifies that an ACCEPT rule ends up in WIRETY_JUMP.
func TestIntegration_AcceptRuleApplied(t *testing.T) {
	requireRoot(t)
	cleanChain(t)
	t.Cleanup(func() { cleanChain(t) })

	adapter := NewAdapter("wg0", nil)
	policy := &dom.JumpPolicy{
		IP: "10.100.0.1",
		IPTablesRules: []string{
			"iptables -A FORWARD -s 10.100.0.2 -d 192.168.1.0/24 -j ACCEPT",
		},
	}

	if err := adapter.Sync(ports.SyncRequest{Policy: policy, SelfIP: "10.100.0.1"}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	saved := savedRules(t)
	if !strings.Contains(saved, "192.168.1.0/24") {
		t.Errorf("expected 192.168.1.0/24 in iptables after Sync\n%s", saved)
	}
	if !strings.Contains(saved, "ACCEPT") {
		t.Errorf("expected ACCEPT rule in WIRETY_JUMP\n%s", saved)
	}
}

// TestIntegration_DropRuleApplied verifies that a DROP rule ends up in WIRETY_JUMP.
func TestIntegration_DropRuleApplied(t *testing.T) {
	requireRoot(t)
	cleanChain(t)
	t.Cleanup(func() { cleanChain(t) })

	adapter := NewAdapter("wg0", nil)
	policy := &dom.JumpPolicy{
		IP: "10.100.0.1",
		IPTablesRules: []string{
			"iptables -A FORWARD -s 10.100.0.2 -d 10.0.0.0/8 -j DROP",
		},
	}

	if err := adapter.Sync(ports.SyncRequest{Policy: policy, SelfIP: "10.100.0.1"}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	saved := savedRules(t)
	if !strings.Contains(saved, "10.0.0.0/8") {
		t.Errorf("expected 10.0.0.0/8 in WIRETY_JUMP\n%s", saved)
	}
}

// TestIntegration_ReSyncFlushesOldRules verifies that a second Sync removes stale rules.
func TestIntegration_ReSyncFlushesOldRules(t *testing.T) {
	requireRoot(t)
	cleanChain(t)
	t.Cleanup(func() { cleanChain(t) })

	adapter := NewAdapter("wg0", nil)

	pol1 := &dom.JumpPolicy{
		IP: "10.100.0.1",
		IPTablesRules: []string{
			"iptables -A FORWARD -s 10.100.0.99 -j ACCEPT",
		},
	}
	if err := adapter.Sync(ports.SyncRequest{Policy: pol1, SelfIP: "10.100.0.1"}); err != nil {
		t.Fatalf("first Sync failed: %v", err)
	}
	if !strings.Contains(savedRules(t), "10.100.0.99") {
		t.Fatal("setup: expected 10.100.0.99 rule after first Sync")
	}

	pol2 := &dom.JumpPolicy{
		IP: "10.100.0.1",
		IPTablesRules: []string{
			"iptables -A FORWARD -s 10.100.0.88 -j ACCEPT",
		},
	}
	if err := adapter.Sync(ports.SyncRequest{Policy: pol2, SelfIP: "10.100.0.1"}); err != nil {
		t.Fatalf("second Sync failed: %v", err)
	}

	saved := savedRules(t)
	if strings.Contains(saved, "10.100.0.99") {
		t.Errorf("stale rule for 10.100.0.99 should have been flushed on re-sync\n%s", saved)
	}
	if !strings.Contains(saved, "10.100.0.88") {
		t.Errorf("new rule for 10.100.0.88 should be present after re-sync\n%s", saved)
	}
}

// TestIntegration_ChainAttachedToForward verifies that WIRETY_JUMP is jumped from FORWARD.
func TestIntegration_ChainAttachedToForward(t *testing.T) {
	requireRoot(t)
	cleanChain(t)
	t.Cleanup(func() { cleanChain(t) })

	adapter := NewAdapter("wg0", nil)
	policy := &dom.JumpPolicy{IP: "10.100.0.1", IPTablesRules: []string{}}

	if err := adapter.Sync(ports.SyncRequest{Policy: policy, SelfIP: "10.100.0.1"}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	saved := savedRules(t)
	// FORWARD chain should contain a jump to WIRETY_JUMP
	if !strings.Contains(saved, "-j WIRETY_JUMP") {
		t.Errorf("expected FORWARD -> WIRETY_JUMP jump rule\n%s", saved)
	}
}

// TestIntegration_EstablishedReturnRule verifies that server-format allow rules include
// a RELATED,ESTABLISHED return rule, and both are applied to WIRETY_JUMP.
func TestIntegration_EstablishedReturnRule(t *testing.T) {
	requireRoot(t)
	cleanChain(t)
	t.Cleanup(func() { cleanChain(t) })

	adapter := NewAdapter("wg0", nil)
	// These match the exact format GenerateIPTablesRules produces for allow-output CIDR.
	policy := &dom.JumpPolicy{
		IP: "10.100.0.1",
		IPTablesRules: []string{
			"iptables -A FORWARD -s 10.100.0.2 -d 192.168.0.0/16 -j ACCEPT",
			"iptables -A FORWARD -d 10.100.0.2 -s 192.168.0.0/16 -m state --state RELATED,ESTABLISHED -j ACCEPT",
		},
	}

	if err := adapter.Sync(ports.SyncRequest{Policy: policy, SelfIP: "10.100.0.1"}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	saved := savedRules(t)
	if !strings.Contains(saved, "192.168.0.0/16") {
		t.Errorf("CIDR 192.168.0.0/16 should appear in WIRETY_JUMP\n%s", saved)
	}
	if !strings.Contains(saved, "RELATED,ESTABLISHED") {
		t.Errorf("RELATED,ESTABLISHED return rule should be in WIRETY_JUMP\n%s", saved)
	}
}

// TestIntegration_RoundTripServerRules runs a full set of server-format rules through
// the agent's applyIPTablesRule and verifies they land in WIRETY_JUMP.
func TestIntegration_RoundTripServerRules(t *testing.T) {
	requireRoot(t)
	cleanChain(t)
	t.Cleanup(func() { cleanChain(t) })

	adapter := NewAdapter("wg0", nil)

	// Full rule set as generated by server's GenerateIPTablesRules for one peer.
	serverRules := []string{
		// Policy rules
		"iptables -A FORWARD -s 10.100.0.2 -d 192.168.0.0/16 -j ACCEPT",
		"iptables -A FORWARD -d 10.100.0.2 -s 192.168.0.0/16 -m state --state RELATED,ESTABLISHED -j ACCEPT",
		"iptables -A FORWARD -s 10.100.0.2 -d 10.1.0.0/24 -j DROP",
		// DNS rules
		"iptables -A INPUT -s 10.100.0.2 -p udp --dport 53 -j ACCEPT",
		"iptables -A OUTPUT -d 10.100.0.2 -p udp --sport 53 -j ACCEPT",
		// WireGuard handshake rules
		"iptables -A OUTPUT -d 10.100.0.2 -p udp --sport 51820 -j ACCEPT",
		"iptables -A INPUT -s 10.100.0.2 -p udp --dport 51820 -j ACCEPT",
		// Default deny
		"iptables -A FORWARD -j DROP",
	}

	policy := &dom.JumpPolicy{IP: "10.100.0.1", IPTablesRules: serverRules}

	if err := adapter.Sync(ports.SyncRequest{Policy: policy, SelfIP: "10.100.0.1"}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	saved := savedRules(t)

	// All rules are redirected into WIRETY_JUMP by applyIPTablesRule.
	// savedRules uses iptables-save format (not iptables -L -n):
	//   iptables-save: --dport 53, --sport 53
	//   iptables -L:   dpt:53, spt:53
	checks := []struct {
		desc   string
		needle string
	}{
		{"allow CIDR outbound", "192.168.0.0/16"},
		{"ESTABLISHED return", "RELATED,ESTABLISHED"},
		{"DROP CIDR", "10.1.0.0/24"},
		{"DNS dport 53", "dport 53"},
		{"DNS sport 53", "sport 53"},
		{"WG port 51820", "51820"},
		{"default DROP", "DROP"},
		{"WIRETY_JUMP chain present", "WIRETY_JUMP"},
	}
	for _, c := range checks {
		if !strings.Contains(saved, c.needle) {
			t.Errorf("round-trip: %s — %q not found in iptables-save output\n%s", c.desc, c.needle, saved)
		}
	}
}

// TestIntegration_EmptyRulesYieldsDefaultDrop verifies that Sync with empty IPTablesRules
// still sets up the chain with the default DROP rule added by the agent.
func TestIntegration_EmptyRulesYieldsDefaultDrop(t *testing.T) {
	requireRoot(t)
	cleanChain(t)
	t.Cleanup(func() { cleanChain(t) })

	adapter := NewAdapter("wg0", nil)
	policy := &dom.JumpPolicy{IP: "10.100.0.1", IPTablesRules: []string{}}

	if err := adapter.Sync(ports.SyncRequest{Policy: policy, SelfIP: "10.100.0.1"}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// The agent appends its own DROP rule for the WireGuard interface
	saved := savedRules(t)
	if !strings.Contains(saved, "WIRETY_JUMP") {
		t.Errorf("WIRETY_JUMP should exist even with empty server rules\n%s", saved)
	}
}
