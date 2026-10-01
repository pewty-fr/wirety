package firewall

import (
	"fmt"
	"strconv"
	"strings"
)

// HostPort is a port (or port range) of the jump host itself that signed-in
// peers may reach through the tunnel, such as 22/tcp for SSH.
type HostPort struct {
	Proto string // "tcp" or "udp"
	Port  string // "22" or a range "60000:61000" (iptables syntax)
}

func (p HostPort) String() string { return p.Port + "/" + p.Proto }

// ParseHostPorts parses a comma-separated list of "port[/proto]" or
// "from-to[/proto]" entries; the protocol defaults to tcp.
// Example: "22,9100/tcp,60000-61000/udp".
func ParseHostPorts(s string) ([]HostPort, error) {
	var out []HostPort
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		spec, proto, hasProto := strings.Cut(entry, "/")
		if !hasProto {
			proto = "tcp"
		}
		proto = strings.ToLower(proto)
		if proto != "tcp" && proto != "udp" {
			return nil, fmt.Errorf("%q: protocol must be tcp or udp", entry)
		}
		from, to, isRange := strings.Cut(spec, "-")
		lo, err := parsePort(from)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", entry, err)
		}
		port := strconv.Itoa(lo)
		if isRange {
			hi, err := parsePort(to)
			if err != nil {
				return nil, fmt.Errorf("%q: %w", entry, err)
			}
			if hi < lo {
				return nil, fmt.Errorf("%q: empty range", entry)
			}
			port += ":" + strconv.Itoa(hi)
		}
		out = append(out, HostPort{Proto: proto, Port: port})
	}
	return out, nil
}

func parsePort(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > 65535 {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	return n, nil
}

// SetHostPorts sets the ports of the jump host itself that signed-in peers may
// reach through the tunnel. Everything else arriving on the WireGuard
// interface for this host is dropped, except the services peers need to sign
// in (see syncInput).
func (a *Adapter) SetHostPorts(ports []HostPort) {
	a.hostPorts = ports
}

// syncInput (re)builds chain, which filters what peers reach on the jump host
// itself through the WireGuard interface (INPUT). The jump's own services —
// sshd, exporters, databases… — are not governed by the policies, which only
// cover what is routed through the jump: without this chain they would be open
// to every device holding a WireGuard config, signed in or not.
//
//   - replies to connections the jump opened (--ctdir REPLY)
//   - every peer: DNS (53), captive portal (80), HTTPS (443, answered with a
//     reset so browsers fail fast), SNI proxy, the Wirety server when it runs
//     on this host, ping
//   - signed-in peers: the operator's host ports (-jump-host-ports), checked on
//     every packet so a sign-out also cuts an open SSH session
//   - everything else: DROP
//
// Only packets arriving on the WireGuard interface go through the chain: the
// host's other interfaces (private network, bastion, console) are never
// filtered, so the jump stays reachable over SSH from there whatever happens
// on the tunnel side.
func (a *Adapter) syncInput(run, runIfNotExists func(...string) error, chain, icmp string, whitelist []string, ep serverEndpoint, serverIPs []string) {
	if a.iface == "" {
		return
	}
	authChain := chain + "_AUTH"
	for _, c := range []string{chain, authChain} {
		_ = run("-N", c)
		_ = run("-F", c)
	}

	_ = run("-A", chain, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "--ctdir", "REPLY", "-j", "ACCEPT")
	for _, svc := range [][2]string{{"udp", "53"}, {"tcp", "53"}, {"tcp", "80"}, {"tcp", "443"}} {
		_ = run("-A", chain, "-p", svc[0], "--dport", svc[1], "-j", "ACCEPT")
	}
	if a.sniProxy {
		_ = run("-A", chain, "-p", "tcp", "--dport", strconv.Itoa(a.httpsPort), "-j", "ACCEPT")
	}
	for _, ip := range serverIPs {
		_ = run("-A", chain, "-d", ip, "-p", "tcp", "--dport", ep.port, "-j", "ACCEPT")
	}
	_ = run("-A", chain, "-p", icmp, "--"+icmp+"-type", "echo-request", "-j", "ACCEPT")

	if len(a.hostPorts) > 0 {
		for _, p := range a.hostPorts {
			_ = run("-A", authChain, "-p", p.Proto, "--dport", p.Port, "-j", "ACCEPT")
		}
		for _, ip := range whitelist {
			_ = run("-A", chain, "-s", ip, "-j", authChain)
		}
	}
	_ = run("-A", chain, "-j", "DROP")

	_ = runIfNotExists("-I", "INPUT", "1", "-i", a.iface, "-j", chain)
	a.removeLegacyInputRules(run)
}

// removeLegacyInputRules deletes the per-port ACCEPT rules that previous agent
// versions inserted directly in INPUT, now part of the WIRETY_INPUT chains.
func (a *Adapter) removeLegacyInputRules(run func(...string) error) {
	legacy := [][2]string{{"tcp", "80"}, {"tcp", "443"}, {"udp", "53"}, {"tcp", "53"}, {"tcp", strconv.Itoa(a.httpsPort)}}
	for _, r := range legacy {
		// A rule may have been inserted more than once; -D removes one copy.
		for i := 0; i < 5; i++ {
			if run("-D", "INPUT", "-i", a.iface, "-p", r[0], "--dport", r[1], "-j", "ACCEPT") != nil {
				break
			}
		}
	}
}
