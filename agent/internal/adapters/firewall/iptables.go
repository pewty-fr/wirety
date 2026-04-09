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
	iface             string
	natInterfaces     []string // explicit override; nil means auto-detect
	httpPort          int
	httpsPort         int
	captivePortalPort int
	serverURL         string // Wirety server URL — peers must always be able to reach it
}

// NewAdapter creates a new firewall adapter.
// wgIface: WireGuard interface name (e.g., "wg0")
// natIfaces: explicit NAT interfaces override; nil or empty slice means auto-detect all
func NewAdapter(wgIface string, natIfaces []string) *Adapter {
	return &Adapter{
		iface:             wgIface,
		natInterfaces:     natIfaces,
		httpPort:          3128,
		httpsPort:         3129,
		captivePortalPort: 8081,
	}
}

// SetProxyPorts sets the HTTP and HTTPS proxy ports
func (a *Adapter) SetProxyPorts(httpPort, httpsPort int) {
	a.httpPort = httpPort
	a.httpsPort = httpsPort
}

// SetCaptivePortalPort sets the local port for the captive portal redirect server
func (a *Adapter) SetCaptivePortalPort(port int) {
	a.captivePortalPort = port
}

// SetServerURL stores the Wirety server URL so the adapter can add an unconditional
// ACCEPT rule for traffic destined to it — unauthenticated peers must always be able
// to reach the server to complete captive portal authentication.
func (a *Adapter) SetServerURL(serverURL string) {
	a.serverURL = serverURL
}

// resolveServerIPs resolves the Wirety server URL to a list of IP addresses.
func (a *Adapter) resolveServerIPs() []string {
	if a.serverURL == "" {
		return nil
	}
	u, err := url.Parse(a.serverURL)
	if err != nil {
		log.Warn().Err(err).Str("server_url", a.serverURL).Msg("failed to parse server URL")
		return nil
	}
	host := u.Hostname()
	if host == "" {
		return nil
	}
	// Already an IP — use directly without DNS lookup
	if net.ParseIP(host) != nil {
		return []string{host}
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		log.Warn().Err(err).Str("host", host).Msg("failed to resolve wirety server hostname")
		return nil
	}
	return addrs
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
	//   1. Wirety server IPs   → ACCEPT  (captive portal auth always reachable)
	//   2. Whitelisted peer IP → jump to WIRETY_POLICY
	//   3. Everything else     → DROP    (unauthenticated peers blocked on ALL ports)
	//
	// WIRETY_POLICY (per-destination rules, only reached by authenticated peers):
	//   1. Explicit policy rules (ACCEPT/DROP per destination CIDR)
	//   2. Catch-all ACCEPT    (backward compat: authenticated = full access by default)
	//
	// This prevents unauthenticated peers from bypassing the captive portal via HTTPS
	// or any other port that is not intercepted by the DNAT rule: they hit the DROP in
	// WIRETY_JUMP before any policy rule is ever evaluated.

	chain := "WIRETY_JUMP"
	policyChain := "WIRETY_POLICY"

	_ = a.run("-N", chain)
	_ = a.run("-F", chain)
	_ = a.run("-N", policyChain)
	_ = a.run("-F", policyChain)

	// Always allow peers to reach the Wirety server regardless of auth state.
	// Without this, peers can't complete captive portal authentication when the
	// server is only reachable via the VPN (not directly over the internet).
	for _, ip := range a.resolveServerIPs() {
		if err := a.run("-A", chain, "-i", a.iface, "-d", ip, "-j", "ACCEPT"); err != nil {
			log.Warn().Err(err).Str("ip", ip).Msg("failed to add Wirety server ACCEPT rule")
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
	if len(p.IPTablesRules) > 0 {
		log.Info().Int("rule_count", len(p.IPTablesRules)).Msg("applying policy-based iptables rules")
		for i, rule := range p.IPTablesRules {
			if err := a.applyIPTablesRule(policyChain, rule); err != nil {
				log.Error().Err(err).Int("rule_index", i).Str("rule", rule).Msg("failed to apply iptables rule")
			}
		}
	} else {
		log.Debug().Msg("no policy-based iptables rules to apply")
	}

	// Catch-all ACCEPT at the end of WIRETY_POLICY: an authenticated peer with no
	// matching explicit policy rule still gets through (maintains prior behaviour
	// where whitelist = full access).
	_ = a.run("-A", policyChain, "-j", "ACCEPT")

	log.Debug().Msg("applied policy-based iptables rules")

	// ── Captive portal DNAT chain (WIRETY_CAPTIVE) ─────────────────────────
	// Redirect unauthenticated peers' HTTP traffic to the local redirect server
	// which creates a token and sends them to the Wirety captive portal page.
	if err := exec.Command("sysctl", "-w", "net.ipv4.conf.all.route_localnet=1").Run(); err != nil { // #nosec G204
		log.Warn().Err(err).Msg("failed to enable route_localnet")
	}

	captiveChain := "WIRETY_CAPTIVE"
	_ = a.run("-t", "nat", "-N", captiveChain)
	_ = a.run("-t", "nat", "-F", captiveChain)

	// Whitelisted peers bypass the DNAT redirect
	for _, ip := range whitelistedIPs {
		_ = a.run("-t", "nat", "-A", captiveChain, "-s", ip, "-j", "RETURN")
	}

	// Redirect all remaining HTTP traffic to the local captive portal server
	_ = a.run("-t", "nat", "-A", captiveChain,
		"-i", a.iface, "-p", "tcp", "--dport", "80",
		"-j", "DNAT", "--to-destination", fmt.Sprintf("127.0.0.1:%d", a.captivePortalPort))

	// Attach WIRETY_CAPTIVE to PREROUTING (idempotent)
	_ = a.runIfNotExists("-t", "nat", "-I", "PREROUTING", "1", "-i", a.iface, "-j", captiveChain)

	// Attach chain to FORWARD (insert at top, only if not already attached)
	_ = a.runIfNotExists("-I", "FORWARD", "1", "-j", chain)

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
