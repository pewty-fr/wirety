//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestE2E stands up the full stack once and runs the scenario as subtests that
// share it (container startup is expensive). The subtests build on each other:
// the network/peers/policy/route/DNS created in setup are asserted from several
// angles.
func TestE2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	st := setupStack(ctx, t)

	// ---- identities --------------------------------------------------------
	// The admin (first user) provisions everything. peer-a is owned by a
	// regular user: captive-portal auth only whitelists a peer for its owner.
	userTok, err := dexPasswordToken(ctx, st.dexTokenURL, dexClientID, dexClientSecret, userEmail, userPassword)
	if err != nil {
		t.Fatalf("dex user token: %v", err)
	}
	owner, err := newAPIClient(st.apiBaseURL, userTok).me(ctx) // also registers the user
	if err != nil {
		t.Fatalf("user /me: %v", err)
	}

	// ---- provision the topology via the admin API -------------------------
	net, err := st.admin.createNetwork(ctx, network{
		Name:         "corp",
		CIDR:         "10.90.0.0/24",
		DomainSuffix: "e2e.internal",
	})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}

	// Jump peer (runs the agent). Its endpoint is a placeholder: the peer's
	// config is rewritten to the agent container's IP once it is known.
	jumpPeer, err := st.admin.createPeer(ctx, net.ID, createPeerReq{
		Name: "jump-1", IsJump: true, UseAgent: true,
		Endpoint: "10.255.255.254", ListenPort: 51820,
	})
	if err != nil {
		t.Fatalf("create jump peer: %v", err)
	}
	if jumpPeer.Token == "" {
		t.Fatal("jump peer create returned no enrollment token")
	}

	// A plain WireGuard peer (no agent), owned by the regular user.
	regPeer, err := st.admin.createPeer(ctx, net.ID, createPeerReq{
		Name: "peer-a", OwnerID: owner.ID,
	})
	if err != nil {
		t.Fatalf("create regular peer: %v", err)
	}
	regPeerIP := stripPrefix(regPeer.Address) // e.g. "10.90.0.2"

	// Two private services behind the jump peer, both routed to peer-a's group:
	// the policy allows only the first, so the second proves default-deny.
	_, svcIP := st.startPrivateService(ctx, t, "private-svc")
	svcCIDR := svcIP + "/32"
	_, deniedIP := st.startPrivateService(ctx, t, "denied-svc")

	rt, err := st.admin.createRoute(ctx, net.ID, createRouteReq{
		Name: "to-private", DestinationCIDR: svcCIDR, JumpPeerID: jumpPeer.ID,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	deniedRt, err := st.admin.createRoute(ctx, net.ID, createRouteReq{
		Name: "to-denied", DestinationCIDR: deniedIP + "/32", JumpPeerID: jumpPeer.ID,
	})
	if err != nil {
		t.Fatalf("create denied route: %v", err)
	}
	dnsRec, err := st.admin.createDNS(ctx, net.ID, rt.ID, createDNSReq{
		Name: "app", IPAddress: svcIP,
	})
	if err != nil {
		t.Fatalf("create dns mapping: %v", err)
	}
	fqdn := fmt.Sprintf("%s.%s.%s", dnsRec.Name, net.Name, net.DomainSuffix) // app.corp.e2e.internal

	// Group + policy: allow peer-a to reach the private service CIDR only.
	grp, err := st.admin.createGroup(ctx, net.ID, "app-users")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := st.admin.addPeerToGroup(ctx, net.ID, grp.ID, regPeer.ID); err != nil {
		t.Fatalf("add peer to group: %v", err)
	}
	for _, r := range []string{rt.ID, deniedRt.ID} {
		if err := st.admin.attachRouteToGroup(ctx, net.ID, grp.ID, r); err != nil {
			t.Fatalf("attach route to group: %v", err)
		}
	}
	pol, err := st.admin.createPolicy(ctx, net.ID, createPolicyReq{
		Name: "allow-app",
		Rules: []policyRule{
			{Direction: "output", Action: "allow", TargetType: "cidr", Target: svcCIDR},
		},
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if err := st.admin.attachPolicyToGroup(ctx, net.ID, grp.ID, pol.ID); err != nil {
		t.Fatalf("attach policy to group: %v", err)
	}

	// ---- start the jump agent and let it sync ------------------------------
	jump := st.startJumpAgent(ctx, t, jumpPeer.Token)
	jumpWgIP := stripPrefix(jumpPeer.Address) // e.g. "10.90.0.1"

	// ==== Subtest 1: server policy => agent iptables (the headline) =========
	t.Run("policy_to_iptables", func(t *testing.T) {
		// WIRETY_JUMP (auth gate) and WIRETY_POLICY (per-destination rules) must
		// both exist once the agent has applied its first policy sync.
		requireChainEventually(ctx, t, jump, "WIRETY_JUMP", "-j DROP")

		// The allow-app policy must materialise as a FORWARD ACCEPT from peer-a's
		// IP to the private service CIDR, inside WIRETY_POLICY.
		policyDump := requireRuleEventually(ctx, t, jump, "WIRETY_POLICY",
			"-s "+regPeerIP+"/32", "-d "+svcCIDR, "-j ACCEPT")
		// Nothing may open the denied service.
		if strings.Contains(policyDump, deniedIP) {
			t.Fatalf("WIRETY_POLICY references the denied service %s:\n%s", deniedIP, policyDump)
		}
		// And the chain must be default-deny (ends in DROP) once policies exist.
		if !strings.Contains(policyDump, "-A WIRETY_POLICY -j DROP") {
			t.Fatalf("WIRETY_POLICY is not default-deny:\n%s", policyDump)
		}
		t.Logf("WIRETY_POLICY:\n%s", policyDump)
	})

	// ==== Subtest 2: private DNS zone resolution ============================
	t.Run("private_dns", func(t *testing.T) {
		// Query the agent's DNS server (bound to the WG IP) from inside the jump
		// container. The query's source is the jump's own WG IP, which is NOT an
		// authenticated peer.
		//
		// Private-zone records resolve to their REAL address whatever the
		// peer's auth state: access control is the jump's iptables, not DNS, so
		// a browser never caches the portal IP for an internal name.
		out := digEventually(ctx, t, jump, jumpWgIP, fqdn, "A", svcIP)
		t.Logf("dig %s @%s (unauthenticated) => %s", fqdn, jumpWgIP, out)

		// OS captive-portal probe hosts are the one thing DNS still steers to
		// the portal for unauthenticated peers, to trip the OS "Sign in to
		// network" prompt.
		digEventually(ctx, t, jump, jumpWgIP, captiveProbeHost, "A", jumpWgIP)
	})

	// ==== Subtest 3: captive-portal auth + gated connectivity ==============
	// A plain WireGuard peer brings its tunnel up and is gated by the jump:
	// HTTP is intercepted by the captive portal even though DNS gives it the
	// real service IP, and OS probe hosts point at the portal.
	// The user then signs in (Dex OIDC) and completes the portal flow exactly
	// as a browser would; the server whitelists the peer, the agent opens the
	// WIRETY_JUMP gate, and the policy decides what is reachable.
	t.Run("captive_portal_connectivity", func(t *testing.T) {
		jumpIP, err := jump.ContainerIP(ctx)
		if err != nil {
			t.Fatalf("jump container ip: %v", err)
		}
		cfg, err := st.admin.peerConfig(ctx, net.ID, regPeer.ID)
		if err != nil {
			t.Fatalf("fetch peer config: %v", err)
		}
		cfg = clientConfig(cfg, fmt.Sprintf("%s:%d", jumpIP, jumpPeer.ListenPort))

		peer := st.startPeer(ctx, t, "peer-a")
		writeFileInContainer(ctx, t, peer, "/etc/wireguard/wg0.conf", cfg)
		mustExec(ctx, t, peer, "wg-quick", "up", "wg0")

		// --- before authentication ------------------------------------------
		// DNS: the private name resolves to the real service IP (the gate is
		// iptables, asserted below); the OS probe host resolves to the portal.
		digEventually(ctx, t, peer, jumpWgIP, fqdn, "A", svcIP)
		digEventually(ctx, t, peer, jumpWgIP, captiveProbeHost, "A", jumpWgIP)

		// HTTP to the (policy-allowed) service is intercepted by the agent's
		// captive portal, which issues a token and redirects to the server.
		var startURL string
		eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
			status, location, err := httpProbe(ctx, peer, "http://"+svcIP+"/")
			if err != nil {
				return err
			}
			if status != "302" || !strings.Contains(location, "/api/v1/captive-portal/start?token=") {
				return fmt.Errorf("want 302 to the captive portal, got %s %q", status, location)
			}
			startURL = location
			return nil
		})
		t.Logf("unauthenticated HTTP intercepted → %s", startURL)

		// The gate is still closed for peer-a.
		gate := []string{"-s " + regPeerIP + "/32", "-j WIRETY_POLICY"}
		if dump, err := dumpChain(ctx, jump, "WIRETY_JUMP"); err != nil {
			t.Fatalf("dump WIRETY_JUMP: %v", err)
		} else if hasRule(dump, gate...) {
			t.Fatalf("peer-a is whitelisted before authenticating:\n%s", dump)
		}

		// --- authenticate through the portal --------------------------------
		session, err := st.wiretySession(ctx, userEmail, userPassword)
		if err != nil {
			t.Fatalf("oidc login: %v", err)
		}
		if err := st.captivePortalLogin(ctx, startURL, session); err != nil {
			t.Fatalf("captive portal login: %v", err)
		}

		// --- after authentication -------------------------------------------
		// The server pushes the whitelist; the agent opens the gate for peer-a.
		requireRuleEventually(ctx, t, jump, "WIRETY_JUMP", gate...)

		// The allowed service is now reachable end-to-end through the tunnel.
		eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
			status, _, err := httpProbe(ctx, peer, "http://"+svcIP+"/")
			if err != nil {
				return err
			}
			if status != "200" {
				return fmt.Errorf("allowed service: want 200, got %s", status)
			}
			return nil
		})

		// DNS still returns the real service IP, and the OS probe host is no
		// longer steered to the portal (several probe hosts are real,
		// HSTS-preloaded sites such as www.apple.com).
		digEventually(ctx, t, peer, jumpWgIP, fqdn, "A", svcIP)
		digEventuallyNot(ctx, t, peer, jumpWgIP, captiveProbeHost, "A", jumpWgIP)

		// The routed-but-not-allowed service stays blocked by WIRETY_POLICY.
		if status, _, err := httpProbe(ctx, peer, "http://"+deniedIP+"/"); err != nil {
			t.Fatalf("probe denied service: %v", err)
		} else if status != "000" {
			t.Fatalf("denied service answered HTTP %s; policy should drop it", status)
		}
	})
}

// captiveProbeHost is an OS captive-portal probe hostname the agent's DNS
// steers to the portal for unauthenticated peers only.
const captiveProbeHost = "captive.apple.com"

// clientConfig adapts a server-generated config for the plain peer container:
// the jump endpoint is replaced by the agent container's address (the jump
// peer was created before that address existed), and the DNS line is dropped
// (wg-quick would need resolvconf; the test queries the jump's DNS explicitly).
func clientConfig(cfg, jumpEndpoint string) string {
	var out []string
	for _, line := range strings.Split(cfg, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "DNS ="):
			continue
		case strings.HasPrefix(trimmed, "Endpoint ="):
			line = "Endpoint = " + jumpEndpoint
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// stripPrefix removes a "/prefix" suffix from an address (e.g. "10.90.0.2/24" -> "10.90.0.2").
func stripPrefix(addr string) string {
	if i := strings.IndexByte(addr, '/'); i != -1 {
		return addr[:i]
	}
	return addr
}
