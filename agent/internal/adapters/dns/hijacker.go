package dnsadapter

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

// Hijacker implements a DNS server that hijacks queries from non-authenticated users
// and redirects them to the captive portal server
type Hijacker struct {
	port             int
	serverURL        string
	serverIP         string
	serverDomain     string
	server           *dns.Server
	mu               sync.RWMutex
	nonAgentPeers    map[string]bool // IP -> true for non-agent peers
	whitelistedPeers map[string]bool // IP -> true for authenticated peers
	upstreamDNS      string          // Upstream DNS server for authenticated users
}

// NewHijacker creates a new DNS hijacker
func NewHijacker(port int, serverURL string, upstreamDNS string) (*Hijacker, error) {
	// Parse server URL to extract domain
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	// Extract hostname (remove port if present)
	hostname := parsedURL.Hostname()

	// Resolve server IP
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve server domain: %w", err)
	}

	var serverIP string
	for _, ip := range ips {
		if ip.To4() != nil {
			serverIP = ip.String()
			break
		}
	}

	if serverIP == "" {
		return nil, fmt.Errorf("no IPv4 address found for server domain")
	}

	hijacker := &Hijacker{
		port:             port,
		serverURL:        serverURL,
		serverIP:         serverIP,
		serverDomain:     strings.ToLower(hostname),
		nonAgentPeers:    make(map[string]bool),
		whitelistedPeers: make(map[string]bool),
		upstreamDNS:      upstreamDNS,
	}

	log.Info().
		Str("server_domain", hostname).
		Str("server_ip", serverIP).
		Int("port", port).
		Msg("DNS hijacker initialized")

	return hijacker, nil
}

// UpdateNonAgentPeers updates the list of non-agent peer IPs
func (h *Hijacker) UpdateNonAgentPeers(peerIPs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.nonAgentPeers = make(map[string]bool)
	for _, ip := range peerIPs {
		h.nonAgentPeers[ip] = true
	}

	log.Debug().Int("count", len(peerIPs)).Msg("updated non-agent peers for DNS hijacker")
}

// AddWhitelistedPeer adds a peer IP to the whitelist
func (h *Hijacker) AddWhitelistedPeer(ip string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.whitelistedPeers[ip] = true
}

// RemoveWhitelistedPeer removes a peer IP from the whitelist
func (h *Hijacker) RemoveWhitelistedPeer(ip string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.whitelistedPeers, ip)
}

// ClearWhitelist clears all whitelisted peers
func (h *Hijacker) ClearWhitelist() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.whitelistedPeers = make(map[string]bool)
}

// isNonAgentPeer checks if an IP is a non-agent peer
func (h *Hijacker) isNonAgentPeer(ip string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.nonAgentPeers[ip]
}

// isWhitelisted checks if an IP is whitelisted (authenticated)
func (h *Hijacker) isWhitelisted(ip string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.whitelistedPeers[ip]
}

// Start starts the DNS hijacker server
func (h *Hijacker) Start() error {
	dns.HandleFunc(".", h.handleDNS)

	h.server = &dns.Server{
		Addr: fmt.Sprintf(":%d", h.port),
		Net:  "udp",
	}

	log.Info().Int("port", h.port).Msg("DNS hijacker started")

	go func() {
		if err := h.server.ListenAndServe(); err != nil {
			log.Error().Err(err).Msg("DNS hijacker server error")
		}
	}()

	return nil
}

// Stop stops the DNS hijacker server
func (h *Hijacker) Stop() error {
	if h.server != nil {
		return h.server.Shutdown()
	}
	return nil
}

// handleDNS handles DNS queries
func (h *Hijacker) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	// Extract client IP
	clientIP := w.RemoteAddr().String()
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		clientIP = clientIP[:idx]
	}

	m := new(dns.Msg)
	m.SetReply(r)

	// Check if this is a non-agent peer
	if !h.isNonAgentPeer(clientIP) {
		// Not a non-agent peer, forward to upstream DNS
		h.forwardToUpstream(w, r)
		return
	}

	// Check if peer is whitelisted (authenticated)
	if h.isWhitelisted(clientIP) {
		// Whitelisted peer, forward to upstream DNS
		h.forwardToUpstream(w, r)
		return
	}

	// Non-authenticated user - hijack all DNS queries
	for _, q := range r.Question {
		if q.Qtype == dns.TypeA {
			queryDomain := strings.TrimSuffix(strings.ToLower(q.Name), ".")

			log.Debug().
				Str("client_ip", clientIP).
				Str("query", queryDomain).
				Str("hijacked_ip", h.serverIP).
				Msg("hijacking DNS query")

			// Return server IP for all queries
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    1, // Very short TTL so it updates quickly after authentication
				},
				A: net.ParseIP(h.serverIP),
			})
		}
	}

	_ = w.WriteMsg(m)
}

// forwardToUpstream forwards the DNS query to the upstream DNS server
func (h *Hijacker) forwardToUpstream(w dns.ResponseWriter, r *dns.Msg) {
	c := new(dns.Client)
	resp, _, err := c.Exchange(r, h.upstreamDNS)
	if err != nil {
		log.Error().Err(err).Msg("failed to forward DNS query to upstream")
		// Return SERVFAIL
		m := new(dns.Msg)
		m.SetReply(r)
		m.SetRcode(r, dns.RcodeServerFailure)
		_ = w.WriteMsg(m)
		return
	}

	_ = w.WriteMsg(resp)
}
