package ports

import (
	"net/http"

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
	Connect(url string, header http.Header) error
	ReadMessage() ([]byte, error)
	WriteMessage(data []byte) error
	Close() error
}

// FirewallPort defines capability to synchronize firewall rules based on policy.
//
// Sync configures the full three-tier captive-portal authentication gate:
//   • AuthenticatedIPs  — peers that completed SSO; full access via WIRETY_POLICY.
//   • PendingAuthIPs    — peers with an in-flight captive-portal token; allowed
//                         to reach external HTTPS for the OIDC redirect chain.
//   • QuarantinedIPs    — peers blocked entirely after repeated auth failures;
//                         no whitelist, no captive portal redirect.
//   • EndpointDenylist  — public source IP:port pairs to drop at the physical
//                         interface (rogue WireGuard sources sharing a stolen
//                         private key with an authenticated peer).
type FirewallPort interface {
	Sync(req SyncRequest) error
	SetProxyPorts(httpPort, httpsPort int)
}

// SyncRequest carries everything the firewall adapter needs to apply the
// captive-portal authentication gate plus per-policy iptables rules.
type SyncRequest struct {
	Policy            *pol.JumpPolicy
	SelfIP            string
	AuthenticatedIPs  []string         // wgIPs whose SSO is current AND endpoint is stable
	PendingAuthIPs    []string         // wgIPs with an in-flight captive-portal token
	QuarantinedIPs    []string         // wgIPs currently in auth-failure quarantine
	EndpointDenylist  []DenylistEntry  // physical-interface DROP rules
	WireGuardListenPort int            // jump peer's WireGuard UDP listen port (for denylist scoping)
}

// DenylistEntry describes a single rogue source the agent must DROP on its
// physical interface for the WireGuard listen port.  BlockedPort == 0 means
// "any port from BlockedIP".
type DenylistEntry struct {
	BlockedIP   string
	BlockedPort int
}
