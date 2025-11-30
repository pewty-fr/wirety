package dnsadapter

import (
	"fmt"
	"net"
	"strings"
	"sync"
	dom "wirety/agent/internal/domain/dns"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

// Server implements DNSStarterPort for serving A records.
// It is constructed from domain + list of domain peers.

type Server struct {
	domain          string
	peers           []dom.DNSPeer
	upstreamServers []string // Upstream DNS servers for forwarding
	mu              sync.RWMutex
}

func NewServer(domain string, peers []dom.DNSPeer) *Server {
	return &Server{
		domain:          domain,
		peers:           peers,
		upstreamServers: []string{"8.8.8.8:53", "1.1.1.1:53"}, // Default upstream DNS
	}
}

// SetUpstreamServers sets the upstream DNS servers for forwarding
func (s *Server) SetUpstreamServers(servers []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add port 53 if not specified
	s.upstreamServers = make([]string, 0, len(servers))
	for _, server := range servers {
		if !strings.Contains(server, ":") {
			server = server + ":53"
		}
		s.upstreamServers = append(s.upstreamServers, server)
	}

	log.Info().Strs("upstream_servers", s.upstreamServers).Msg("DNS upstream servers updated")
}

func (s *Server) Start(addr string) error {
	// Register handler for all DNS queries (not just s.domain)
	// This allows us to handle both peer domains and route domains with different suffixes
	dns.HandleFunc(".", s.handleDNS)
	server := &dns.Server{Addr: addr, Net: "udp"}
	log.Info().Str("addr", addr).Strs("upstream", s.upstreamServers).Str("domain", s.domain).Int("peer_count", len(s.peers)).Msg("starting DNS server")
	return server.ListenAndServe()
}

func (s *Server) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	// Try to resolve from local records first
	resolved := false
	for _, q := range r.Question {
		if q.Qtype == dns.TypeA {
			name := strings.TrimSuffix(q.Name, ".")
			if ip := s.lookupPeerIP(name); ip != "" {
				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    60,
					},
					A: net.ParseIP(ip),
				})
				resolved = true
			}
		}
	}

	// If we resolved it locally, return the answer
	if resolved {
		_ = w.WriteMsg(m)
		return
	}

	// Otherwise, forward to upstream DNS servers
	s.forwardToUpstream(w, r)
}

// forwardToUpstream forwards DNS queries to upstream DNS servers
func (s *Server) forwardToUpstream(w dns.ResponseWriter, r *dns.Msg) {
	s.mu.RLock()
	upstreams := s.upstreamServers
	s.mu.RUnlock()

	// Try each upstream server until one responds
	for _, upstream := range upstreams {
		c := new(dns.Client)
		c.Net = "udp"

		resp, _, err := c.Exchange(r, upstream)
		if err != nil {
			log.Debug().
				Err(err).
				Str("upstream", upstream).
				Str("query", r.Question[0].Name).
				Msg("failed to forward DNS query to upstream")
			continue
		}

		// Successfully got a response from upstream
		log.Debug().
			Str("upstream", upstream).
			Str("query", r.Question[0].Name).
			Int("answers", len(resp.Answer)).
			Msg("forwarded DNS query to upstream")

		_ = w.WriteMsg(resp)
		return
	}

	// All upstreams failed, return SERVFAIL
	m := new(dns.Msg)
	m.SetReply(r)
	m.SetRcode(r, dns.RcodeServerFailure)
	_ = w.WriteMsg(m)

	log.Warn().
		Str("query", r.Question[0].Name).
		Msg("all upstream DNS servers failed")
}

func (s *Server) lookupPeerIP(name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.peers {
		// Check if this is a route DNS mapping (contains full FQDN in Name field)
		// Route DNS mappings have format: name.route_name.domain_suffix
		// and are stored with the full FQDN in the Name field
		if strings.Contains(p.Name, ".") {
			// This is a route DNS mapping with full FQDN
			fqdn := p.Name
			if name == fqdn {
				return p.IP
			}
		} else {
			// This is a peer DNS record, construct FQDN
			fqdn := fmt.Sprintf("%s.%s", p.Name, s.domain)
			if name == fqdn {
				return p.IP
			}
		}
	}
	return ""
}

// Update updates the DNS server configuration with new domain, peers, and upstream servers
func (s *Server) Update(domain string, peers []dom.DNSPeer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.domain = domain
	s.peers = peers

	log.Info().
		Str("domain", domain).
		Int("peer_count", len(peers)).
		Strs("upstream_servers", s.upstreamServers).
		Msg("DNS server configuration updated")
}
