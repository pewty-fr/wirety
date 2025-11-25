package firewall

import (
	"fmt"
	"os/exec"
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

func (a *Adapter) run(args ...string) error {
	cmd := exec.Command("iptables", args...) // #nosec G204
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables %v failed: %v output=%s", args, err, string(out))
	}
	return nil
}

// ruleExists checks if an iptables rule exists
func (a *Adapter) ruleExists(args ...string) bool {
	// Replace -A (append) or -I (insert) with -C (check)
	checkArgs := make([]string, len(args))
	copy(checkArgs, args)
	for i, arg := range checkArgs {
		if arg == "-A" || arg == "-I" {
			checkArgs[i] = "-C"
			break
		}
	}
	cmd := exec.Command("iptables", checkArgs...) // #nosec G204
	return cmd.Run() == nil
}

// runIfNotExists runs an iptables command only if the rule doesn't already exist
func (a *Adapter) runIfNotExists(args ...string) error {
	if a.ruleExists(args...) {
		return nil // Rule already exists, skip
	}
	return a.run(args...)
}

// Sync applies forwarding/NAT plus isolation & ACL blocks.
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
	// Create custom chain
	chain := "WIRETY_JUMP"
	// Flush or create chain (we flush the chain content but keep chain attachments)
	_ = a.run("-N", chain)
	_ = a.run("-F", chain)

	// Base accept rules (will be appended after drop rules)
	// Isolation logic:
	isolated := map[string]bool{}
	for _, peer := range p.Peers {
		if peer.Isolated {
			isolated[peer.IP] = true
		}
	}

	// Rule sets: isolated<->isolated, non->isolated, jump->isolated
	ips := make([]string, 0, len(p.Peers))
	for _, peer := range p.Peers {
		ips = append(ips, peer.IP)
	}

	// isolated <-> isolated
	for i := 0; i < len(ips); i++ {
		for j := i + 1; j < len(ips); j++ {
			ip1, ip2 := ips[i], ips[j]
			if isolated[ip1] && isolated[ip2] {
				_ = a.run("-A", chain, "-s", ip1, "-d", ip2, "-m", "state", "--state", "NEW", "-j", "DROP")
				_ = a.run("-A", chain, "-s", ip2, "-d", ip1, "-m", "state", "--state", "NEW", "-j", "DROP")
			}
		}
	}
	// non-isolated -> isolated
	for _, src := range ips {
		if !isolated[src] {
			for _, dst := range ips {
				if isolated[dst] {
					_ = a.run("-A", chain, "-s", src, "-d", dst, "-m", "state", "--state", "NEW", "-j", "DROP")
				}
			}
		}
	}
	// jump -> isolated
	// for _, dst := range ips {
	// 	if isolated[dst] {
	// 		_ = a.run("-A", chain, "-s", selfIP, "-d", dst, "-m", "state", "--state", "NEW", "-j", "DROP")
	// 	}
	// }

	// ACL blocked: treat as bidirectional drop pairs
	blockedSet := map[string]bool{}
	for _, id := range p.ACLBlocked {
		blockedSet[id] = true
	}
	// Need mapping from peer ID to IP
	idToIP := map[string]string{}
	for _, peer := range p.Peers {
		idToIP[peer.ID] = peer.IP
	}
	for _, srcPeer := range p.Peers {
		if blockedSet[srcPeer.ID] {
			for _, dstPeer := range p.Peers {
				if blockedSet[dstPeer.ID] && srcPeer.ID != dstPeer.ID {
					_ = a.run("-A", chain, "-s", srcPeer.IP, "-d", dstPeer.IP, "-m", "state", "--state", "NEW", "-j", "DROP")
				}
			}
		}
	}

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

	// Accept passes for authenticated peers and agent peers
	_ = a.run("-A", chain, "-i", a.iface, "-j", "ACCEPT")
	_ = a.run("-A", chain, "-o", a.iface, "-j", "ACCEPT")

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
