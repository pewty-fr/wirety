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
	domain string
	peers  []dom.DNSPeer
	mu     sync.RWMutex
}

func NewServer(domain string, peers []dom.DNSPeer) *Server {
	return &Server{domain: domain, peers: peers}
}

func (s *Server) Start(addr string) error {
	// Register handler for all DNS queries (not just s.domain)
	// This allows us to handle both peer domains and route domains with different suffixes
	dns.HandleFunc(".", s.handleDNS)
	server := &dns.Server{Addr: addr, Net: "udp"}
	log.Info().Str("addr", addr).Str("domain", s.domain).Int("peer_count", len(s.peers)).Msg("starting DNS server")
	return server.ListenAndServe()
}

func (s *Server) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	for _, q := range r.Question {
		if q.Qtype == dns.TypeA {
			name := strings.TrimSuffix(q.Name, ".")
			if ip := s.lookupPeerIP(name); ip != "" {
				m.Answer = append(m.Answer, &dns.A{Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60}, A: net.ParseIP(ip)})
			}
		}
	}
	_ = w.WriteMsg(m)
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

// Update updates the DNS server configuration with new domain and peers
func (s *Server) Update(domain string, peers []dom.DNSPeer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.domain = domain
	s.peers = peers

	log.Info().Str("domain", domain).Int("peer_count", len(peers)).Msg("DNS server configuration updated")
}
