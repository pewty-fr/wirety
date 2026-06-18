package dnsadapter

import (
	"net"
	"testing"
	dom "wirety/agent/internal/domain/dns"

	"github.com/miekg/dns"
)

func TestNewServer(t *testing.T) {
	domain := "example.com"
	peers := []dom.DNSPeer{
		{Name: "peer1", IP: "10.0.0.1"},
		{Name: "peer2", IP: "10.0.0.2"},
	}

	server := NewServer(domain, peers)

	if server.domain != domain {
		t.Errorf("Expected domain '%s', got '%s'", domain, server.domain)
	}

	if len(server.peers) != len(peers) {
		t.Errorf("Expected %d peers, got %d", len(peers), len(server.peers))
	}

	for i, peer := range server.peers {
		if peer.Name != peers[i].Name {
			t.Errorf("Expected peer %d name '%s', got '%s'", i, peers[i].Name, peer.Name)
		}
		if peer.IP != peers[i].IP {
			t.Errorf("Expected peer %d IP '%s', got '%s'", i, peers[i].IP, peer.IP)
		}
	}

	// Check default upstream servers
	if len(server.upstreamServers) != 2 {
		t.Errorf("Expected 2 default upstream servers, got %d", len(server.upstreamServers))
	}

	expectedUpstreams := []string{"8.8.8.8:53", "1.1.1.1:53"}
	for i, expected := range expectedUpstreams {
		if i < len(server.upstreamServers) && server.upstreamServers[i] != expected {
			t.Errorf("Expected upstream server %d '%s', got '%s'", i, expected, server.upstreamServers[i])
		}
	}
}

func TestSetUpstreamServers(t *testing.T) {
	server := NewServer("example.com", []dom.DNSPeer{})

	// Test with servers without port
	servers := []string{"8.8.8.8", "1.1.1.1", "9.9.9.9"}
	server.SetUpstreamServers(servers)

	expectedServers := []string{"8.8.8.8:53", "1.1.1.1:53", "9.9.9.9:53"}
	if len(server.upstreamServers) != len(expectedServers) {
		t.Errorf("Expected %d upstream servers, got %d", len(expectedServers), len(server.upstreamServers))
	}

	for i, expected := range expectedServers {
		if i < len(server.upstreamServers) && server.upstreamServers[i] != expected {
			t.Errorf("Expected upstream server %d '%s', got '%s'", i, expected, server.upstreamServers[i])
		}
	}

	// Test with servers that already have port
	serversWithPort := []string{"8.8.8.8:53", "1.1.1.1:5353"}
	server.SetUpstreamServers(serversWithPort)

	if len(server.upstreamServers) != len(serversWithPort) {
		t.Errorf("Expected %d upstream servers, got %d", len(serversWithPort), len(server.upstreamServers))
	}

	for i, expected := range serversWithPort {
		if i < len(server.upstreamServers) && server.upstreamServers[i] != expected {
			t.Errorf("Expected upstream server %d '%s', got '%s'", i, expected, server.upstreamServers[i])
		}
	}
}

func TestLookupPeerIP(t *testing.T) {
	domain := "example.com"
	peers := []dom.DNSPeer{
		{Name: "peer1", IP: "10.0.0.1"},
		{Name: "peer2", IP: "10.0.0.2"},
		{Name: "route1.vpn.example.org", IP: "10.0.1.1"}, // Route DNS mapping with full FQDN
	}

	server := NewServer(domain, peers)

	// Test peer DNS lookups
	tests := []struct {
		name     string
		expected string
	}{
		{"peer1.example.com", "10.0.0.1"},
		{"peer2.example.com", "10.0.0.2"},
		{"nonexistent.example.com", ""},
		{"peer1.wrong.com", ""},
		{"route1.vpn.example.org", "10.0.1.1"}, // Route DNS mapping
		{"wrong.vpn.example.org", ""},
	}

	for _, tt := range tests {
		result := server.lookupPeerIP(tt.name)
		if result != tt.expected {
			t.Errorf("lookupPeerIP(%q) = %q, want %q", tt.name, result, tt.expected)
		}
	}
}

func TestUpdate(t *testing.T) {
	server := NewServer("old.com", []dom.DNSPeer{})

	newDomain := "new.com"
	newPeers := []dom.DNSPeer{
		{Name: "newpeer1", IP: "192.168.1.1"},
		{Name: "newpeer2", IP: "192.168.1.2"},
	}

	server.Update(newDomain, newPeers)

	if server.domain != newDomain {
		t.Errorf("Expected domain '%s', got '%s'", newDomain, server.domain)
	}

	if len(server.peers) != len(newPeers) {
		t.Errorf("Expected %d peers, got %d", len(newPeers), len(server.peers))
	}

	for i, peer := range server.peers {
		if peer.Name != newPeers[i].Name {
			t.Errorf("Expected peer %d name '%s', got '%s'", i, newPeers[i].Name, peer.Name)
		}
		if peer.IP != newPeers[i].IP {
			t.Errorf("Expected peer %d IP '%s', got '%s'", i, newPeers[i].IP, peer.IP)
		}
	}
}

func TestHandleDNSWithLocalRecords(t *testing.T) {
	domain := "test.com"
	peers := []dom.DNSPeer{
		{Name: "peer1", IP: "10.0.0.1"},
		{Name: "peer2", IP: "10.0.0.2"},
	}

	server := NewServer(domain, peers)

	// Create DNS query for local record
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("peer1.test.com"), dns.TypeA)

	// Create mock response writer
	mockWriter := &mockResponseWriter{}

	server.handleDNS(mockWriter, m)

	// Verify response was written
	if mockWriter.msg == nil {
		t.Fatal("Expected response message to be written")
	}

	// Verify response contains answer
	if len(mockWriter.msg.Answer) != 1 {
		t.Errorf("Expected 1 answer, got %d", len(mockWriter.msg.Answer))
	}

	// Verify answer is correct A record
	if aRecord, ok := mockWriter.msg.Answer[0].(*dns.A); ok {
		expectedIP := net.ParseIP("10.0.0.1")
		if !aRecord.A.Equal(expectedIP) {
			t.Errorf("Expected IP %s, got %s", expectedIP, aRecord.A)
		}
	} else {
		t.Error("Expected A record in answer")
	}
}

// TestProbeDomainInterceptionGatedOnAuth locks in the fix for the HSTS bug:
// well-known captive-portal probe hosts (which include real, HSTS-preloaded
// sites like www.apple.com) must be redirected to the portal ONLY while the
// peer is unauthenticated. Once authenticated they must resolve normally, or
// browsing to apple.com yields an unbypassable HSTS error.
func TestProbeDomainInterceptionGatedOnAuth(t *testing.T) {
	server := NewServer("test.com", []dom.DNSPeer{})
	server.SetCaptivePortalIP("10.255.0.1")
	// Only 10.0.0.99 is authenticated.
	server.SetAuthChecker(func(peerIP string) bool { return peerIP == "10.0.0.99" })
	// Forward path for the authenticated case points at TEST-NET (unreachable)
	// so the test never touches real DNS.
	server.SetUpstreamServers([]string{"192.0.2.1:53"})

	portalIP := net.ParseIP("10.255.0.1")

	query := func(peerIP string) *dns.Msg {
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn("www.apple.com"), dns.TypeA)
		w := &mockResponseWriter{remoteAddr: &net.UDPAddr{IP: net.ParseIP(peerIP), Port: 12345}}
		server.handleDNS(w, m)
		return w.msg
	}

	// Unauthenticated peer: probe host MUST be hijacked to the portal IP so the
	// OS "Sign in to network" detector fires.
	unauth := query("10.0.0.5")
	if unauth == nil || len(unauth.Answer) != 1 {
		t.Fatalf("unauthenticated: expected 1 answer, got %v", unauth)
	}
	if a, ok := unauth.Answer[0].(*dns.A); !ok || !a.A.Equal(portalIP) {
		t.Errorf("unauthenticated: expected portal IP %s, got %v", portalIP, unauth.Answer)
	}

	// Authenticated peer: probe host MUST NOT resolve to the portal — that is
	// exactly what produced the HSTS error. It is forwarded upstream instead
	// (which fails in-test), so we assert only that no portal-IP answer appears.
	authed := query("10.0.0.99")
	if authed != nil {
		for _, rr := range authed.Answer {
			if a, ok := rr.(*dns.A); ok && a.A.Equal(portalIP) {
				t.Errorf("authenticated: probe host redirected to portal IP %s (HSTS bug regression)", portalIP)
			}
		}
	}
}

func TestHandleDNSWithNonLocalRecords(t *testing.T) {
	domain := "test.com"
	peers := []dom.DNSPeer{
		{Name: "peer1", IP: "10.0.0.1"},
	}

	server := NewServer(domain, peers)

	// Create DNS query for non-local record
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("google.com"), dns.TypeA)

	// Create mock response writer
	mockWriter := &mockResponseWriter{}

	// This will try to forward to upstream, which will likely fail in test environment
	server.handleDNS(mockWriter, m)

	// Should have attempted to write a response (even if upstream fails)
	if mockWriter.msg == nil {
		t.Error("Expected some response to be written")
	}
}

func TestHandleDNSWithNonARecord(t *testing.T) {
	domain := "test.com"
	peers := []dom.DNSPeer{
		{Name: "peer1", IP: "10.0.0.1"},
	}

	server := NewServer(domain, peers)

	// Create DNS query for MX record (not A record)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("peer1.test.com"), dns.TypeMX)

	// Create mock response writer
	mockWriter := &mockResponseWriter{}

	server.handleDNS(mockWriter, m)

	// Should forward to upstream since it's not an A record
	if mockWriter.msg == nil {
		t.Error("Expected response to be written")
	}
}

func TestForwardToUpstream(t *testing.T) {
	server := NewServer("test.com", []dom.DNSPeer{})

	// Set upstream servers to non-existent servers to test failure handling
	server.SetUpstreamServers([]string{"192.0.2.1:53", "192.0.2.2:53"}) // TEST-NET addresses

	// Create DNS query
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("example.com"), dns.TypeA)

	// Create mock response writer
	mockWriter := &mockResponseWriter{}

	server.forwardToUpstream(mockWriter, m)

	// Should have written a response (likely SERVFAIL due to upstream failure)
	if mockWriter.msg == nil {
		t.Error("Expected response to be written even when upstream fails")
	}

	// If all upstreams fail, should get SERVFAIL
	if mockWriter.msg.Rcode != dns.RcodeServerFailure {
		t.Logf("Expected SERVFAIL when all upstreams fail, got rcode %d", mockWriter.msg.Rcode)
	}
}

func TestStartServer(t *testing.T) {
	server := NewServer("test.com", []dom.DNSPeer{})

	// Test starting server on invalid address (should fail)
	err := server.Start("invalid:address:format")
	if err == nil {
		t.Error("Expected error for invalid address format")
	}

	// Test starting server on port 0 (should work but we can't easily test the actual serving)
	// We skip this test as it would start a real server
	t.Skip("Skipping actual server start test to avoid port conflicts")
}

// Mock response writer for testing

type mockResponseWriter struct {
	msg        *dns.Msg
	remoteAddr net.Addr
}

func (m *mockResponseWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53}
}

func (m *mockResponseWriter) RemoteAddr() net.Addr {
	if m.remoteAddr != nil {
		return m.remoteAddr
	}
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}

func (m *mockResponseWriter) WriteMsg(msg *dns.Msg) error {
	m.msg = msg
	return nil
}

func (m *mockResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (m *mockResponseWriter) Close() error {
	return nil
}

func (m *mockResponseWriter) TsigStatus() error {
	return nil
}

func (m *mockResponseWriter) TsigTimersOnly(bool) {}

func (m *mockResponseWriter) Hijack() {}

// Test edge cases

// TestLookupWildcard verifies that wildcard records resolve correctly and that
// exact matches always take priority over wildcards.
func TestLookupWildcard(t *testing.T) {
	domain := "mynet.internal"
	peers := []dom.DNSPeer{
		// Exact record for "svc.mynet.internal"
		{Name: "svc.mynet.internal", IP: "10.0.0.10"},
		// Bare wildcard: matches any single label under "mynet.internal"
		{Name: "*.mynet.internal", IP: "10.0.0.100"},
		// More-specific wildcard: matches single label under "api.mynet.internal"
		{Name: "*.api.mynet.internal", IP: "10.0.0.200"},
	}
	server := NewServer(domain, peers)

	tests := []struct {
		query    string
		wantIPv4 string
		desc     string
	}{
		// Exact match wins over wildcard
		{"svc.mynet.internal", "10.0.0.10", "exact beats wildcard"},
		// Bare wildcard matches one label
		{"anything.mynet.internal", "10.0.0.100", "bare wildcard match"},
		{"other.mynet.internal", "10.0.0.100", "bare wildcard match (different label)"},
		// More specific wildcard wins
		{"v1.api.mynet.internal", "10.0.0.200", "specific wildcard wins"},
		{"v2.api.mynet.internal", "10.0.0.200", "specific wildcard wins (different label)"},
		// Wildcard does NOT match two levels deep (RFC 4592)
		{"x.y.mynet.internal", "", "two-level query misses bare wildcard"},
		// No match at all
		{"nonexistent.other.com", "", "no match"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := server.lookupPeerIP(tt.query)
			if got != tt.wantIPv4 {
				t.Errorf("lookupPeerIP(%q) = %q, want %q", tt.query, got, tt.wantIPv4)
			}
		})
	}
}

func TestLookupPeerIPEdgeCases(t *testing.T) {
	server := NewServer("", []dom.DNSPeer{}) // Empty domain

	// Test with empty name
	result := server.lookupPeerIP("")
	if result != "" {
		t.Errorf("Expected empty result for empty name, got '%s'", result)
	}

	// Test with peers that have empty names or IPs
	server.peers = []dom.DNSPeer{
		{Name: "", IP: "10.0.0.1"},
		{Name: "peer1", IP: ""},
		{Name: "peer2", IP: "10.0.0.2"},
	}
	server.domain = "test.com"

	result = server.lookupPeerIP("peer2.test.com")
	if result != "10.0.0.2" {
		t.Errorf("Expected '10.0.0.2', got '%s'", result)
	}

	result = server.lookupPeerIP("peer1.test.com")
	if result != "" {
		t.Errorf("Expected empty result for peer with empty IP, got '%s'", result)
	}
}

func TestSetUpstreamServersEdgeCases(t *testing.T) {
	server := NewServer("test.com", []dom.DNSPeer{})

	// Test with empty slice
	server.SetUpstreamServers([]string{})
	if len(server.upstreamServers) != 0 {
		t.Errorf("Expected 0 upstream servers, got %d", len(server.upstreamServers))
	}

	// Test with servers containing colons in various positions
	servers := []string{
		"::1",                  // IPv6
		"[::1]:53",             // IPv6 with port
		"example.com::",        // Invalid format
		"192.168.1.1:53:extra", // Extra colon
	}

	server.SetUpstreamServers(servers)

	// Should handle these gracefully (may not be valid but shouldn't panic)
	if len(server.upstreamServers) != len(servers) {
		t.Errorf("Expected %d upstream servers, got %d", len(servers), len(server.upstreamServers))
	}
}

func TestConcurrentAccess(t *testing.T) {
	server := NewServer("test.com", []dom.DNSPeer{
		{Name: "peer1", IP: "10.0.0.1"},
	})

	// Test concurrent reads and writes
	done := make(chan bool, 2)

	// Goroutine 1: Read operations
	go func() {
		for i := 0; i < 100; i++ {
			server.lookupPeerIP("peer1.test.com")
		}
		done <- true
	}()

	// Goroutine 2: Write operations
	go func() {
		for i := 0; i < 100; i++ {
			server.Update("test.com", []dom.DNSPeer{
				{Name: "peer1", IP: "10.0.0.1"},
				{Name: "peer2", IP: "10.0.0.2"},
			})
			server.SetUpstreamServers([]string{"8.8.8.8", "1.1.1.1"})
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Should not panic or race
}
