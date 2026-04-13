package firewall

import (
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strings"
	dom "wirety/agent/internal/domain/policy"

	"github.com/rs/zerolog/log"
)

// Adapter implements dynamic filtering using iptables commands (still kernel-level
// but managed by agent rather than embedded in wg config).
// Simplified: rules applied each sync by flushing dedicated chain.

type Adapter struct {
	iface         string
	natInterfaces []string // explicit override; nil means auto-detect
	httpPort      int
	httpsPort     int
	serverURL     string // Wirety server URL — peers must always be able to reach it
}

// NewAdapter creates a new firewall adapter.
// wgIface: WireGuard interface name (e.g., "wg0")
// natIfaces: explicit NAT interfaces override; nil or empty slice means auto-detect all
func NewAdapter(wgIface string, natIfaces []string) *Adapter {
	return &Adapter{
		iface:         wgIface,
		natInterfaces: natIfaces,
		httpPort:      3128,
		httpsPort:     3129,
	}
}

// SetProxyPorts sets the HTTP and HTTPS proxy ports
func (a *Adapter) SetProxyPorts(httpPort, httpsPort int) {
	a.httpPort = httpPort
	a.httpsPort = httpsPort
}

// EnsureKernelModules loads the kernel modules required for Wirety's iptables rules.
// It is best-effort: a missing module degrades functionality (logged as a warning)
// but never prevents the agent from starting.
//
// Required modules:
//   - nf_conntrack — conntrack state matching (ESTABLISHED/RELATED).
//   - nft_compat   — xtables compatibility layer for iptables-nft; allows xt_string
//     to be used through the nf_tables backend. No-op on legacy iptables.
//   - xt_string    — payload string matching for SNI / Host-header vhost isolation.
//     Works on both legacy iptables and iptables-nft (via nft_compat).
//     If unavailable, Sync() falls back to port-only server ACCEPT rule.
func (a *Adapter) EnsureKernelModules() {
	modules := []struct {
		name    string
		purpose string
	}{
		{"nf_conntrack", "conntrack state matching (ESTABLISHED/RELATED)"},
		{"nft_compat", "xtables compatibility layer for iptables-nft (needed for xt_string on nf_tables backend)"},
		{"xt_string", "payload string matching (SNI / Host-header vhost isolation)"},
	}

	for _, m := range modules {
		if err := exec.Command("modprobe", m.name).Run(); err != nil { // #nosec G204 - static module names
			log.Warn().
				Str("module", m.name).
				Str("purpose", m.purpose).
				Err(err).
				Msg("firewall: failed to load kernel module — functionality may be degraded")
		} else {
			log.Debug().Str("module", m.name).Msg("kernel module loaded")
		}
	}
}

// SetServerURL stores the Wirety server URL so the adapter can add an unconditional
// ACCEPT rule for traffic destined to it — unauthenticated peers must always be able
// to reach the server to complete captive portal authentication.
func (a *Adapter) SetServerURL(serverURL string) {
	a.serverURL = serverURL
}

// serverEndpoint holds the resolved IPs, TCP port, and hostname for the Wirety server.
type serverEndpoint struct {
	ips      []string
	port     string // TCP port string, e.g. "443"
	hostname string // original hostname (used for SNI / Host-header filtering)
	https    bool   // true when the scheme is https
}

// resolveServerEndpoint resolves the Wirety server URL to a list of IP addresses,
// the TCP port, and the original hostname.
//
// The hostname is used for L7 filtering: for HTTPS the TLS SNI is matched (cleartext
// in ClientHello), for HTTP the Host header is matched.  This restricts
// unauthenticated peers to Wirety's virtual host only — other apps served by the same
// reverse-proxy IP:port remain unreachable until captive-portal auth completes.
func (a *Adapter) resolveServerEndpoint() serverEndpoint {
	if a.serverURL == "" {
		return serverEndpoint{}
	}
	u, err := url.Parse(a.serverURL)
	if err != nil {
		log.Warn().Err(err).Str("server_url", a.serverURL).Msg("failed to parse server URL")
		return serverEndpoint{}
	}
	host := u.Hostname()
	if host == "" {
		return serverEndpoint{}
	}

	// Determine the TCP port: explicit in URL, or scheme default.
	port := u.Port()
	isHTTPS := u.Scheme == "https"
	if port == "" {
		if isHTTPS {
			port = "443"
		} else {
			port = "80"
		}
	}

	ep := serverEndpoint{port: port, https: isHTTPS}

	// If the "host" in the URL is already an IP, hostname filtering is not possible
	// (there is no SNI / Host header to match against an IP literal).  Fall back to
	// port-only filtering in that case.
	if net.ParseIP(host) != nil {
		ep.ips = []string{host}
		return ep
	}

	ep.hostname = host
	addrs, err := net.LookupHost(host)
	if err != nil {
		log.Warn().Err(err).Str("host", host).Msg("failed to resolve wirety server hostname")
		return ep
	}
	ep.ips = addrs
	return ep
}

// detectNATInterfaces auto-detects all physical egress interfaces that need a
// MASQUERADE rule by scanning the full routing table. Every unique "dev" entry
// that is not the WireGuard interface, not loopback, and has an IPv4 address is
// considered a NAT interface. This handles multi-homed hosts where different
// destinations exit through different interfaces (e.g. ens2 for internet,
// ens6 for a private VLAN).
func (a *Adapter) detectNATInterfaces() []string {
	cmd := exec.Command("ip", "route", "show") // #nosec G204 - static command
	output, err := cmd.Output()
	if err != nil {
		log.Warn().Err(err).Msg("failed to read routing table, falling back to common interfaces")
		return a.fallbackNATInterfaces()
	}

	seen := make(map[string]bool)
	var ifaces []string

	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.Fields(line)
		for i, part := range parts {
			if part == "dev" && i+1 < len(parts) {
				iface := parts[i+1]
				if iface == "lo" || iface == a.iface || seen[iface] {
					continue
				}
				// Only include interfaces that actually have an IPv4 address
				// (filters out WireGuard tunnels and other virtual interfaces)
				addrOut, addrErr := exec.Command("ip", "addr", "show", iface).Output() // #nosec G204
				if addrErr == nil && strings.Contains(string(addrOut), "inet ") {
					seen[iface] = true
					ifaces = append(ifaces, iface)
				}
			}
		}
	}

	if len(ifaces) == 0 {
		log.Warn().Msg("no NAT interfaces found in routing table, falling back to common interfaces")
		return a.fallbackNATInterfaces()
	}

	log.Info().Strs("interfaces", ifaces).Msg("auto-detected NAT interfaces")
	return ifaces
}

// fallbackNATInterfaces tries common interface names when routing-table detection fails
func (a *Adapter) fallbackNATInterfaces() []string {
	commonInterfaces := []string{"eth0", "ens2", "ens3", "ens6", "ens18", "enp0s3", "wlan0", "wlp2s0"}

	var found []string
	for _, iface := range commonInterfaces {
		cmd := exec.Command("ip", "addr", "show", iface) // #nosec G204 - controlled interface names
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "inet ") {
			log.Info().Str("interface", iface).Msg("using fallback NAT interface")
			found = append(found, iface)
		}
	}

	if len(found) == 0 {
		log.Warn().Msg("no suitable NAT interface found")
	}
	return found
}

// getNATInterfaces returns the NAT interfaces to use: the explicit override list
// if configured, otherwise all auto-detected egress interfaces.
func (a *Adapter) getNATInterfaces() []string {
	if len(a.natInterfaces) > 0 {
		log.Debug().Strs("interfaces", a.natInterfaces).Msg("using configured NAT interfaces")
		return a.natInterfaces
	}

	detected := a.detectNATInterfaces()
	if len(detected) == 0 {
		log.Warn().Msg("no NAT interfaces available - peers will not have internet/routed access")
	}
	return detected
}

// EnableDebugLogging adds LOG rules to help debug packet drops
func (a *Adapter) EnableDebugLogging() error {
	chain := "WIRETY_JUMP"
	// Add LOG rule at the beginning of the chain to log all packets
	if err := a.run("-I", chain, "1", "-j", "LOG", "--log-prefix", "WIRETY-DEBUG: ", "--log-level", "4"); err != nil {
		return fmt.Errorf("failed to add debug logging: %w", err)
	}
	log.Info().Msg("iptables debug logging enabled - check /var/log/kern.log or dmesg")
	return nil
}

func (a *Adapter) run(args ...string) error {
	cmd := exec.Command("iptables", args...) // #nosec G204
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables %v failed: %v output=%s", args, err, string(out))
	}
	return nil
}

// runIPv6 runs an ip6tables command
// func (a *Adapter) runIPv6(args ...string) error {
// 	cmd := exec.Command("ip6tables", args...) // #nosec G204
// 	out, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return fmt.Errorf("ip6tables %v failed: %v output=%s", args, err, string(out))
// 	}
// 	return nil
// }

// runIfNotExists runs an iptables command only if the exact rule doesn't already
// exist. It uses `iptables -C` (the built-in check command) which returns exit
// code 0 when the rule is present, matching on every parameter — not just chain
// and target. This avoids the false-positive bug where a rule like
// `-o ens2 -j MASQUERADE` would mask a distinct rule `-o ens6 -j MASQUERADE`.
func (a *Adapter) runIfNotExists(args ...string) error {
	checkArgs := toCheckArgs(args)
	if exec.Command("iptables", checkArgs...).Run() == nil { // #nosec G204
		return nil // exact rule already present
	}
	return a.run(args...)
}

// toCheckArgs converts -A/-I arguments to their -C (check) equivalent.
// `iptables -C` does not accept a position number, so for `-I CHAIN N …` the
// position N is dropped, producing `-C CHAIN …`.
func toCheckArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-A":
			out = append(out, "-C")
		case "-I":
			out = append(out, "-C")
			// -I CHAIN [position] → -C CHAIN  (skip numeric position if present)
			if i+2 < len(args) && isPositiveInt(args[i+2]) {
				out = append(out, args[i+1]) // chain name
				i += 2                       // skip chain name + position
			}
		default:
			out = append(out, args[i])
		}
	}
	return out
}

func isPositiveInt(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// applyIPTablesRule parses and applies a single iptables rule to the specified chain
// The rule string should be in the format: "iptables -A CHAIN [options]"
// We extract the options and apply them to our custom chain
func (a *Adapter) applyIPTablesRule(chain, rule string) error {
	// Parse the rule string to extract iptables arguments
	// Expected format: "iptables -A CHAIN [options]" or just the options part
	// We'll replace any chain reference with our custom chain

	// Split the rule into tokens
	tokens := strings.Fields(rule)
	if len(tokens) == 0 {
		return fmt.Errorf("empty iptables rule")
	}

	// Build the arguments for our iptables command
	args := make([]string, 0, len(tokens)+2)

	// Skip "iptables" if it's the first token
	startIdx := 0
	if tokens[0] == "iptables" {
		startIdx = 1
	}

	// Look for -A or -I and replace the chain name
	foundChain := false
	for i := startIdx; i < len(tokens); i++ {
		if tokens[i] == "-A" || tokens[i] == "-I" {
			args = append(args, "-A") // Always use -A for appending
			if i+1 < len(tokens) {
				// Skip the original chain name and use our custom chain
				i++
				foundChain = true
			}
			args = append(args, chain)
		} else {
			args = append(args, tokens[i])
		}
	}

	// If no chain was specified, prepend -A CHAIN
	if !foundChain {
		args = append([]string{"-A", chain}, args...)
	}

	// Apply the rule
	if err := a.run(args...); err != nil {
		return fmt.Errorf("failed to apply rule: %w", err)
	}

	log.Debug().Str("rule", rule).Strs("args", args).Msg("applied iptables rule")
	return nil
}

// Sync applies forwarding/NAT plus policy-based iptables rules.
// This method is called periodically when policy updates are received.
// To avoid dropping active connections, we check if rules exist before adding them.
func (a *Adapter) Sync(p *dom.JumpPolicy, selfIP string, whitelistedIPs []string) error {
	if p == nil {
		return nil
	}
	// Ensure IP forwarding enabled
	if err := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run(); err != nil {
		log.Warn().Err(err).Msg("failed enabling ip_forward")
	}

	// Enable IPv6 forwarding for dual-stack support
	if err := exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1").Run(); err != nil {
		log.Warn().Err(err).Msg("failed enabling ipv6 forwarding")
	}

	// TODO: Apply IPv6 firewall rules (ip6tables) based on policies
	// For now, IPv6 traffic is allowed but not filtered by policies
	// This should be implemented as part of full dual-stack support

	// ── Two-chain design ────────────────────────────────────────────────────
	//
	// WIRETY_JUMP (authentication gate, in FORWARD):
	//   0. ESTABLISHED/RELATED   → ACCEPT  (conntrack: ongoing sessions pass through)
	//   1. Wirety server IPs     → ACCEPT  (only Wirety port+hostname; captive portal reachable)
	//   2. Whitelisted peer IP   → jump to WIRETY_POLICY
	//   3. Everything else       → DROP    (unauthenticated peers blocked on ALL ports)
	//
	// WIRETY_POLICY (per-destination rules, only reached by authenticated peers):
	//   1. Explicit policy rules (ACCEPT/DROP per destination CIDR)
	//   2. Catch-all ACCEPT    (backward compat: authenticated = full access by default)
	//
	// This prevents unauthenticated peers from bypassing the captive portal via HTTPS
	// or any other port that is not intercepted by the DNAT rule: they hit the DROP in
	// WIRETY_JUMP before any policy rule is ever evaluated.
	//
	// When multiple virtual hosts share the same reverse-proxy IP:port, L7 hostname
	// filtering is applied:
	//   - HTTPS: iptables string-matches the TLS SNI (cleartext in ClientHello)
	//   - HTTP:  iptables string-matches the "Host:" header
	// Only the Wirety virtual host is reachable before authentication; other vhosts
	// remain blocked. Subsequent packets in an accepted TCP session are allowed via
	// the ESTABLISHED/RELATED rule without re-checking the hostname.

	chain := "WIRETY_JUMP"
	policyChain := "WIRETY_POLICY"

	_ = a.run("-N", chain)
	_ = a.run("-F", chain)
	_ = a.run("-N", policyChain)
	_ = a.run("-F", policyChain)

	// Rule 0: allow packets belonging to already-established connections.
	// Required because string matching (SNI / Host header) only works on the first
	// packet of a TCP handshake; subsequent packets carry no hostname and would
	// otherwise be dropped.  Conntrack is available on all modern Linux kernels.
	_ = a.run("-A", chain, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT")

	// Rule 1: allow peers to reach the Wirety server so they can complete captive
	// portal authentication.  Filtering is applied in three layers:
	//   a) destination IP  — resolved from the server URL at sync time
	//   b) destination port — derived from the URL scheme (80 / 443 / explicit)
	//   c) hostname         — SNI for HTTPS, Host header for HTTP
	// Layer (c) prevents other virtual hosts on the same reverse-proxy from being
	// reachable. If the server URL uses a bare IP (no hostname), only (a)+(b) apply.
	endpoint := a.resolveServerEndpoint()
	for _, ip := range endpoint.ips {
		base := []string{"-A", chain, "-i", a.iface, "-d", ip, "-p", "tcp", "--dport", endpoint.port}

		if endpoint.hostname != "" {
			// Build the string-match pattern:
			//   HTTPS → match raw SNI bytes in TLS ClientHello (appears as the plain
			//           hostname string inside the extension payload)
			//   HTTP  → match "Host: <hostname>" header line
			var pattern string
			if endpoint.https {
				pattern = endpoint.hostname
			} else {
				pattern = "Host: " + endpoint.hostname
			}

			rule := append(append([]string{}, base...),
				"-m", "string", "--algo", "bm", "--string", pattern, "--to", "65535",
				"-j", "ACCEPT")
			if err := a.run(rule...); err != nil {
				// xt_string loaded but rule still failed — fall back to port-only.
				log.Warn().Err(err).
					Str("ip", ip).Str("port", endpoint.port).Str("hostname", endpoint.hostname).
					Msg("string match rule failed — falling back to port-only server ACCEPT rule (other vhosts on same IP:port will be reachable before auth)")
				fallback := append(append([]string{}, base...), "-j", "ACCEPT")
				_ = a.run(fallback...)
			}
		} else {
			// Bare-IP server URL: no hostname to match against; use port-only rule.
			rule := append(append([]string{}, base...), "-j", "ACCEPT")
			if err := a.run(rule...); err != nil {
				log.Warn().Err(err).Str("ip", ip).Str("port", endpoint.port).Msg("failed to add Wirety server ACCEPT rule")
			}
		}
	}

	// Authenticated peers jump to the policy chain; all others hit the DROP below.
	for _, ip := range whitelistedIPs {
		if err := a.run("-A", chain, "-i", a.iface, "-s", ip, "-j", policyChain); err != nil {
			log.Warn().Err(err).Str("ip", ip).Msg("failed to add whitelist jump rule")
		}
	}

	// For unauthenticated peers, reject HTTPS (443) with a TCP RST so the
	// browser fails immediately instead of spinning for ~30 s on a silent DROP.
	// This also nudges the OS captive-portal detection flow: the HTTP probes
	// that every modern OS (iOS, Android, Windows, macOS) sends on network join
	// are plain HTTP — they hit the DNAT rule above and reach the captive portal.
	// Note: a full HTTPS → captive-portal redirect is not feasible because it
	// requires TLS termination with a valid certificate; self-signed certs cause
	// browser warnings and HSTS-preloaded sites (most major domains) refuse to
	// show the warning at all.
	_ = a.run("-A", chain, "-i", a.iface, "-p", "tcp", "--dport", "443", "-j", "REJECT", "--reject-with", "tcp-reset")
	_ = a.run("-A", chain, "-i", a.iface, "-j", "DROP")

	// Populate WIRETY_POLICY with per-destination rules for authenticated peers.
	//
	// When policy rules are present they define the complete access control for
	// authenticated peers — typically an allowlist ending with a catch-all DROP.
	// In that case we must NOT append a catch-all ACCEPT after those rules,
	// because it would be unreachable (dead code after the DROP) or, if inserted
	// first, would bypass the policy entirely.
	//
	// When no policy rules are present we add a catch-all ACCEPT to preserve
	// backward-compat behaviour: being on the whitelist implies full access.
	if len(p.IPTablesRules) > 0 {
		log.Info().Int("rule_count", len(p.IPTablesRules)).Msg("applying policy-based iptables rules")
		for i, rule := range p.IPTablesRules {
			if err := a.applyIPTablesRule(policyChain, rule); err != nil {
				log.Error().Err(err).Int("rule_index", i).Str("rule", rule).Msg("failed to apply iptables rule")
			}
		}
		log.Debug().Msg("policy rules applied; default verdict determined by policy")
	} else {
		// No policy configured — authenticated peer gets full access (legacy behaviour).
		_ = a.run("-A", policyChain, "-j", "ACCEPT")
		log.Debug().Msg("no policy rules — catch-all ACCEPT applied (full access for authenticated peers)")
	}

	// Remove legacy WIRETY_CAPTIVE chain if present from a previous agent version
	// that used DNAT to redirect port-80 traffic to a localhost port.
	// The captive portal HTTP server now listens directly on the WireGuard interface.
	_ = a.run("-t", "nat", "-D", "PREROUTING", "-i", a.iface, "-j", "WIRETY_CAPTIVE")
	_ = a.run("-t", "nat", "-F", "WIRETY_CAPTIVE")
	_ = a.run("-t", "nat", "-X", "WIRETY_CAPTIVE")

	// Attach chain to FORWARD (insert at top, only if not already attached)
	_ = a.runIfNotExists("-I", "FORWARD", "1", "-j", chain)

	// Allow peers to reach the services running on the jump peer itself.
	// Traffic from a peer to the jump peer's own WG IP goes through the INPUT
	// chain — not FORWARD — so WIRETY_JUMP never sees it. On servers with a
	// restrictive INPUT policy (UFW default-deny, firewalld, etc.) these packets
	// are silently dropped before reaching the HTTP or DNS server.
	//
	//   Port 80  — captive portal HTTP server (redirect / probe-success)
	//   Port 53  — DNS server (probe domain interception + peer name resolution)
	//
	// These rules are inserted idempotently and must come before any DROP rule
	// that the host firewall may have added to the INPUT chain.
	_ = a.runIfNotExists("-I", "INPUT", "1", "-i", a.iface, "-p", "tcp", "--dport", "80", "-j", "ACCEPT")
	_ = a.runIfNotExists("-I", "INPUT", "1", "-i", a.iface, "-p", "tcp", "--dport", "443", "-j", "ACCEPT")
	_ = a.runIfNotExists("-I", "INPUT", "1", "-i", a.iface, "-p", "udp", "--dport", "53", "-j", "ACCEPT")
	_ = a.runIfNotExists("-I", "INPUT", "1", "-i", a.iface, "-p", "tcp", "--dport", "53", "-j", "ACCEPT")

	// MASQUERADE on every egress interface so that forwarded traffic is NATed
	// regardless of which interface the routing table selects for a given destination.
	// Example: ens2 for internet, ens6 for a private VLAN — both need MASQUERADE.
	natIfaces := a.getNATInterfaces()
	if len(natIfaces) == 0 {
		log.Info().Msg("no NAT interfaces configured — peers will not have routed access")
	}
	for _, natIface := range natIfaces {
		if err := a.runIfNotExists("-t", "nat", "-A", "POSTROUTING", "-o", natIface, "-j", "MASQUERADE"); err != nil {
			log.Warn().Err(err).Str("interface", natIface).Msg("failed to add MASQUERADE rule")
		} else {
			log.Debug().Str("interface", natIface).Msg("MASQUERADE rule configured")
		}
	}
	return nil
}
