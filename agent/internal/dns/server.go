package dns

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

// Peer holds peer name and IP
// This should be populated from the agent's config
// For now, use a stub. Replace with real config integration.
type Peer struct {
	Name string
	IP   string
}

type Server struct {
	Domain string
	Peers  []Peer
}

func NewServer(domain string, peers []Peer) *Server {
	return &Server{Domain: domain, Peers: peers}
}

func (s *Server) Start(addr string) error {
	dns.HandleFunc(s.Domain+".", s.handleDNS)
	server := &dns.Server{Addr: addr, Net: "udp"}
	log.Info().Str("addr", addr).Str("domain", s.Domain).Msg("starting DNS server")
	return server.ListenAndServe()
}

func (s *Server) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	for _, q := range r.Question {
		if q.Qtype == dns.TypeA {
			name := strings.TrimSuffix(q.Name, ".")
			if ip := s.lookupPeerIP(name); ip != "" {
				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
					A:   net.ParseIP(ip),
				})
			}
		}
	}
	w.WriteMsg(m)
}

func (s *Server) lookupPeerIP(name string) string {
	for _, p := range s.Peers {
		fqdn := fmt.Sprintf("%s.%s", p.Name, s.Domain)
		if name == fqdn {
			return p.IP
		}
	}
	return ""
}
