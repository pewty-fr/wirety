package firewall

import (
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	dom "wirety/agent/internal/domain/policy"
	"wirety/agent/internal/ports"

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
	ips      []string // IPv4 addresses
	ipsv6    []string // IPv6 addresses
	port     string   // TCP port string, e.g. "443"
	hostname string   // original hostname (used for SNI / Host-header filtering)
	https    bool     // true when the scheme is https
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

	// Separate IPv4 and IPv6 addresses — each is applied to the corresponding
	// iptables / ip6tables chain respectively.
	var ipv4s, ipv6s []string
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			ipv4s = append(ipv4s, addr)
		} else {
			ipv6s = append(ipv6s, addr)
		}
	}
	if len(ipv4s) == 0 && len(ipv6s) > 0 {
		log.Warn().
			Str("host", host).
			Strs("ipv6_only", ipv6s).
			Msg("wirety server hostname resolved to IPv6 addresses only — no iptables ACCEPT rule added; unauthenticated peers may not reach the server via IPv4")
	}
	ep.ips = ipv4s
	ep.ipsv6 = ipv6s
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

// detectNATInterfacesIPv6 auto-detects egress interfaces that have an IPv6
// address (for ip6tables MASQUERADE rules).  Uses `ip -6 route show` to find
// interfaces with a default IPv6 route, then filters those that actually carry
// a global-scope IPv6 address (not just link-local).
func (a *Adapter) detectNATInterfacesIPv6() []string {
	cmd := exec.Command("ip", "-6", "route", "show") // #nosec G204
	output, err := cmd.Output()
	if err != nil {
		log.Debug().Err(err).Msg("failed to read IPv6 routing table — IPv6 MASQUERADE skipped")
		return nil
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
				// Only include interfaces that carry a global-scope IPv6 address.
				//
				// We use `ip -6 addr show scope global dev <iface>` which returns
				// EMPTY output on link-local-only interfaces and at least one
				// `inet6` line on globally addressed ones.  The previous heuristic
				// — check for `inet6 ` AND absence of `inet6 fe80` — was broken
				// because every IPv6-enabled interface carries BOTH a link-local
				// fe80:: and (typically) a global address.  The negative check
				// always tripped, no interface was ever picked, and IPv6
				// MASQUERADE was silently never installed.  Result: forwarded
				// IPv6 traffic from peers (ULA source) reached the egress NIC
				// unmodified and was discarded upstream as un-routable.
				addrOut, addrErr := exec.Command("ip", "-6", "addr", "show", "scope", "global", "dev", iface).Output() // #nosec G204
				if addrErr == nil && strings.Contains(string(addrOut), "inet6 ") {
					seen[iface] = true
					ifaces = append(ifaces, iface)
				}
			}
		}
	}

	if len(ifaces) > 0 {
		log.Info().Strs("interfaces", ifaces).Msg("auto-detected IPv6 NAT interfaces")
	} else {
		log.Warn().Msg("no IPv6 NAT interfaces detected — IPv6 MASQUERADE not configured; ULA peers will have no IPv6 internet egress")
	}

	return ifaces
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

// runIPv6 runs an ip6tables command (mirrors run for IPv6).
func (a *Adapter) runIPv6(args ...string) error {
	cmd := exec.Command("ip6tables", args...) // #nosec G204
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip6tables %v failed: %v output=%s", args, err, string(out))
	}
	return nil
}

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

// runIPv6IfNotExists is the ip6tables equivalent of runIfNotExists.
func (a *Adapter) runIPv6IfNotExists(args ...string) error {
	checkArgs := toCheckArgs(args)
	if exec.Command("ip6tables", checkArgs...).Run() == nil { // #nosec G204
		return nil // exact rule already present
	}
	return a.runIPv6(args...)
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

// parseRuleFamily classifies a rule string into the (iptables, ip6tables, unknown)
// space.  Returns "ip6tables" if the rule starts with "ip6tables", otherwise
// "iptables" (default — handles bare-options rules and explicit "iptables" prefix).
func parseRuleFamily(rule string) string {
	tokens := strings.Fields(rule)
	if len(tokens) > 0 && tokens[0] == "ip6tables" {
		return "ip6tables"
	}
	return "iptables"
}

// applyIPTablesRule parses and applies a single iptables / ip6tables rule to the
// specified chain.  The rule string is in one of these forms:
//   - "iptables -A CHAIN [options]"   → applied to iptables (IPv4)
//   - "ip6tables -A CHAIN [options]"  → applied to ip6tables (IPv6)
//   - "[options]"                     → applied as iptables (legacy default)
//
// `family` selects which table this call is allowed to touch:
//   - "iptables"  → only iptables rules; ip6tables rules are skipped
//   - "ip6tables" → only ip6tables rules; iptables rules are skipped
//   - ""          → auto-detect from the prefix (defaults to iptables for bare rules)
//
// The chain reference in the rule is rewritten to the supplied `chain`.
func (a *Adapter) applyIPTablesRule(chain, rule, family string) error {
	tokens := strings.Fields(rule)
	if len(tokens) == 0 {
		return fmt.Errorf("empty iptables rule")
	}

	// Detect the rule's native family from its prefix.
	ruleFamily := "iptables"
	startIdx := 0
	if tokens[0] == "iptables" {
		startIdx = 1
	} else if tokens[0] == "ip6tables" {
		ruleFamily = "ip6tables"
		startIdx = 1
	}

	// If the caller restricted to a specific family, skip rules from the other.
	if family != "" && family != ruleFamily {
		log.Debug().Str("rule", rule).Str("rule_family", ruleFamily).Str("call_family", family).Msg("rule skipped (family mismatch)")
		return nil
	}

	// Build the arguments for the iptables/ip6tables command.
	args := make([]string, 0, len(tokens)+2)

	// Look for -A or -I and replace the chain name with the supplied one.
	foundChain := false
	for i := startIdx; i < len(tokens); i++ {
		if tokens[i] == "-A" || tokens[i] == "-I" {
			args = append(args, "-A") // Always append (the chain is freshly flushed each sync)
			if i+1 < len(tokens) {
				i++ // skip the original chain name
				foundChain = true
			}
			args = append(args, chain)
		} else {
			args = append(args, tokens[i])
		}
	}

	if !foundChain {
		args = append([]string{"-A", chain}, args...)
	}

	// Dispatch to the appropriate table.
	var runErr error
	if ruleFamily == "ip6tables" {
		runErr = a.runIPv6(args...)
	} else {
		runErr = a.run(args...)
	}
	if runErr != nil {
		return fmt.Errorf("failed to apply rule: %w", runErr)
	}

	log.Debug().Str("rule", rule).Strs("args", args).Str("family", ruleFamily).Msg("applied iptables rule")
	return nil
}

// splitByFamily partitions a slice of IP addresses into IPv4 and IPv6 slices.
// Addresses that don't parse are silently dropped.
func splitByFamily(ips []string) (ipv4s, ipv6s []string) {
	for _, s := range ips {
		ip := net.ParseIP(s)
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			ipv4s = append(ipv4s, s)
		} else {
			ipv6s = append(ipv6s, s)
		}
	}
	return
}

// Sync applies forwarding/NAT plus policy-based iptables rules.
// This method is called periodically when policy updates are received.
// To avoid dropping active connections, we check if rules exist before adding them.
//
// Three-tier authentication gate (WIRETY_JUMP chain on FORWARD):
//   1. AuthenticatedIPs — full network access, jumps to WIRETY_POLICY.
//   2. PendingAuthIPs   — peer has an in-flight captive-portal token; only
//                          external HTTPS is allowed (for the OIDC redirect chain).
//   3. QuarantinedIPs   — repeated auth failures; explicit DROP, no captive
//                          portal redirect (the user gets nothing until quarantine
//                          expires or admin clears it).
//   Default              — DROP, but DNS/captive-portal HTTP/HTTPS to the jump
//                          peer itself remain reachable via the INPUT chain so
//                          new peers can complete the OIDC flow.
//
// Physical-interface denylist (INPUT on the egress NIC, scoped to WireGuard
// listen port): drops UDP packets from rogue source IP:port pairs that the
// server flagged after detecting endpoint takeovers.  This prevents a rogue
// source from completing further WireGuard handshakes, ending the oscillation
// that would otherwise force the legitimate user to re-authenticate every
// keepalive cycle.
func (a *Adapter) Sync(req ports.SyncRequest) error {
	p := req.Policy
	_ = req.SelfIP // currently unused; reserved for future per-peer rules
	whitelistedIPs := req.AuthenticatedIPs
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

	// Separate IPv4 and IPv6 whitelisted addresses.
	whitelistIPv4, whitelistIPv6 := splitByFamily(whitelistedIPs)

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

		// NOTE: SNI / Host-header string matching (xt_string) was previously
		// attempted here for vhost isolation, but it fundamentally cannot work
		// with iptables+conntrack in this chain:
		//
		//   • The TCP SYN (conntrack state NEW — the only packet that is not
		//     already ESTABLISHED/RELATED) carries zero payload, so the string
		//     match never fires on it.
		//   • The TLS ClientHello (which contains the SNI) only arrives after
		//     the 3-way TCP handshake completes, at which point the connection
		//     is already in the ESTABLISHED state and is accepted by the
		//     ESTABLISHED/RELATED rule at the top of the chain, before the
		//     string-match rule is ever evaluated.
		//
		// Using the broken string-match rule made the Wirety server unreachable:
		// the SYN had no payload → no match → fell through to the port-443 REJECT
		// rule → tcp-reset before the TLS handshake could begin.
		//
		// We therefore always use destination-IP + port filtering only.
		// The security trade-off (other vhosts on the same reverse-proxy IP:port
		// being reachable) is acceptable: the alternative is that unauthenticated
		// peers cannot complete captive-portal auth at all.
		rule := append(append([]string{}, base...), "-j", "ACCEPT")
		if err := a.run(rule...); err != nil {
			log.Warn().Err(err).Str("ip", ip).Str("port", endpoint.port).Msg("failed to add Wirety server ACCEPT rule")
		}
	}

	// Tier 0 (highest priority): explicitly drop traffic from quarantined peers,
	// even before checking the authenticated whitelist.  This stops a peer that
	// already had a stale whitelist entry from being treated as authenticated
	// in the brief window after their auth is revoked but before the next
	// server push.
	for _, ip := range req.QuarantinedIPs {
		if err := a.run("-A", chain, "-i", a.iface, "-s", ip, "-j", "DROP"); err != nil {
			log.Warn().Err(err).Str("ip", ip).Msg("failed to add quarantine DROP rule")
		}
	}

	// Tier 1: Authenticated peers jump to the policy chain.
	for _, ip := range whitelistIPv4 {
		if err := a.run("-A", chain, "-i", a.iface, "-s", ip, "-j", policyChain); err != nil {
			log.Warn().Err(err).Str("ip", ip).Msg("failed to add whitelist jump rule")
		}
	}

	// Block HTTPS to RFC 1918 private address ranges with a TCP RST so the
	// browser fails immediately (instead of spinning for ~30 s on a silent DROP).
	// This applies to ALL non-authenticated peers (pending-auth and unauth alike)
	// so internal VPN resources stay protected during the OIDC flow.
	for _, privateNet := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"} {
		_ = a.run("-A", chain, "-i", a.iface, "-d", privateNet, "-p", "tcp", "--dport", "443", "-j", "REJECT", "--reject-with", "tcp-reset")
	}

	// Tier 2: peers with an in-flight captive portal token get external HTTPS
	// access for the duration of the token (≈10 minutes).  Scoped per-peer
	// (`-s wgIP`) so a peer that hasn't even hit the captive portal yet does NOT
	// get this grant — they must trigger token creation first via an HTTP probe
	// that gets redirected.  This replaces the previous global "ACCEPT all
	// HTTPS" rule that was the captive-portal bypass we're closing.
	pendingIPv4, _ := splitByFamily(req.PendingAuthIPs)
	for _, ip := range pendingIPv4 {
		if err := a.run("-A", chain, "-i", a.iface, "-s", ip, "-p", "tcp", "--dport", "443", "-j", "ACCEPT"); err != nil {
			log.Warn().Err(err).Str("ip", ip).Msg("failed to add pending-auth HTTPS allow rule")
		}
	}

	// RST port-443 (HTTPS) connections from unauthenticated peers instead of
	// silently DROPping them.  A TCP RST causes the browser to fail immediately
	// rather than waiting for a timeout, and triggers the OS captive-portal
	// notification on iOS and Android (which monitors for RST-on-443 to infer
	// that a captive portal is blocking HTTPS).  Without this, the phone shows
	// a spinning loader for ~30 s before giving up, and the "Sign in to network"
	// prompt may never appear.
	//
	// Note: connections to the jump peer's OWN WireGuard IP (captive portal) go
	// through the INPUT chain, not FORWARD, so this rule never affects the
	// captive portal HTTPS listener on the WireGuard interface.
	_ = a.run("-A", chain, "-i", a.iface, "-p", "tcp", "--dport", "443", "-j", "REJECT", "--reject-with", "tcp-reset")

	// Drop all remaining traffic from unauthenticated peers.
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
		log.Info().Int("rule_count", len(p.IPTablesRules)).Msg("applying policy-based iptables rules (IPv4)")
		for i, rule := range p.IPTablesRules {
			// Family="iptables" — silently skip ip6tables-prefixed rules (they
			// are applied by syncIPv6 against the WIRETY6_POLICY chain).
			if err := a.applyIPTablesRule(policyChain, rule, "iptables"); err != nil {
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

	// HTTP DNAT redirect: any port-80 TCP packet arriving on the WireGuard
	// interface from an unauthenticated peer that was NOT already directed at the
	// jump peer's own IP (i.e. the peer bypassed DNS and connected to a public IP
	// directly) is redirected to this host's port 80 — the captive portal HTTP
	// server.  The nat table's PREROUTING hook runs before the filter table, so
	// the redirected packet is then treated as destined for INPUT (local delivery)
	// and accepted by the INPUT rule for port 80.
	//
	// Authenticated peers (whitelisted) are excluded explicitly so their HTTP
	// traffic continues to be forwarded normally after auth.
	//
	// We rebuild the WIRETY_REDIR chain idempotently on each sync so the
	// authenticated-peer exclusions stay current.
	redirChain := "WIRETY_REDIR"
	_ = a.run("-t", "nat", "-N", redirChain)
	_ = a.run("-t", "nat", "-F", redirChain)
	// Exclude authenticated peers — their HTTP traffic must be forwarded, not redirected.
	for _, ip := range whitelistIPv4 {
		_ = a.run("-t", "nat", "-A", redirChain, "-s", ip, "-j", "RETURN")
	}
	// Redirect all remaining port-80 traffic from the WireGuard interface.
	_ = a.run("-t", "nat", "-A", redirChain, "-p", "tcp", "--dport", "80", "-j", "REDIRECT", "--to-port", "80")
	// Wire the chain into PREROUTING (idempotent).
	_ = a.runIfNotExists("-t", "nat", "-I", "PREROUTING", "1", "-i", a.iface, "-p", "tcp", "--dport", "80", "-j", redirChain)

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
		log.Info().Msg("no NAT interfaces configured — peers will not have routed IPv4 access")
	}
	for _, natIface := range natIfaces {
		if err := a.runIfNotExists("-t", "nat", "-A", "POSTROUTING", "-o", natIface, "-j", "MASQUERADE"); err != nil {
			log.Warn().Err(err).Str("interface", natIface).Msg("failed to add IPv4 MASQUERADE rule")
		} else {
			log.Debug().Str("interface", natIface).Msg("IPv4 MASQUERADE rule configured")
		}
	}

	// ── ip6tables: dual-stack FORWARD + NAT ──────────────────────────────────
	//
	// Mirror the iptables two-chain design for IPv6.  The semantics are identical:
	//   WIRETY6_JUMP  — authentication gate
	//   WIRETY6_POLICY — per-destination rules for authenticated peers
	//
	// Private IPv6 ranges:
	//   fc00::/7  — ULA (Unique Local Addresses, RFC 4193) — analogous to RFC 1918
	//   fe80::/10 — link-local — not routable, but blocked for completeness
	a.syncIPv6(p, whitelistIPv6, endpoint, req)

	// ── Physical-interface denylist (rogue WireGuard sources) ────────────────
	//
	// Drops UDP packets to the WireGuard listen port from sources flagged by the
	// server as rogue takeovers.  This is the second-line defence that actually
	// stops a stolen WireGuard config from being usable concurrently with the
	// legitimate session: even though WireGuard's protocol accepts handshakes
	// from any source matching the cryptographic identity, our iptables rule
	// drops them before the kernel module ever sees them, so the legitimate
	// peer's stored endpoint is never overwritten.
	a.syncWireGuardDenylist(req.EndpointDenylist, req.WireGuardListenPort)

	return nil
}

// WIRETY_WGDENY is the chain that holds the per-source DROP rules for rogue
// WireGuard UDP packets.  Maintained idempotently: each Sync flushes and
// rebuilds it from the current denylist set.
const wgDenyChain = "WIRETY_WGDENY"

// syncWireGuardDenylist (re)builds the WIRETY_WGDENY chain on the INPUT path
// of the physical interface.  Each entry becomes a `-s <ip> [--sport <port>]
// -p udp --dport <wg_port> -j DROP` rule, covering both legacy iptables and
// ip6tables (entries with IPv6 addresses go to ip6tables only).
//
// If wgListenPort is zero we skip the whole step — without a WireGuard port we
// can't safely DROP UDP packets (we'd risk dropping unrelated UDP traffic from
// the same source, e.g. a DNS reply).
func (a *Adapter) syncWireGuardDenylist(entries []ports.DenylistEntry, wgListenPort int) {
	if wgListenPort <= 0 {
		log.Debug().Msg("WireGuard denylist: listen port unknown, skipping")
		// Still flush the chain so stale rules from a previous sync don't linger.
		_ = a.run("-N", wgDenyChain)
		_ = a.run("-F", wgDenyChain)
		_ = a.runIPv6("-N", wgDenyChain)
		_ = a.runIPv6("-F", wgDenyChain)
		return
	}

	// Create + flush idempotently.
	_ = a.run("-N", wgDenyChain)
	_ = a.run("-F", wgDenyChain)
	_ = a.runIPv6("-N", wgDenyChain)
	_ = a.runIPv6("-F", wgDenyChain)

	wgPortStr := strconv.Itoa(wgListenPort)

	for _, e := range entries {
		ip := net.ParseIP(e.BlockedIP)
		if ip == nil {
			continue
		}
		args := []string{"-A", wgDenyChain, "-p", "udp", "--dport", wgPortStr, "-s", e.BlockedIP}
		if e.BlockedPort > 0 {
			args = append(args, "--sport", strconv.Itoa(e.BlockedPort))
		}
		args = append(args, "-j", "DROP")
		if ip.To4() != nil {
			if err := a.run(args...); err != nil {
				log.Warn().Err(err).Str("source", e.BlockedIP).Int("port", e.BlockedPort).Msg("failed to add WireGuard denylist rule")
			} else {
				log.Info().Str("source", e.BlockedIP).Int("port", e.BlockedPort).Int("wg_port", wgListenPort).Msg("WireGuard denylist: rogue source blocked at physical interface")
			}
		} else {
			if err := a.runIPv6(args...); err != nil {
				log.Warn().Err(err).Str("source", e.BlockedIP).Int("port", e.BlockedPort).Msg("failed to add WireGuard denylist rule (IPv6)")
			} else {
				log.Info().Str("source", e.BlockedIP).Int("port", e.BlockedPort).Int("wg_port", wgListenPort).Msg("WireGuard denylist: rogue source blocked at physical interface (IPv6)")
			}
		}
	}

	// Wire WIRETY_WGDENY into INPUT (idempotent — no -i filter, the rules
	// inside the chain match by destination port so they only affect WireGuard
	// traffic regardless of which interface it arrived on).
	_ = a.runIfNotExists("-I", "INPUT", "1", "-p", "udp", "--dport", wgPortStr, "-j", wgDenyChain)
	_ = a.runIPv6IfNotExists("-I", "INPUT", "1", "-p", "udp", "--dport", wgPortStr, "-j", wgDenyChain)
}

// syncIPv6 applies ip6tables rules mirroring the iptables WIRETY_JUMP / WIRETY_POLICY
// two-chain design for IPv6 traffic on the WireGuard interface.
func (a *Adapter) syncIPv6(p *dom.JumpPolicy, whitelistIPv6 []string, endpoint serverEndpoint, req ports.SyncRequest) {
	chain6 := "WIRETY6_JUMP"
	policy6 := "WIRETY6_POLICY"

	_ = a.runIPv6("-N", chain6)
	_ = a.runIPv6("-F", chain6)
	_ = a.runIPv6("-N", policy6)
	_ = a.runIPv6("-F", policy6)

	// Rule 0: ESTABLISHED/RELATED → ACCEPT
	_ = a.runIPv6("-A", chain6, "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT")

	// Rule 1: Allow peers to reach the Wirety server via its IPv6 addresses.
	for _, ip := range endpoint.ipsv6 {
		base := []string{"-A", chain6, "-i", a.iface, "-d", ip, "-p", "tcp", "--dport", endpoint.port}
		rule := append(append([]string{}, base...), "-j", "ACCEPT")
		if err := a.runIPv6(rule...); err != nil {
			log.Warn().Err(err).Str("ip", ip).Str("port", endpoint.port).Msg("failed to add IPv6 Wirety server ACCEPT rule")
		}
	}

	// Tier 0: explicit DROP for quarantined IPv6 addresses (parallels IPv4).
	_, quarantineIPv6 := splitByFamily(req.QuarantinedIPs)
	for _, ip := range quarantineIPv6 {
		if err := a.runIPv6("-A", chain6, "-i", a.iface, "-s", ip, "-j", "DROP"); err != nil {
			log.Warn().Err(err).Str("ip", ip).Msg("failed to add IPv6 quarantine DROP rule")
		}
	}

	// Tier 1: Authenticated peer IPv6 addresses jump to the policy chain.
	for _, ip := range whitelistIPv6 {
		if err := a.runIPv6("-A", chain6, "-i", a.iface, "-s", ip, "-j", policy6); err != nil {
			log.Warn().Err(err).Str("ip", ip).Msg("failed to add IPv6 whitelist jump rule")
		}
	}

	// Block HTTPS to ULA and link-local ranges (private IPv6 resources).
	// These are the IPv6 equivalents of RFC 1918 — unauthenticated peers must not
	// reach private IPv6 services before completing captive portal authentication.
	for _, privateNet6 := range []string{"fc00::/7", "fe80::/10"} {
		_ = a.runIPv6("-A", chain6, "-i", a.iface, "-d", privateNet6, "-p", "tcp", "--dport", "443", "-j", "REJECT", "--reject-with", "tcp-reset")
	}

	// Tier 2: pending-auth peers get external HTTPS access for OIDC redirects.
	_, pendingIPv6 := splitByFamily(req.PendingAuthIPs)
	for _, ip := range pendingIPv6 {
		if err := a.runIPv6("-A", chain6, "-i", a.iface, "-s", ip, "-p", "tcp", "--dport", "443", "-j", "ACCEPT"); err != nil {
			log.Warn().Err(err).Str("ip", ip).Msg("failed to add IPv6 pending-auth HTTPS allow rule")
		}
	}

	// RST HTTPS for unauthenticated peers (mirrors IPv4 — see Sync() for rationale).
	_ = a.runIPv6("-A", chain6, "-i", a.iface, "-p", "tcp", "--dport", "443", "-j", "REJECT", "--reject-with", "tcp-reset")

	// Drop all remaining IPv6 traffic from unauthenticated peers.
	_ = a.runIPv6("-A", chain6, "-i", a.iface, "-j", "DROP")

	// Policy chain: per-destination rules (or catch-all ACCEPT for backward compat).
	//
	// Server-side rule generation now emits family-tagged rules ("iptables …" or
	// "ip6tables …" prefix). We dispatch each rule through applyIPTablesRule with
	// family="ip6tables" so only the ip6tables-prefixed ones land here — IPv4
	// rules are silently skipped (they're applied by Sync's IPv4 path).
	if len(p.IPTablesRules) > 0 {
		log.Info().Int("rule_count", len(p.IPTablesRules)).Msg("applying policy-based iptables rules (IPv6)")
		for i, rule := range p.IPTablesRules {
			if err := a.applyIPTablesRule(policy6, rule, "ip6tables"); err != nil {
				log.Debug().Err(err).Int("rule_index", i).Str("rule", rule).Msg("ip6tables policy rule skipped")
			}
		}
	} else {
		_ = a.runIPv6("-A", policy6, "-j", "ACCEPT")
	}

	// Attach the IPv6 chain to FORWARD (idempotent).
	_ = a.runIPv6IfNotExists("-I", "FORWARD", "1", "-j", chain6)

	// IPv6 HTTP DNAT redirect (mirrors IPv4 — see Sync() for rationale).
	// ip6tables nat PREROUTING redirects port-80 from the WireGuard interface to
	// the local captive portal HTTP server.  Authenticated peers are excluded.
	redir6Chain := "WIRETY6_REDIR"
	_ = a.runIPv6("-t", "nat", "-N", redir6Chain)
	_ = a.runIPv6("-t", "nat", "-F", redir6Chain)
	for _, ip := range whitelistIPv6 {
		_ = a.runIPv6("-t", "nat", "-A", redir6Chain, "-s", ip, "-j", "RETURN")
	}
	_ = a.runIPv6("-t", "nat", "-A", redir6Chain, "-p", "tcp", "--dport", "80", "-j", "REDIRECT", "--to-port", "80")
	_ = a.runIPv6IfNotExists("-t", "nat", "-I", "PREROUTING", "1", "-i", a.iface, "-p", "tcp", "--dport", "80", "-j", redir6Chain)

	// Allow jump-peer services on the WireGuard interface INPUT chain (IPv6).
	_ = a.runIPv6IfNotExists("-I", "INPUT", "1", "-i", a.iface, "-p", "tcp", "--dport", "80", "-j", "ACCEPT")
	_ = a.runIPv6IfNotExists("-I", "INPUT", "1", "-i", a.iface, "-p", "tcp", "--dport", "443", "-j", "ACCEPT")
	_ = a.runIPv6IfNotExists("-I", "INPUT", "1", "-i", a.iface, "-p", "udp", "--dport", "53", "-j", "ACCEPT")
	_ = a.runIPv6IfNotExists("-I", "INPUT", "1", "-i", a.iface, "-p", "tcp", "--dport", "53", "-j", "ACCEPT")

	// IPv6 MASQUERADE on egress interfaces with global IPv6 addresses.
	natIfacesIPv6 := a.detectNATInterfacesIPv6()
	for _, natIface := range natIfacesIPv6 {
		if err := a.runIPv6IfNotExists("-t", "nat", "-A", "POSTROUTING", "-o", natIface, "-j", "MASQUERADE"); err != nil {
			log.Warn().Err(err).Str("interface", natIface).Msg("failed to add IPv6 MASQUERADE rule")
		} else {
			log.Debug().Str("interface", natIface).Msg("IPv6 MASQUERADE rule configured")
		}
	}

	log.Debug().
		Int("whitelist_ipv6", len(whitelistIPv6)).
		Int("server_ipv6", len(endpoint.ipsv6)).
		Msg("ip6tables rules applied")
}
