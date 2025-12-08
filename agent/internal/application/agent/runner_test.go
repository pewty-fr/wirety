package agent

import (
	"encoding/json"
	"testing"
	"time"
	dom "wirety/agent/internal/domain/dns"
	pol "wirety/agent/internal/domain/policy"
)

// Mock implementations for testing

type mockWebSocketClient struct {
	url        string
	connected  bool
	messages   [][]byte
	readIndex  int
	closed     bool
	connectErr error
	readErr    error
	writeErr   error
}

func (m *mockWebSocketClient) Connect(url string) error {
	if m.connectErr != nil {
		return m.connectErr
	}
	m.url = url
	m.connected = true
	return nil
}

func (m *mockWebSocketClient) ReadMessage() ([]byte, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	if m.readIndex < len(m.messages) {
		msg := m.messages[m.readIndex]
		m.readIndex++
		return msg, nil
	}
	// Block indefinitely if no more messages (simulating real WebSocket behavior)
	select {}
}

func (m *mockWebSocketClient) WriteMessage(data []byte) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	m.messages = append(m.messages, data)
	return nil
}

func (m *mockWebSocketClient) Close() error {
	m.closed = true
	m.connected = false
	return nil
}

type mockConfigWriter struct {
	config        string
	interfaceName string
	applied       bool
	writeErr      error
	updateErr     error
}

func (m *mockConfigWriter) WriteAndApply(cfg string) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	m.config = cfg
	m.applied = true
	return nil
}

func (m *mockConfigWriter) UpdateInterface(newInterface string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.interfaceName = newInterface
	return nil
}

func (m *mockConfigWriter) GetInterface() string {
	return m.interfaceName
}

type mockDNSServer struct {
	addr            string
	domain          string
	peers           []dom.DNSPeer
	upstreamServers []string
	started         bool
	startErr        error
}

func (m *mockDNSServer) Start(addr string) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.addr = addr
	m.started = true
	return nil
}

func (m *mockDNSServer) Update(domain string, peers []dom.DNSPeer) {
	m.domain = domain
	m.peers = peers
}

func (m *mockDNSServer) SetUpstreamServers(servers []string) {
	m.upstreamServers = servers
}

type mockFirewall struct {
	policy         *pol.JumpPolicy
	selfIP         string
	whitelistedIPs []string
	httpPort       int
	httpsPort      int
	synced         bool
	syncErr        error
}

func (m *mockFirewall) Sync(policy *pol.JumpPolicy, selfIP string, whitelistedIPs []string) error {
	if m.syncErr != nil {
		return m.syncErr
	}
	m.policy = policy
	m.selfIP = selfIP
	m.whitelistedIPs = whitelistedIPs
	m.synced = true
	return nil
}

func (m *mockFirewall) SetProxyPorts(httpPort, httpsPort int) {
	m.httpPort = httpPort
	m.httpsPort = httpsPort
}

func TestNewRunner(t *testing.T) {
	wsClient := &mockWebSocketClient{}
	writer := &mockConfigWriter{}
	dnsServer := &mockDNSServer{}
	fwAdapter := &mockFirewall{}

	runner := NewRunner(wsClient, writer, dnsServer, fwAdapter, "ws://localhost:8080", "wg0")

	if runner.wsClient != wsClient {
		t.Error("Expected wsClient to be set")
	}

	if runner.cfgWriter != writer {
		t.Error("Expected cfgWriter to be set")
	}

	if runner.dnsServer != dnsServer {
		t.Error("Expected dnsServer to be set")
	}

	if runner.fwAdapter != fwAdapter {
		t.Error("Expected fwAdapter to be set")
	}

	if runner.wsURL != "ws://localhost:8080" {
		t.Errorf("Expected wsURL 'ws://localhost:8080', got '%s'", runner.wsURL)
	}

	if runner.wgInterface != "wg0" {
		t.Errorf("Expected wgInterface 'wg0', got '%s'", runner.wgInterface)
	}

	if runner.backoffBase != time.Second {
		t.Errorf("Expected backoffBase 1s, got %v", runner.backoffBase)
	}

	if runner.backoffMax != 30*time.Second {
		t.Errorf("Expected backoffMax 30s, got %v", runner.backoffMax)
	}

	if runner.heartbeatInterval != 30*time.Second {
		t.Errorf("Expected heartbeatInterval 30s, got %v", runner.heartbeatInterval)
	}
}

func TestSanitizeInterfaceName(t *testing.T) {
	runner := &Runner{}

	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with-dash"},
		{"with_underscore", "with_underscore"},
		{"With.Dots", "with_dots"},
		{"with spaces", "with_spaces"},
		{"with@special#chars", "with_special_ch"}, // Truncated to 15 chars
		{"UPPERCASE", "uppercase"},
		{"verylongnamethatexceedsfifteencharacters", "verylongnametha"}, // Truncated to 15 chars
		{"", "wg0"}, // Empty becomes default
		{"123numbers", "123numbers"},
		{"mixed-123_test", "mixed-123_test"},
	}

	for _, tt := range tests {
		got := runner.sanitizeInterfaceName(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeInterfaceName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSetCurrentPeerName(t *testing.T) {
	runner := &Runner{}

	runner.SetCurrentPeerName("test-peer")

	if runner.currentPeerName != "test-peer" {
		t.Errorf("Expected currentPeerName 'test-peer', got '%s'", runner.currentPeerName)
	}
}

func TestHandlePeerNameChange(t *testing.T) {
	writer := &mockConfigWriter{interfaceName: "wg0"}
	runner := &Runner{
		cfgWriter:       writer,
		currentPeerName: "old-peer",
		wgInterface:     "wg0",
	}

	// Test no change
	err := runner.handlePeerNameChange("old-peer")
	if err != nil {
		t.Errorf("Expected no error for same peer name, got: %v", err)
	}

	// Test empty name
	err = runner.handlePeerNameChange("")
	if err != nil {
		t.Errorf("Expected no error for empty peer name, got: %v", err)
	}

	// Test name change that results in same interface
	err = runner.handlePeerNameChange("old_peer") // Sanitizes to same interface
	if err != nil {
		t.Errorf("Expected no error for name change with same interface, got: %v", err)
	}

	// Test name change that results in different interface
	err = runner.handlePeerNameChange("new-peer")
	if err != nil {
		t.Errorf("Expected no error for valid name change, got: %v", err)
	}

	if runner.currentPeerName != "new-peer" {
		t.Errorf("Expected currentPeerName 'new-peer', got '%s'", runner.currentPeerName)
	}

	expectedInterface := "new-peer"
	if writer.interfaceName != expectedInterface {
		t.Errorf("Expected interface '%s', got '%s'", expectedInterface, writer.interfaceName)
	}
}

func TestHandlePeerNameChangeWithError(t *testing.T) {
	writer := &mockConfigWriter{
		interfaceName: "wg0",
		updateErr:     &mockError{"update failed"},
	}
	runner := &Runner{
		cfgWriter:       writer,
		currentPeerName: "old-peer",
		wgInterface:     "wg0",
	}

	err := runner.handlePeerNameChange("new-peer")
	if err == nil {
		t.Error("Expected error when UpdateInterface fails")
	}

	if !contains(err.Error(), "failed to update interface") {
		t.Errorf("Expected error to mention interface update, got: %v", err)
	}
}

func TestSendHeartbeat(t *testing.T) {
	wsClient := &mockWebSocketClient{}
	runner := &Runner{
		wsClient:    wsClient,
		wgInterface: "wg0",
	}

	// This will call CollectSystemInfo which may fail in test environment
	// We mainly test that it doesn't panic
	runner.sendHeartbeat()

	// If system info collection succeeds, we should have a message
	if len(wsClient.messages) > 0 {
		// Verify the message is valid JSON
		var heartbeat map[string]interface{}
		err := json.Unmarshal(wsClient.messages[0], &heartbeat)
		if err != nil {
			t.Errorf("Expected valid JSON heartbeat, got error: %v", err)
		}

		// Check for expected fields
		expectedFields := []string{"hostname", "system_uptime", "wireguard_uptime", "peer_endpoints"}
		for _, field := range expectedFields {
			if _, exists := heartbeat[field]; !exists {
				t.Errorf("Expected heartbeat to contain field '%s'", field)
			}
		}
	}
}

func TestSendHeartbeatWithWriteError(t *testing.T) {
	wsClient := &mockWebSocketClient{
		writeErr: &mockError{"write failed"},
	}
	runner := &Runner{
		wsClient:    wsClient,
		wgInterface: "wg0",
	}

	// Should not panic even if write fails
	runner.sendHeartbeat()
}

// Helper types and functions

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// Test WebSocket message processing

func TestProcessWSMessage(t *testing.T) {
	wsClient := &mockWebSocketClient{}
	writer := &mockConfigWriter{}
	dnsServer := &mockDNSServer{}
	fwAdapter := &mockFirewall{}

	runner := NewRunner(wsClient, writer, dnsServer, fwAdapter, "ws://localhost:8080", "wg0")

	// Create a test message
	msg := WSMessage{
		Config:   "[Interface]\nPrivateKey = test\n",
		PeerName: "test-peer",
		DNS: &dom.DNSConfig{
			IP:              "10.0.0.1",
			Domain:          "example.com",
			Peers:           []dom.DNSPeer{{Name: "peer1", IP: "10.0.0.2"}},
			UpstreamServers: []string{"8.8.8.8", "1.1.1.1"},
		},
		Policy: &pol.JumpPolicy{
			IP:            "10.0.0.1",
			IPTablesRules: []string{"-A INPUT -j ACCEPT"},
		},
		Whitelist:   []string{"10.0.0.3"},
		OAuthIssuer: "https://oauth.example.com",
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal test message: %v", err)
	}

	wsClient.messages = [][]byte{msgBytes}

	// Start runner in a goroutine and stop it quickly
	stop := make(chan struct{})
	go runner.Start(stop)

	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)
	close(stop)

	// Verify config was applied
	if !writer.applied {
		t.Error("Expected config to be applied")
	}

	if writer.config != msg.Config {
		t.Errorf("Expected config '%s', got '%s'", msg.Config, writer.config)
	}

	// Verify DNS server was updated
	if dnsServer.domain != "example.com" {
		t.Errorf("Expected DNS domain 'example.com', got '%s'", dnsServer.domain)
	}

	if len(dnsServer.peers) != 1 {
		t.Errorf("Expected 1 DNS peer, got %d", len(dnsServer.peers))
	}

	if len(dnsServer.upstreamServers) != 2 {
		t.Errorf("Expected 2 upstream servers, got %d", len(dnsServer.upstreamServers))
	}

	// Verify firewall was synced
	if !fwAdapter.synced {
		t.Error("Expected firewall to be synced")
	}

	if fwAdapter.policy == nil {
		t.Error("Expected firewall policy to be set")
	} else if fwAdapter.policy.IP != msg.Policy.IP {
		t.Errorf("Expected firewall policy IP '%s', got '%s'", msg.Policy.IP, fwAdapter.policy.IP)
	}

	if len(fwAdapter.whitelistedIPs) != 1 {
		t.Errorf("Expected 1 whitelisted IP, got %d", len(fwAdapter.whitelistedIPs))
	}
}

func TestProcessWSMessageWithErrors(t *testing.T) {
	wsClient := &mockWebSocketClient{}
	writer := &mockConfigWriter{writeErr: &mockError{"write failed"}}
	dnsServer := &mockDNSServer{}
	fwAdapter := &mockFirewall{syncErr: &mockError{"sync failed"}}

	runner := NewRunner(wsClient, writer, dnsServer, fwAdapter, "ws://localhost:8080", "wg0")

	// Create a test message
	msg := WSMessage{
		Config: "[Interface]\nPrivateKey = test\n",
		Policy: &pol.JumpPolicy{
			IP:            "10.0.0.1",
			IPTablesRules: []string{"-A INPUT -j ACCEPT"},
		},
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal test message: %v", err)
	}

	wsClient.messages = [][]byte{msgBytes}

	// Start runner in a goroutine and stop it quickly
	stop := make(chan struct{})
	go runner.Start(stop)

	// Give it a moment to process
	time.Sleep(50 * time.Millisecond)
	close(stop)

	// Verify that errors don't prevent processing
	// Config write should have been attempted despite error
	if writer.applied {
		t.Error("Expected config not to be applied due to error")
	}

	// Firewall sync should have been attempted despite error
	if fwAdapter.synced {
		t.Error("Expected firewall not to be synced due to error")
	}
}

func TestStartWithConnectionError(t *testing.T) {
	wsClient := &mockWebSocketClient{
		connectErr: &mockError{"connection failed"},
	}
	writer := &mockConfigWriter{}
	dnsServer := &mockDNSServer{}
	fwAdapter := &mockFirewall{}

	runner := NewRunner(wsClient, writer, dnsServer, fwAdapter, "ws://localhost:8080", "wg0")

	// Start runner in a goroutine and stop it quickly
	stop := make(chan struct{})
	go runner.Start(stop)

	// Give it a moment to try connecting
	time.Sleep(50 * time.Millisecond)
	close(stop)

	// Should not panic despite connection errors
}
