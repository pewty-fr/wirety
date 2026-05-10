// Package ra implements an ICMPv6 Router Advertisement emitter that includes
// the Captive-Portal option defined by RFC 8910 (option type 37).
//
// The goal is to make captive-portal-aware clients — most notably Android 14+
// (which honours the Captive Portal API custom-tabs flow) — discover the URL
// of our RFC 8908 captive-portal API endpoint via the IPv6 router solicitation
// it sends when joining a network.  Without this discovery channel, the client
// only finds out about a captive portal via the legacy HTTP probe heuristic
// (connectivitycheck.gstatic.com / captive.apple.com), which is unreliable
// on VPN interfaces and can be bypassed by Private DNS / DoH.
//
// Implementation notes:
//
//   - We send unsolicited RAs every `interval` (default 30 s) and respond to
//     Router Solicitations (ICMPv6 type 133) immediately.  The destination
//     for unsolicited is ff02::1 (all-nodes), for solicited it's the source
//     of the RS.
//
//   - The agent runs as root (it manages iptables) so we have CAP_NET_RAW —
//     no extra capabilities needed.
//
//   - RFC 4861 specifies that RAs SHOULD originate from the link-local address
//     of the sending interface.  WireGuard interfaces don't get a link-local
//     auto-assigned (they're `link/none`), so we add `fe80::1/64` to the
//     interface ourselves before binding.  This is idempotent — if a
//     link-local already exists we use it.
//
//   - The Router Lifetime field is set to 0.  RFC 8910 §3 explicitly permits
//     this: the option's purpose is purely to advertise the captive-portal
//     URL, not to claim default-router status (the WG peer is already a
//     hop, not a "router on the link" in the IPv6 sense).
//
//   - Hop Limit on the IPv6 packet MUST be 255 (RFC 4861 §6.1.2) — we set
//     this via the IPV6_UNICAST_HOPS / IPV6_MULTICAST_HOPS socket options.
package ra

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"time"

	"golang.org/x/net/ipv6"

	"github.com/rs/zerolog/log"
)

// Captive-Portal RA option type assigned by RFC 8910.
const captivePortalOptionType uint8 = 37

// allNodesMulticast is the IPv6 link-scope all-nodes multicast address (ff02::1).
// Unsolicited RAs are sent here; every IPv6 host on the link processes them.
var allNodesMulticast = net.ParseIP("ff02::1")

// fallbackLinkLocal is the address we add to the WG interface if it has no
// link-local of its own.  WG `link/none` interfaces don't get one auto-assigned.
const fallbackLinkLocalCIDR = "fe80::1/64"

// Emitter sends Router Advertisements containing the Captive-Portal option
// on the configured interface.
type Emitter struct {
	iface     string
	portalURL string
	interval  time.Duration

	mu      sync.Mutex
	stop    chan struct{}
	running bool
}

// NewEmitter constructs a new emitter.  iface is the WireGuard interface name
// (e.g. "jump"); portalURL is the captive-portal API URL to advertise (e.g.
// "https://[fd1d:7f8c:e149:fc0c::1]/api/captive-portal").  Per RFC 8908 the URL
// SHOULD be HTTPS — clients that strictly enforce this will reject http:// URLs.
func NewEmitter(iface, portalURL string) *Emitter {
	return &Emitter{
		iface:     iface,
		portalURL: portalURL,
		interval:  30 * time.Second,
	}
}

// SetInterval overrides the unsolicited-RA interval (default 30 s).
func (e *Emitter) SetInterval(d time.Duration) {
	if d > 0 {
		e.interval = d
	}
}

// Start begins emitting RAs in background goroutines.  Returns an error if the
// initial setup fails (interface lookup, link-local provisioning, socket bind).
// Subsequent send failures are logged but do not stop the emitter.
func (e *Emitter) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return nil
	}

	if e.portalURL == "" {
		return errors.New("portal URL not configured")
	}

	// Make sure the WG interface has a link-local address so RAs sourced from
	// it are RFC-compliant.
	if err := e.ensureLinkLocal(); err != nil {
		return fmt.Errorf("ensure link-local on %s: %w", e.iface, err)
	}

	iface, err := net.InterfaceByName(e.iface)
	if err != nil {
		return fmt.Errorf("interface %s not found: %w", e.iface, err)
	}

	// Open a raw ICMPv6 socket bound to the interface.  We use the lower-level
	// net.ListenPacket("ip6:ipv6-icmp", ...) instead of icmp.ListenPacket so
	// we can write custom ICMPv6 type 134 (RA) bodies; icmp.Message doesn't
	// know about RA fields.
	conn, err := net.ListenPacket("ip6:ipv6-icmp", "::")
	if err != nil {
		return fmt.Errorf("listen ICMPv6: %w", err)
	}

	pc := ipv6.NewPacketConn(conn)
	// RFC 4861 §6.1.2 — hop limit on outgoing RAs MUST be 255.
	if err := pc.SetMulticastHopLimit(255); err != nil {
		log.Warn().Err(err).Msg("ra: SetMulticastHopLimit failed")
	}
	if err := pc.SetHopLimit(255); err != nil {
		log.Warn().Err(err).Msg("ra: SetHopLimit failed")
	}
	// Restrict outgoing multicast to the WG interface (otherwise it goes out
	// the default route — which would leak our captive portal URL onto the
	// public network!).
	if err := pc.SetMulticastInterface(iface); err != nil {
		log.Warn().Err(err).Msg("ra: SetMulticastInterface failed")
	}
	if err := pc.SetMulticastLoopback(false); err != nil {
		log.Warn().Err(err).Msg("ra: SetMulticastLoopback failed")
	}
	// Filter inbound traffic to Router Solicitations only — we don't care
	// about anything else on this socket.
	var f ipv6.ICMPFilter
	f.SetAll(true)
	f.Accept(ipv6.ICMPTypeRouterSolicitation)
	if err := pc.SetICMPFilter(&f); err != nil {
		log.Warn().Err(err).Msg("ra: SetICMPFilter failed")
	}
	// We need the receive-control-message data to know which interface
	// solicitations arrived on (so we don't reply to RS from other interfaces).
	if err := pc.SetControlMessage(ipv6.FlagInterface|ipv6.FlagSrc, true); err != nil {
		log.Warn().Err(err).Msg("ra: SetControlMessage failed")
	}

	e.stop = make(chan struct{})
	e.running = true

	go e.tickLoop(pc, iface)
	go e.solicitLoop(pc, iface)

	log.Info().
		Str("iface", e.iface).
		Str("portal_url", e.portalURL).
		Dur("interval", e.interval).
		Msg("ra: emitter started — advertising captive portal URL via RA option 37")

	return nil
}

// Stop halts the emitter.  Safe to call from any goroutine.
func (e *Emitter) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	close(e.stop)
	e.running = false
}

// ensureLinkLocal makes sure the configured interface has at least one IPv6
// link-local address (fe80::/10).  WireGuard interfaces are `link/none` so the
// kernel does NOT auto-generate one — without manual provisioning the RA
// sender has no compliant source address.
func (e *Emitter) ensureLinkLocal() error {
	iface, err := net.InterfaceByName(e.iface)
	if err != nil {
		return err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return err
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ipnet.IP.To4() == nil && ipnet.IP.IsLinkLocalUnicast() {
			log.Debug().Str("iface", e.iface).Str("addr", ipnet.IP.String()).Msg("ra: link-local already present")
			return nil
		}
	}
	// Add fe80::1/64.  Best-effort: if it's already there (race with another
	// agent run), `ip` returns "File exists" which we treat as success.
	cmd := exec.Command("ip", "-6", "addr", "add", fallbackLinkLocalCIDR, "dev", e.iface) // #nosec G204 - iface validated
	out, err := cmd.CombinedOutput()
	if err != nil && !bytes.Contains(out, []byte("File exists")) {
		return fmt.Errorf("ip -6 addr add: %v: %s", err, bytes.TrimSpace(out))
	}
	log.Info().Str("iface", e.iface).Str("addr", fallbackLinkLocalCIDR).Msg("ra: provisioned link-local address on WG interface")
	return nil
}

// tickLoop sends an unsolicited RA every `interval`.
func (e *Emitter) tickLoop(pc *ipv6.PacketConn, iface *net.Interface) {
	defer pc.Close()
	// Send one immediately so listeners that joined just before us learn the
	// option without waiting `interval`.
	if err := e.sendRA(pc, iface, allNodesMulticast); err != nil {
		log.Warn().Err(err).Msg("ra: initial unsolicited RA failed")
	}

	t := time.NewTicker(e.interval)
	defer t.Stop()
	for {
		select {
		case <-e.stop:
			return
		case <-t.C:
			if err := e.sendRA(pc, iface, allNodesMulticast); err != nil {
				log.Warn().Err(err).Msg("ra: unsolicited RA failed")
			}
		}
	}
}

// solicitLoop reads Router Solicitations on the WG interface and replies with
// a unicast RA to the source.  Per RFC 4861 §6.2.6, replies SHOULD be unicast
// to the solicitor unless the source is unspecified (in which case we
// multicast).
func (e *Emitter) solicitLoop(pc *ipv6.PacketConn, iface *net.Interface) {
	buf := make([]byte, 1500)
	for {
		select {
		case <-e.stop:
			return
		default:
		}
		_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, cm, src, err := pc.ReadFrom(buf)
		if err != nil {
			if isTimeout(err) {
				continue
			}
			log.Debug().Err(err).Msg("ra: ReadFrom error")
			continue
		}
		if n < 1 || buf[0] != byte(ipv6.ICMPTypeRouterSolicitation) {
			continue
		}
		// Drop packets that arrived on a different interface (we only care
		// about RS from peers on the WG interface).
		if cm != nil && cm.IfIndex != iface.Index {
			continue
		}
		dst := allNodesMulticast
		if udpAddr, ok := src.(*net.IPAddr); ok {
			if !udpAddr.IP.IsUnspecified() {
				dst = udpAddr.IP
			}
		}
		log.Debug().Str("from", src.String()).Msg("ra: received Router Solicitation, sending unicast RA")
		if err := e.sendRA(pc, iface, dst); err != nil {
			log.Warn().Err(err).Msg("ra: solicited RA failed")
		}
	}
}

// sendRA constructs and writes a single Router Advertisement to `dst`.
// dst is either ff02::1 (all-nodes multicast, periodic) or the source of an
// RS we received (unicast reply).
func (e *Emitter) sendRA(pc *ipv6.PacketConn, iface *net.Interface, dst net.IP) error {
	body := buildRA(e.portalURL)
	msg := append([]byte{
		134, // ICMPv6 Type: Router Advertisement
		0,   // Code
		0, 0, // Checksum (kernel computes for raw ICMPv6 sockets)
	}, body...)

	addr := &net.IPAddr{IP: dst, Zone: iface.Name}
	if _, err := pc.WriteTo(msg, nil, addr); err != nil {
		return err
	}
	return nil
}

// buildRA returns the bytes that follow the ICMPv6 type/code/checksum header
// for a Router Advertisement (RFC 4861 §4.2) plus the Captive-Portal option
// (RFC 8910).  All RA-specific fields are zeroed:
//
//   - Cur Hop Limit = 0   (use system default)
//   - Flags M, O = 0       (no managed/other config)
//   - Router Lifetime = 0  (we are NOT a default router)
//   - Reachable Time = 0
//   - Retrans Timer = 0
func buildRA(portalURL string) []byte {
	var b bytes.Buffer
	// Cur Hop Limit, Flags, Router Lifetime, Reachable, Retrans (12 bytes total)
	header := make([]byte, 12)
	b.Write(header)
	b.Write(encodeCaptivePortalOption(portalURL))
	return b.Bytes()
}

// encodeCaptivePortalOption serialises an RFC 8910 Captive-Portal option:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Type      |     Length    |          URI                  |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+                               |
//	~                          NUL-padded to 8-byte boundary        ~
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Length is the total option length in 8-byte units (so it includes the
// type/length bytes themselves).  The URI field is padded with NUL bytes so
// the option ends on an 8-byte boundary.
func encodeCaptivePortalOption(uri string) []byte {
	minLen := 2 + len(uri)
	paddedLen := ((minLen + 7) / 8) * 8
	if paddedLen > 255*8 {
		// Length field is one byte (in 8-byte units) so 255*8 = 2040 bytes max.
		// Truncate (this should never happen for sensible URLs).
		paddedLen = 255 * 8
	}
	buf := make([]byte, paddedLen)
	buf[0] = captivePortalOptionType
	buf[1] = byte(paddedLen / 8)
	copy(buf[2:], uri)
	return buf
}

// isTimeout returns true if err is a deadline-exceeded error from ReadFrom.
func isTimeout(err error) bool {
	type timeoutError interface {
		Timeout() bool
	}
	var t timeoutError
	if errors.As(err, &t) {
		return t.Timeout()
	}
	return false
}
