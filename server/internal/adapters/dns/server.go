package dns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"wirety/internal/application/network"
	domain "wirety/internal/domain/network"

	"github.com/miekg/dns"
)

// DNSServer is an adapter that exposes network peers via DNS
// Implements hexagonal architecture: depends only on application service

type DNSServer struct {
	service *network.Service
	addr    string // listen address
	zone    string // authoritative zone (network domain)
}

func NewDNSServer(service *network.Service, addr, zone string) *DNSServer {
	return &DNSServer{service: service, addr: addr, zone: zone}
}

func (s *DNSServer) Start() error {
	dns.HandleFunc(s.zone+".", s.handleDNS)
	server := &dns.Server{Addr: s.addr, Net: "udp"}
	return server.ListenAndServe()
}

func (s *DNSServer) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	for _, q := range r.Question {
		name := strings.TrimSuffix(q.Name, ".")
		if !strings.HasSuffix(name, s.zone) {
			continue // not our zone
		}
		labels := strings.Split(name, ".")
		if len(labels) < 2 {
			continue // not peer.network-domain
		}
		peerName := labels[0]
		peerName = sanitizeLabel(peerName)
		zone := s.zone

		peer := s.findPeerByName(peerName, zone)
		if peer == nil {
			continue
		}
		if q.Qtype == dns.TypeA {
			ip := strings.Split(peer.Address, "/")[0]
			rr := &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP(ip),
			}
			m.Answer = append(m.Answer, rr)
		}
	}
	w.WriteMsg(m)
}

func (s *DNSServer) findPeerByName(name, zone string) *domain.Peer {
	nets, err := s.service.ListNetworks(context.Background())
	if err != nil {
		return nil
	}
	for _, net := range nets {
		if net.Domain != zone {
			continue
		}
		for _, peer := range net.GetAllPeers() {
			if sanitizeLabel(peer.Name) == name {
				return peer
			}
		}
	}
	return nil
}

func sanitizeLabel(label string) string {
	label = strings.ToLower(label)
	label = strings.ReplaceAll(label, "_", "-")
	label = strings.ReplaceAll(label, " ", "-")
	label = strings.Trim(label, "-")
	if len(label) > 63 {
		label = label[:63]
	}
	return label
}

func parseIP(addr string) [4]byte {
	// addr is expected as "x.x.x.x" or "x.x.x.x/32"
	ip := strings.Split(addr, "/")[0]
	var out [4]byte
	fmt.Sscanf(ip, "%d.%d.%d.%d", &out[0], &out[1], &out[2], &out[3])
	return out
}
