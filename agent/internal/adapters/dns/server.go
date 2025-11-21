package dnsadapter

import (
	"fmt"
	"net"
	"strings"
	dom "wirety/agent/internal/domain/dns"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

// Server implements DNSStarterPort for serving A records.
// It is constructed from domain + list of domain peers.

type Server struct {
	domain string
	peers  []dom.DNSPeer
}

func NewServer(domain string, peers []dom.DNSPeer) *Server {
	return &Server{domain: domain, peers: peers}
}

func (s *Server) Start(addr string) error {
	dns.HandleFunc(s.domain+".", s.handleDNS)
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
	for _, p := range s.peers {
		fqdn := fmt.Sprintf("%s.%s", p.Name, s.domain)
		if name == fqdn {
			return p.IP
		}
	}
	return ""
}
