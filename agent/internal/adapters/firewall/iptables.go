package firewall

import (
	"fmt"
	"os/exec"
	"strings"
	dom "wirety/agent/internal/domain/policy"

	"github.com/rs/zerolog/log"
)

// Adapter implements dynamic filtering using iptables commands (still kernel-level
// but managed by agent rather than embedded in wg config).
// Simplified: rules applied each sync by flushing dedicated chain.

type Adapter struct {
	iface        string
	natInterface string
	httpPort     int
	httpsPort    int
}

func NewAdapter(wgIface, natIface string) *Adapter {
	return &Adapter{
		iface:        wgIface,
		natInterface: natIface,
		httpPort:     3128, // Default HTTP proxy port
		httpsPort:    3129, // Default HTTPS proxy port
	}
}

// SetProxyPorts sets the HTTP and HTTPS proxy ports
func (a *Adapter) SetProxyPorts(httpPort, httpsPort int) {
	a.httpPort = httpPort
	a.httpsPort = httpsPort
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

// ruleExists checks if an iptables rule exists by parsing the rule arguments
func (a *Adapter) ruleExists(args ...string) bool {
	// Extract table, chain, and target from args
	table := "filter"
	var chain, target string

	// Parse arguments to find table, chain, and jump target
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-t":
			if i+1 < len(args) {
				table = args[i+1]
				i++
			}
		case "-A", "-I":
			if i+1 < len(args) {
				chain = args[i+1]
				i++
				// Skip position number if present (for -I)
				if args[i-1] == "-I" && i < len(args) && args[i] != "" && args[i][0] >= '0' && args[i][0] <= '9' {
					i++
				}
			}
		case "-j":
			if i+1 < len(args) {
				target = args[i+1]
				i++
			}
		}
	}

	if chain == "" || target == "" {
		return false
	}

	// Use iptables-save to check if rule exists
	var cmd *exec.Cmd
	if table == "filter" {
		cmd = exec.Command("iptables-save", "-t", "filter") // #nosec G204
	} else {
		cmd = exec.Command("iptables-save", "-t", table) // #nosec G204
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	// Look for the rule in the output
	// Format: -A CHAIN ... -j TARGET
	searchPattern := fmt.Sprintf("-A %s ", chain)
	targetPattern := fmt.Sprintf("-j %s", target)

	lines := string(output)
	for i := 0; i < len(lines); {
		end := i
		for end < len(lines) && lines[end] != '\n' {
			end++
		}
		line := lines[i:end]

		// Check if line contains both chain and target
		if containsSubstring(line, searchPattern) && containsSubstring(line, targetPattern) {
			return true
		}

		i = end + 1
	}

	return false
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// runIfNotExists runs an iptables command only if the rule doesn't already exist
func (a *Adapter) runIfNotExists(args ...string) error {
	if a.ruleExists(args...) {
		return nil // Rule already exists, skip
	}
	return a.run(args...)
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
	// Create custom chain if it doesn't exist, then flush it
	chain := "WIRETY_JUMP"
	// Try to create the chain (will fail silently if it already exists)
	_ = a.run("-N", chain)
	_ = a.run("-F", chain)

	// Build whitelist map for quick lookup (needed for blocking rules)
	whitelisted := make(map[string]bool)
	for _, ip := range whitelistedIPs {
		whitelisted[ip] = true
	}

	// Block all traffic from unauthenticated non-agent peers (except to proxy ports)
	// This ensures they can only access the captive portal
	for _, peer := range p.Peers {
		if !peer.UseAgent && !whitelisted[peer.IP] {
			// Allow traffic to proxy ports (for captive portal redirect)
			_ = a.run("-A", chain,
				"-i", a.iface,
				"-s", peer.IP,
				"-p", "tcp",
				"--dport", fmt.Sprintf("%d", a.httpPort),
				"-j", "ACCEPT")

			_ = a.run("-A", chain,
				"-i", a.iface,
				"-s", peer.IP,
				"-p", "tcp",
				"--dport", fmt.Sprintf("%d", a.httpsPort),
				"-j", "ACCEPT")

			// Allow DNS (port 53) for captive portal to work
			_ = a.run("-A", chain,
				"-i", a.iface,
				"-s", peer.IP,
				"-p", "udp",
				"--dport", "53",
				"-j", "ACCEPT")

			// Block all other traffic from unauthenticated non-agent peers
			_ = a.run("-A", chain,
				"-i", a.iface,
				"-s", peer.IP,
				"-j", "DROP")

			log.Debug().
				Str("peer_ip", peer.IP).
				Str("peer_name", peer.Name).
				Msg("blocking all traffic from unauthenticated non-agent peer")
		}
	}

	// Apply policy-based iptables rules in order
	// These rules are generated by the server based on policies attached to groups
	if len(p.IPTablesRules) > 0 {
		log.Info().Int("rule_count", len(p.IPTablesRules)).Msg("applying policy-based iptables rules")
		for i, rule := range p.IPTablesRules {
			if err := a.applyIPTablesRule(chain, rule); err != nil {
				log.Error().Err(err).Int("rule_index", i).Str("rule", rule).Msg("failed to apply iptables rule")
				// Continue applying other rules even if one fails
			}
		}
	} else {
		log.Debug().Msg("no policy-based iptables rules to apply")
	}

	// Default deny rule at the end for policy-based access control
	// This ensures that any traffic not explicitly allowed by policies is denied
	_ = a.run("-A", chain, "-i", a.iface, "-j", "DROP")
	_ = a.run("-A", chain, "-o", a.iface, "-j", "DROP")

	log.Debug().Msg("applied default deny rules for policy-based access control")

	// Attach chain to FORWARD (insert at top, only if not already attached)
	_ = a.runIfNotExists("-I", "FORWARD", "1", "-j", chain)

	// Captive portal redirection for non-agent peers
	// Create a custom chain for captive portal redirects
	// We flush the chain content but keep chain attachments to avoid dropping connections
	captiveChain := "WIRETY_CAPTIVE"
	_ = a.run("-t", "nat", "-N", captiveChain)
	_ = a.run("-t", "nat", "-F", captiveChain)

	// Redirect HTTP and HTTPS traffic from non-agent peers
	// HTTP (port 80) → HTTP proxy (port from --http-port flag)
	// HTTPS (port 443) → TLS-SNI gateway (port from --https-port flag)
	for _, peer := range p.Peers {
		if !peer.UseAgent && !whitelisted[peer.IP] {
			// Redirect HTTP traffic (port 80) to HTTP proxy
			_ = a.run("-t", "nat", "-A", captiveChain,
				"-i", a.iface,
				"-s", peer.IP,
				"-p", "tcp",
				"--dport", "80",
				"-j", "REDIRECT",
				"--to-port", fmt.Sprintf("%d", a.httpPort))

			// Redirect HTTPS traffic (port 443) to TLS-SNI gateway
			// The TLS-SNI gateway will parse SNI and only allow server domain
			_ = a.run("-t", "nat", "-A", captiveChain,
				"-i", a.iface,
				"-s", peer.IP,
				"-p", "tcp",
				"--dport", "443",
				"-j", "REDIRECT",
				"--to-port", fmt.Sprintf("%d", a.httpsPort))

			log.Debug().
				Str("peer_ip", peer.IP).
				Str("peer_name", peer.Name).
				Int("http_redirect_port", a.httpPort).
				Int("https_redirect_port", a.httpsPort).
				Msg("added captive portal redirect for non-agent peer")
		} else if !peer.UseAgent && whitelisted[peer.IP] {
			log.Debug().
				Str("peer_ip", peer.IP).
				Str("peer_name", peer.Name).
				Msg("skipping captive portal redirect for whitelisted peer")
		}
	}

	// Attach captive portal chain to PREROUTING (only if not already attached)
	_ = a.runIfNotExists("-t", "nat", "-I", "PREROUTING", "1", "-j", captiveChain)

	// NAT (MASQUERADE) if needed (only add if not already present)
	if a.natInterface != "" {
		_ = a.runIfNotExists("-t", "nat", "-A", "POSTROUTING", "-o", a.natInterface, "-j", "MASQUERADE")
	}
	return nil
}
