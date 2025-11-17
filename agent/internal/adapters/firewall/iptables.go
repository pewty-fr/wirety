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
}

func NewAdapter(wgIface, natIface string) *Adapter {
	return &Adapter{iface: wgIface, natInterface: natIface}
}

func (a *Adapter) run(args ...string) error {
	cmd := exec.Command("iptables", args...) // #nosec G204
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables %v failed: %v output=%s", args, err, string(out))
	}
	return nil
}

// Sync applies forwarding/NAT plus isolation & ACL blocks.
func (a *Adapter) Sync(p *dom.JumpPolicy, selfIP string) error {
	if p == nil {
		return nil
	}
	// Ensure IP forwarding enabled
	if err := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run(); err != nil {
		log.Warn().Err(err).Msg("failed enabling ip_forward")
	}
	// Create custom chain
	chain := "WIRETY_JUMP"
	// Flush or create chain
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

	// Accept passes
	_ = a.run("-A", chain, "-i", a.iface, "-j", "ACCEPT")
	_ = a.run("-A", chain, "-o", a.iface, "-j", "ACCEPT")

	// Attach chain to FORWARD (insert at top)
	_ = a.run("-D", "FORWARD", "-j", chain) // remove if exists
	_ = a.run("-I", "FORWARD", "1", "-j", chain)

	// NAT (MASQUERADE) if needed
	if a.natInterface != "" {
		_ = a.run("-t", "nat", "-D", "POSTROUTING", "-o", a.natInterface, "-j", "MASQUERADE")
		_ = a.run("-t", "nat", "-A", "POSTROUTING", "-o", a.natInterface, "-j", "MASQUERADE")
	}
	return nil
}
