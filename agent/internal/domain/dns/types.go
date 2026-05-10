package dns

// DNSPeer represents minimal peer info for DNS publishing.
type DNSPeer struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	IPv6 string `json:"ipv6,omitempty"` // IPv6 WireGuard address (optional, set for dual-stack networks)
}

// DNSConfig represents domain + peers list delivered to jump agent.
type DNSConfig struct {
	IP              string    `json:"ip"`
	Domain          string    `json:"domain"`
	Peers           []DNSPeer `json:"peers"`
	UpstreamServers []string  `json:"upstream_servers"` // Upstream DNS servers for forwarding
}
