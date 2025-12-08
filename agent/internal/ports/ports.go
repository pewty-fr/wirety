package ports

import (
	dom "wirety/agent/internal/domain/dns"
	pol "wirety/agent/internal/domain/policy"
)

// ConfigWriterPort defines capability to write and apply WireGuard config.
type ConfigWriterPort interface {
	WriteAndApply(cfg string) error
	UpdateInterface(newInterface string) error
	GetInterface() string
}

// DNSStarterPort defines capability to start DNS server with given domain and peers.
type DNSStarterPort interface {
	Start(addr string) error
	Update(domain string, peers []dom.DNSPeer)
	SetUpstreamServers(servers []string) // Set upstream DNS servers for forwarding
}

// WebSocketClientPort defines capability to connect and receive messages.
type WebSocketClientPort interface {
	Connect(url string) error
	ReadMessage() ([]byte, error)
	WriteMessage(data []byte) error
	Close() error
}

// FirewallPort defines capability to synchronize firewall rules based on policy.
type FirewallPort interface {
	Sync(policy *pol.JumpPolicy, selfIP string, whitelistedIPs []string) error
	SetProxyPorts(httpPort, httpsPort int)
}
