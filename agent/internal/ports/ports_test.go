package ports

import (
	"testing"

	dom "wirety/agent/internal/domain/dns"
	pol "wirety/agent/internal/domain/policy"
)

// Test that all port interfaces are properly defined and can be implemented

// mockConfigWriter implements ConfigWriterPort for testing
type mockConfigWriter struct {
	config        string
	interfaceName string
	applied       bool
}

func (m *mockConfigWriter) WriteAndApply(cfg string) error {
	m.config = cfg
	m.applied = true
	return nil
}

func (m *mockConfigWriter) UpdateInterface(newInterface string) error {
	m.interfaceName = newInterface
	return nil
}

func (m *mockConfigWriter) GetInterface() string {
	return m.interfaceName
}

// mockDNSStarter implements DNSStarterPort for testing
type mockDNSStarter struct {
	addr            string
	domain          string
	peers           []dom.DNSPeer
	upstreamServers []string
	started         bool
}

func (m *mockDNSStarter) Start(addr string) error {
	m.addr = addr
	m.started = true
	return nil
}

func (m *mockDNSStarter) Update(domain string, peers []dom.DNSPeer) {
	m.domain = domain
	m.peers = peers
}

func (m *mockDNSStarter) SetUpstreamServers(servers []string) {
	m.upstreamServers = servers
}

// mockWebSocketClient implements WebSocketClientPort for testing
type mockWebSocketClient struct {
	url       string
	connected bool
	messages  [][]byte
	closed    bool
}

func (m *mockWebSocketClient) Connect(url string) error {
	m.url = url
	m.connected = true
	return nil
}

func (m *mockWebSocketClient) ReadMessage() ([]byte, error) {
	if len(m.messages) > 0 {
		msg := m.messages[0]
		m.messages = m.messages[1:]
		return msg, nil
	}
	return nil, nil
}

func (m *mockWebSocketClient) WriteMessage(data []byte) error {
	m.messages = append(m.messages, data)
	return nil
}

func (m *mockWebSocketClient) Close() error {
	m.closed = true
	m.connected = false
	return nil
}

// mockFirewall implements FirewallPort for testing
type mockFirewall struct {
	policy         *pol.JumpPolicy
	selfIP         string
	whitelistedIPs []string
	httpPort       int
	httpsPort      int
	synced         bool
}

func (m *mockFirewall) Sync(policy *pol.JumpPolicy, selfIP string, whitelistedIPs []string) error {
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

// Test ConfigWriterPort interface
func TestConfigWriterPort(t *testing.T) {
	var port ConfigWriterPort = &mockConfigWriter{}

	// Test WriteAndApply
	err := port.WriteAndApply("test config")
	if err != nil {
		t.Errorf("WriteAndApply failed: %v", err)
	}

	mock := port.(*mockConfigWriter)
	if mock.config != "test config" {
		t.Errorf("Expected config 'test config', got '%s'", mock.config)
	}
	if !mock.applied {
		t.Error("Expected config to be applied")
	}

	// Test UpdateInterface
	err = port.UpdateInterface("wg1")
	if err != nil {
		t.Errorf("UpdateInterface failed: %v", err)
	}
	if mock.interfaceName != "wg1" {
		t.Errorf("Expected interface 'wg1', got '%s'", mock.interfaceName)
	}

	// Test GetInterface
	iface := port.GetInterface()
	if iface != "wg1" {
		t.Errorf("Expected interface 'wg1', got '%s'", iface)
	}
}

// Test DNSStarterPort interface
func TestDNSStarterPort(t *testing.T) {
	var port DNSStarterPort = &mockDNSStarter{}

	// Test Start
	err := port.Start("127.0.0.1:53")
	if err != nil {
		t.Errorf("Start failed: %v", err)
	}

	mock := port.(*mockDNSStarter)
	if mock.addr != "127.0.0.1:53" {
		t.Errorf("Expected addr '127.0.0.1:53', got '%s'", mock.addr)
	}
	if !mock.started {
		t.Error("Expected DNS server to be started")
	}

	// Test Update
	peers := []dom.DNSPeer{
		{Name: "peer1", IP: "10.0.0.1"},
		{Name: "peer2", IP: "10.0.0.2"},
	}
	port.Update("example.com", peers)

	if mock.domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", mock.domain)
	}
	if len(mock.peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(mock.peers))
	}

	// Test SetUpstreamServers
	servers := []string{"8.8.8.8", "1.1.1.1"}
	port.SetUpstreamServers(servers)

	if len(mock.upstreamServers) != 2 {
		t.Errorf("Expected 2 upstream servers, got %d", len(mock.upstreamServers))
	}
}

// Test WebSocketClientPort interface
func TestWebSocketClientPort(t *testing.T) {
	var port WebSocketClientPort = &mockWebSocketClient{}

	// Test Connect
	err := port.Connect("ws://localhost:8080")
	if err != nil {
		t.Errorf("Connect failed: %v", err)
	}

	mock := port.(*mockWebSocketClient)
	if mock.url != "ws://localhost:8080" {
		t.Errorf("Expected URL 'ws://localhost:8080', got '%s'", mock.url)
	}
	if !mock.connected {
		t.Error("Expected client to be connected")
	}

	// Test WriteMessage
	testData := []byte("test message")
	err = port.WriteMessage(testData)
	if err != nil {
		t.Errorf("WriteMessage failed: %v", err)
	}

	// Test ReadMessage
	data, err := port.ReadMessage()
	if err != nil {
		t.Errorf("ReadMessage failed: %v", err)
	}
	if string(data) != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", string(data))
	}

	// Test Close
	err = port.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if mock.connected {
		t.Error("Expected client to be disconnected")
	}
	if !mock.closed {
		t.Error("Expected client to be closed")
	}
}

// Test FirewallPort interface
func TestFirewallPort(t *testing.T) {
	var port FirewallPort = &mockFirewall{}

	// Test SetProxyPorts
	port.SetProxyPorts(3128, 3129)

	mock := port.(*mockFirewall)
	if mock.httpPort != 3128 {
		t.Errorf("Expected HTTP port 3128, got %d", mock.httpPort)
	}
	if mock.httpsPort != 3129 {
		t.Errorf("Expected HTTPS port 3129, got %d", mock.httpsPort)
	}

	// Test Sync
	policy := &pol.JumpPolicy{
		IP:            "10.0.0.1",
		IPTablesRules: []string{"-A INPUT -j ACCEPT"},
	}
	whitelistedIPs := []string{"10.0.0.2", "10.0.0.3"}

	err := port.Sync(policy, "10.0.0.1", whitelistedIPs)
	if err != nil {
		t.Errorf("Sync failed: %v", err)
	}

	if mock.policy != policy {
		t.Error("Expected policy to be set")
	}
	if mock.selfIP != "10.0.0.1" {
		t.Errorf("Expected selfIP '10.0.0.1', got '%s'", mock.selfIP)
	}
	if len(mock.whitelistedIPs) != 2 {
		t.Errorf("Expected 2 whitelisted IPs, got %d", len(mock.whitelistedIPs))
	}
	if !mock.synced {
		t.Error("Expected firewall to be synced")
	}
}
