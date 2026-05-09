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

// captiveProbeHosts mirrors the set in the captiveportal package.
// Keeping a local copy avoids an import cycle between the dns and captiveportal packages.
var captiveProbeHosts = map[string]struct{}{
	"connectivitycheck.gstatic.com": {},
	"clients3.google.com":           {},
	"clients1.google.com":           {},
	"captive.apple.com":             {},
	"www.apple.com":                 {},
	"www.msftconnecttest.com":       {},
	"ipv6.msftconnecttest.com":      {},
	"detectportal.firefox.com":      {},
	"nmcheck.gnome.org":             {},
	"network-test.debian.org":       {},
}

type Server struct {
	domain          string
	peers           []dom.DNSPeer
	upstreamServers []string // Upstream DNS servers for forwarding
	captivePortalIP string   // WireGuard IP of this jump peer; when set, probe domains resolve here
	isAuthenticated func(peerIP string) bool
	// redirectExclusions is the set of hostnames that must always resolve to
	// their real peer IP even for unauthenticated peers — typically the Wirety
	// server and captive portal page hostnames. Without these exclusions the
	// captive portal redirect URL itself would resolve to the captive portal IP,
	// causing an infinite redirect loop.
	redirectExclusions map[string]struct{}
	mu                 sync.RWMutex
}

func NewServer(domain string, peers []dom.DNSPeer) *Server {
	return &Server{
		domain:          domain,
		peers:           peers,
		upstreamServers: []string{"8.8.8.8:53", "1.1.1.1:53"}, // Default upstream DNS
	}
}

// SetCaptivePortalIP sets the WireGuard IP that well-known OS captive-portal probe
// domains should resolve to. When set, probes are routed through the tunnel even
// when AllowedIPs is restricted to a private range, and the captive portal HTTP
// server (listening on that IP:80) handles them directly.
func (s *Server) SetCaptivePortalIP(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.captivePortalIP = ip
	log.Info().Str("ip", ip).Msg("DNS: captive portal probe interception enabled")
}

// SetAuthChecker sets a callback that reports whether a peer IP has completed
// captive portal authentication.
//
// When set alongside SetCaptivePortalIP, internal VPN domain queries (peer names,
// route FQDNs) from unauthenticated peers are resolved to the captive portal IP
// instead of the real peer IP. This ensures that any attempt to reach a private
// resource triggers a redirect to the authentication page, without touching
// external DNS resolution (internet keeps working for unauthenticated peers).
//
// For full-tunnel setups (AllowedIPs = 0.0.0.0/0) the OS captive portal detection
// (CNA / NCSI) fires automatically via the probe domain interception above.
func (s *Server) SetAuthChecker(fn func(peerIP string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isAuthenticated = fn
}

// SetRedirectExclusions sets the hostnames that must always resolve to their real
// peer IP, even for unauthenticated peers. Use this to exclude the Wirety server
// hostname and captive portal page hostname so that the redirect target URL itself
// is reachable — otherwise the portal redirect URL resolves to the captive portal
// IP and causes an infinite redirect loop.
func (s *Server) SetRedirectExclusions(hosts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.redirectExclusions = make(map[string]struct{}, len(hosts))
	for _, h := range hosts {
		s.redirectExclusions[h] = struct{}{}
	}
	log.Info().Strs("exclusions", hosts).Msg("DNS: redirect exclusions updated")
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

	// Extract the peer IP from the query source for auth checks.
	peerIP := ""
	if addr := w.RemoteAddr(); addr != nil {
		if host, _, err := net.SplitHostPort(addr.String()); err == nil {
			peerIP = host
		}
	}

	s.mu.RLock()
	portalIP := s.captivePortalIP
	authFn := s.isAuthenticated
	exclusions := s.redirectExclusions
	s.mu.RUnlock()

	// Is this peer unauthenticated and should internal domains be redirected?
	redirectInternal := portalIP != "" && authFn != nil && peerIP != "" && !authFn(peerIP)

	resolved := false
	for _, q := range r.Question {
		// Only handle A and AAAA; forward everything else to upstream.
		if q.Qtype != dns.TypeA && q.Qtype != dns.TypeAAAA {
			continue
		}
		name := strings.TrimSuffix(q.Name, ".")

		// 1. Internal VPN domain records (peer names, route FQDNs).
		//
		// For authenticated peers: return the real peer IP (normal behaviour).
		//
		// For unauthenticated peers: return the captive portal IP with a short TTL
		// so that any attempt to reach a private resource triggers a redirect to the
		// authentication page. External DNS is unaffected — internet keeps working.
		//
		// For AAAA queries on internal domains: always return NODATA. VPN peer IPs
		// are IPv4-only; returning NODATA forces the OS to use the A record.
		if realIP := s.lookupPeerIP(name); realIP != "" {
			if q.Qtype == dns.TypeAAAA {
				// NODATA — the VPN has no IPv6 peer addresses.
				resolved = true
				continue
			}
			resolvedIP := realIP
			ttl := uint32(60)
			// Redirect unauthenticated peers to captive portal IP, unless this
			// hostname is excluded (e.g. the portal hostname itself or the Wirety
			// server hostname). Without the exclusion, the redirect target URL
			// would also resolve to the captive portal IP, causing an infinite
			// redirect loop.
			_, isExcluded := exclusions[name]
			if redirectInternal && !isExcluded {
				log.Debug().Str("domain", name).Str("peer", peerIP).Str("real_ip", realIP).Str("portal_ip", portalIP).
					Msg("DNS: unauthenticated peer — redirecting internal domain to captive portal")
				resolvedIP = portalIP
				ttl = 1 // TTL=1s so the browser re-queries DNS within 1 second after auth
			}
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
				A:   net.ParseIP(resolvedIP),
			})
			resolved = true
			continue
		}

		// 2. Well-known OS captive-portal probe domains.
		//
		// For A queries: resolve to the jump peer's WireGuard IP so the probe
		// travels through the tunnel and hits the captive portal HTTP server.
		//
		// For AAAA queries: return NODATA to force the OS to fall back to IPv4.
		// Without this, a peer that prefers IPv6 bypasses the captive portal.
		if portalIP != "" {
			if _, ok := captiveProbeHosts[name]; ok {
				if q.Qtype == dns.TypeA {
					log.Debug().Str("domain", name).Str("ip", portalIP).Msg("DNS: intercepting captive portal probe")
					m.Answer = append(m.Answer, &dns.A{
						Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 10},
						A:   net.ParseIP(portalIP),
					})
				} else {
					log.Debug().Str("domain", name).Msg("DNS: suppressing AAAA for captive portal probe (forcing IPv4)")
				}
				resolved = true
				continue
			}
		}
	}

	if resolved {
		_ = w.WriteMsg(m)
		return
	}

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

// LookupPeerIP returns the WireGuard IP for the given hostname (FQDN), or an
// empty string if not found. Exported so the captive portal server can proxy
// authenticated-peer requests directly to the real backend while the browser's
// DNS cache is stale (Firefox ignores TTL=1 and caches for up to 60 s).
func (s *Server) LookupPeerIP(name string) string {
	return s.lookupPeerIP(name)
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
