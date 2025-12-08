package dns

// DNSPeer represents minimal peer info for DNS publishing.
type DNSPeer struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// DNSConfig represents domain + peers list delivered to jump agent.
type DNSConfig struct {
	IP              string    `json:"ip"`
	Domain          string    `json:"domain"`
	Peers           []DNSPeer `json:"peers"`
	UpstreamServers []string  `json:"upstream_servers"` // Upstream DNS servers for forwarding
}
