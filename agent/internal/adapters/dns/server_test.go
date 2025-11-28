package dnsadapter

import (
	"testing"
	dom "wirety/agent/internal/domain/dns"
)

func TestLookupPeerIP_PeerDomain(t *testing.T) {
	peers := []dom.DNSPeer{
		{Name: "peer1", IP: "10.0.0.1"},
		{Name: "peer2", IP: "10.0.0.2"},
	}

	server := NewServer("mynetwork.internal", peers)

	// Test peer domain lookup
	ip := server.lookupPeerIP("peer1.mynetwork.internal")
	if ip != "10.0.0.1" {
		t.Errorf("Expected IP 10.0.0.1, got %s", ip)
	}

	ip = server.lookupPeerIP("peer2.mynetwork.internal")
	if ip != "10.0.0.2" {
		t.Errorf("Expected IP 10.0.0.2, got %s", ip)
	}

	// Test non-existent peer
	ip = server.lookupPeerIP("peer3.mynetwork.internal")
	if ip != "" {
		t.Errorf("Expected empty IP for non-existent peer, got %s", ip)
	}
}

func TestLookupPeerIP_RouteDomain(t *testing.T) {
	peers := []dom.DNSPeer{
		{Name: "peer1", IP: "10.0.0.1"},
		{Name: "server1.route1.internal", IP: "192.168.1.10"},
		{Name: "server2.route1.internal", IP: "192.168.1.11"},
		{Name: "db.route2.custom", IP: "172.16.0.5"},
	}

	server := NewServer("mynetwork.internal", peers)

	// Test peer domain lookup
	ip := server.lookupPeerIP("peer1.mynetwork.internal")
	if ip != "10.0.0.1" {
		t.Errorf("Expected IP 10.0.0.1, got %s", ip)
	}

	// Test route domain lookups with full FQDN
	ip = server.lookupPeerIP("server1.route1.internal")
	if ip != "192.168.1.10" {
		t.Errorf("Expected IP 192.168.1.10, got %s", ip)
	}

	ip = server.lookupPeerIP("server2.route1.internal")
	if ip != "192.168.1.11" {
		t.Errorf("Expected IP 192.168.1.11, got %s", ip)
	}

	// Test route domain with custom suffix
	ip = server.lookupPeerIP("db.route2.custom")
	if ip != "172.16.0.5" {
		t.Errorf("Expected IP 172.16.0.5, got %s", ip)
	}

	// Test non-existent route domain
	ip = server.lookupPeerIP("nonexistent.route1.internal")
	if ip != "" {
		t.Errorf("Expected empty IP for non-existent route domain, got %s", ip)
	}
}

func TestUpdate(t *testing.T) {
	peers := []dom.DNSPeer{
		{Name: "peer1", IP: "10.0.0.1"},
	}

	server := NewServer("mynetwork.internal", peers)

	// Verify initial state
	ip := server.lookupPeerIP("peer1.mynetwork.internal")
	if ip != "10.0.0.1" {
		t.Errorf("Expected IP 10.0.0.1, got %s", ip)
	}

	// Update with new peers and domain
	newPeers := []dom.DNSPeer{
		{Name: "peer2", IP: "10.0.0.2"},
		{Name: "server1.route1.internal", IP: "192.168.1.10"},
	}
	server.Update("newnetwork.internal", newPeers)

	// Old peer should not be found
	ip = server.lookupPeerIP("peer1.mynetwork.internal")
	if ip != "" {
		t.Errorf("Expected empty IP for old peer, got %s", ip)
	}

	// New peer should be found with new domain
	ip = server.lookupPeerIP("peer2.newnetwork.internal")
	if ip != "10.0.0.2" {
		t.Errorf("Expected IP 10.0.0.2, got %s", ip)
	}

	// Route domain should still work
	ip = server.lookupPeerIP("server1.route1.internal")
	if ip != "192.168.1.10" {
		t.Errorf("Expected IP 192.168.1.10, got %s", ip)
	}
}
