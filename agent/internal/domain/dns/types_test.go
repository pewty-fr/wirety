package dns

import (
	"encoding/json"
	"testing"
)

func TestDNSPeer_JSONSerialization(t *testing.T) {
	peer := DNSPeer{
		Name: "test-peer",
		IP:   "10.0.0.1",
	}

	// Test JSON marshaling
	data, err := json.Marshal(peer)
	if err != nil {
		t.Errorf("Failed to marshal DNSPeer: %v", err)
	}

	expected := `{"name":"test-peer","ip":"10.0.0.1"}`
	if string(data) != expected {
		t.Errorf("Expected JSON %s, got %s", expected, string(data))
	}

	// Test JSON unmarshaling
	var unmarshaled DNSPeer
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal DNSPeer: %v", err)
	}

	if unmarshaled.Name != peer.Name {
		t.Errorf("Expected name %s, got %s", peer.Name, unmarshaled.Name)
	}

	if unmarshaled.IP != peer.IP {
		t.Errorf("Expected IP %s, got %s", peer.IP, unmarshaled.IP)
	}
}

func TestDNSConfig_JSONSerialization(t *testing.T) {
	config := DNSConfig{
		IP:     "10.0.0.1",
		Domain: "example.com",
		Peers: []DNSPeer{
			{Name: "peer1", IP: "10.0.0.2"},
			{Name: "peer2", IP: "10.0.0.3"},
		},
		UpstreamServers: []string{"8.8.8.8", "1.1.1.1"},
	}

	// Test JSON marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Errorf("Failed to marshal DNSConfig: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled DNSConfig
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal DNSConfig: %v", err)
	}

	if unmarshaled.IP != config.IP {
		t.Errorf("Expected IP %s, got %s", config.IP, unmarshaled.IP)
	}

	if unmarshaled.Domain != config.Domain {
		t.Errorf("Expected domain %s, got %s", config.Domain, unmarshaled.Domain)
	}

	if len(unmarshaled.Peers) != len(config.Peers) {
		t.Errorf("Expected %d peers, got %d", len(config.Peers), len(unmarshaled.Peers))
	}

	for i, peer := range unmarshaled.Peers {
		if peer.Name != config.Peers[i].Name {
			t.Errorf("Expected peer %d name %s, got %s", i, config.Peers[i].Name, peer.Name)
		}
		if peer.IP != config.Peers[i].IP {
			t.Errorf("Expected peer %d IP %s, got %s", i, config.Peers[i].IP, peer.IP)
		}
	}

	if len(unmarshaled.UpstreamServers) != len(config.UpstreamServers) {
		t.Errorf("Expected %d upstream servers, got %d", len(config.UpstreamServers), len(unmarshaled.UpstreamServers))
	}

	for i, server := range unmarshaled.UpstreamServers {
		if server != config.UpstreamServers[i] {
			t.Errorf("Expected upstream server %d %s, got %s", i, config.UpstreamServers[i], server)
		}
	}
}

func TestDNSConfig_EmptyPeers(t *testing.T) {
	config := DNSConfig{
		IP:              "10.0.0.1",
		Domain:          "example.com",
		Peers:           []DNSPeer{},
		UpstreamServers: []string{},
	}

	// Test JSON marshaling with empty slices
	data, err := json.Marshal(config)
	if err != nil {
		t.Errorf("Failed to marshal DNSConfig with empty peers: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled DNSConfig
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal DNSConfig with empty peers: %v", err)
	}

	if len(unmarshaled.Peers) != 0 {
		t.Errorf("Expected 0 peers, got %d", len(unmarshaled.Peers))
	}

	if len(unmarshaled.UpstreamServers) != 0 {
		t.Errorf("Expected 0 upstream servers, got %d", len(unmarshaled.UpstreamServers))
	}
}

func TestDNSPeer_EmptyValues(t *testing.T) {
	peer := DNSPeer{
		Name: "",
		IP:   "",
	}

	// Test JSON marshaling with empty values
	data, err := json.Marshal(peer)
	if err != nil {
		t.Errorf("Failed to marshal DNSPeer with empty values: %v", err)
	}

	expected := `{"name":"","ip":""}`
	if string(data) != expected {
		t.Errorf("Expected JSON %s, got %s", expected, string(data))
	}
}

func TestDNSConfig_NilSlices(t *testing.T) {
	config := DNSConfig{
		IP:              "10.0.0.1",
		Domain:          "example.com",
		Peers:           nil,
		UpstreamServers: nil,
	}

	// Test JSON marshaling with nil slices
	data, err := json.Marshal(config)
	if err != nil {
		t.Errorf("Failed to marshal DNSConfig with nil slices: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled DNSConfig
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal DNSConfig with nil slices: %v", err)
	}

	// nil slices should become empty slices after JSON round-trip
	if unmarshaled.Peers == nil {
		unmarshaled.Peers = []DNSPeer{}
	}
	if len(unmarshaled.Peers) != 0 {
		t.Errorf("Expected 0 peers, got %d", len(unmarshaled.Peers))
	}

	if unmarshaled.UpstreamServers == nil {
		unmarshaled.UpstreamServers = []string{}
	}
	if len(unmarshaled.UpstreamServers) != 0 {
		t.Errorf("Expected 0 upstream servers, got %d", len(unmarshaled.UpstreamServers))
	}
}
