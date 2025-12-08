package policy

import (
	"encoding/json"
	"testing"
)

func TestJumpPolicy_JSONSerialization(t *testing.T) {
	policy := JumpPolicy{
		IP: "10.0.0.1",
		IPTablesRules: []string{
			"-A INPUT -s 10.0.0.2 -j ACCEPT",
			"-A OUTPUT -d 10.0.0.3 -j DROP",
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(policy)
	if err != nil {
		t.Errorf("Failed to marshal JumpPolicy: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled JumpPolicy
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal JumpPolicy: %v", err)
	}

	if unmarshaled.IP != policy.IP {
		t.Errorf("Expected IP %s, got %s", policy.IP, unmarshaled.IP)
	}

	if len(unmarshaled.IPTablesRules) != len(policy.IPTablesRules) {
		t.Errorf("Expected %d iptables rules, got %d", len(policy.IPTablesRules), len(unmarshaled.IPTablesRules))
	}

	for i, rule := range unmarshaled.IPTablesRules {
		if rule != policy.IPTablesRules[i] {
			t.Errorf("Expected rule %d %s, got %s", i, policy.IPTablesRules[i], rule)
		}
	}
}

func TestJumpPolicy_EmptyRules(t *testing.T) {
	policy := JumpPolicy{
		IP:            "10.0.0.1",
		IPTablesRules: []string{},
	}

	// Test JSON marshaling with empty rules
	data, err := json.Marshal(policy)
	if err != nil {
		t.Errorf("Failed to marshal JumpPolicy with empty rules: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled JumpPolicy
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal JumpPolicy with empty rules: %v", err)
	}

	if len(unmarshaled.IPTablesRules) != 0 {
		t.Errorf("Expected 0 iptables rules, got %d", len(unmarshaled.IPTablesRules))
	}
}

func TestJumpPolicy_NilRules(t *testing.T) {
	policy := JumpPolicy{
		IP:            "10.0.0.1",
		IPTablesRules: nil,
	}

	// Test JSON marshaling with nil rules
	data, err := json.Marshal(policy)
	if err != nil {
		t.Errorf("Failed to marshal JumpPolicy with nil rules: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled JumpPolicy
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal JumpPolicy with nil rules: %v", err)
	}

	// nil slice should become empty slice after JSON round-trip
	if unmarshaled.IPTablesRules == nil {
		unmarshaled.IPTablesRules = []string{}
	}
	if len(unmarshaled.IPTablesRules) != 0 {
		t.Errorf("Expected 0 iptables rules, got %d", len(unmarshaled.IPTablesRules))
	}
}

func TestJumpPolicy_EmptyIP(t *testing.T) {
	policy := JumpPolicy{
		IP:            "",
		IPTablesRules: []string{"rule1", "rule2"},
	}

	// Test JSON marshaling with empty IP
	data, err := json.Marshal(policy)
	if err != nil {
		t.Errorf("Failed to marshal JumpPolicy with empty IP: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled JumpPolicy
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal JumpPolicy with empty IP: %v", err)
	}

	if unmarshaled.IP != "" {
		t.Errorf("Expected empty IP, got %s", unmarshaled.IP)
	}
}

func TestJumpPolicy_ComplexRules(t *testing.T) {
	policy := JumpPolicy{
		IP: "192.168.1.100",
		IPTablesRules: []string{
			"-A INPUT -p tcp --dport 22 -j ACCEPT",
			"-A INPUT -p udp --dport 53 -j ACCEPT",
			"-A FORWARD -i wg0 -o eth0 -j ACCEPT",
			"-A FORWARD -i eth0 -o wg0 -m state --state RELATED,ESTABLISHED -j ACCEPT",
			"-A POSTROUTING -t nat -o eth0 -j MASQUERADE",
		},
	}

	// Test JSON marshaling with complex rules
	data, err := json.Marshal(policy)
	if err != nil {
		t.Errorf("Failed to marshal JumpPolicy with complex rules: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled JumpPolicy
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal JumpPolicy with complex rules: %v", err)
	}

	if len(unmarshaled.IPTablesRules) != len(policy.IPTablesRules) {
		t.Errorf("Expected %d iptables rules, got %d", len(policy.IPTablesRules), len(unmarshaled.IPTablesRules))
	}

	for i, rule := range unmarshaled.IPTablesRules {
		if rule != policy.IPTablesRules[i] {
			t.Errorf("Expected rule %d %s, got %s", i, policy.IPTablesRules[i], rule)
		}
	}
}
