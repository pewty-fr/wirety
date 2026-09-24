//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestE2EIPv6 runs the TestE2E scenario on a dual-stack deployment: the docker
// network, the Wirety network (cidr_v6), the private services, routes, DNS
// records and policy all carry IPv6 as well as IPv4.
//
// It asserts the IPv6 half of every guarantee — WIRETY6_* chains programmed
// from policy, AAAA records served, IPv6 traffic gated then allowed/denied by
// policy — and the dual-stack auth path: the peer's first intercepted request
// goes over IPv6, and authenticating that token must open both families.
func TestE2EIPv6(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	st := setupStack(ctx, t, withIPv6())

	userTok, err := dexPasswordToken(ctx, st.dexTokenURL, dexClientID, dexClientSecret, userEmail, userPassword)
	if err != nil {
		t.Fatalf("dex user token: %v", err)
	}
	owner, err := newAPIClient(st.apiBaseURL, userTok).me(ctx)
	if err != nil {
		t.Fatalf("user /me: %v", err)
	}

	// ---- provision a dual-stack topology -----------------------------------
	net, err := st.admin.createNetwork(ctx, network{
		Name:         "corp6",
		CIDR:         "10.93.0.0/24",
		CIDRv6:       "fd93::/64",
		DomainSuffix: "e2e.internal",
	})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	jumpPeer, err := st.admin.createPeer(ctx, net.ID, createPeerReq{
		Name: "jump-1", IsJump: true, UseAgent: true,
		Endpoint: "10.255.255.254", ListenPort: 51820,
	})
	if err != nil {
		t.Fatalf("create jump peer: %v", err)
	}
	regPeer, err := st.admin.createPeer(ctx, net.ID, createPeerReq{Name: "peer-a", OwnerID: owner.ID})
	if err != nil {
		t.Fatalf("create regular peer: %v", err)
	}
	if jumpPeer.AddressV6 == "" || regPeer.AddressV6 == "" {
		t.Fatalf("dual-stack peers need IPv6 addresses: jump=%q peer=%q", jumpPeer.AddressV6, regPeer.AddressV6)
	}
	regPeerIP, regPeerIP6 := stripPrefix(regPeer.Address), stripPrefix(regPeer.AddressV6)

	svc, svcIP := st.startPrivateService(ctx, t, "private-svc")
	svcIP6 := st.containerIPv6(ctx, t, svc)
	denied, deniedIP := st.startPrivateService(ctx, t, "denied-svc")
	deniedIP6 := st.containerIPv6(ctx, t, denied)

	// Dual-stack routes: one route carries both families.
	rt, err := st.admin.createRoute(ctx, net.ID, createRouteReq{
		Name: "to-private", DestinationCIDR: svcIP + "/32", DestinationCIDRv6: svcIP6 + "/128",
		JumpPeerID: jumpPeer.ID,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	deniedRt, err := st.admin.createRoute(ctx, net.ID, createRouteReq{
		Name: "to-denied", DestinationCIDR: deniedIP + "/32", DestinationCIDRv6: deniedIP6 + "/128",
		JumpPeerID: jumpPeer.ID,
	})
	if err != nil {
		t.Fatalf("create denied route: %v", err)
	}
	dnsRec, err := st.admin.createDNS(ctx, net.ID, rt.ID, createDNSReq{
		Name: "app", IPAddress: svcIP, IPv6Address: svcIP6,
	})
	if err != nil {
		t.Fatalf("create dns mapping: %v", err)
	}
	fqdn := fmt.Sprintf("%s.%s.%s", dnsRec.Name, net.Name, net.DomainSuffix)

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
	// Allow the private service in both families; the denied one in neither.
	pol, err := st.admin.createPolicy(ctx, net.ID, createPolicyReq{
		Name: "allow-app",
		Rules: []policyRule{
			{Direction: "output", Action: "allow", TargetType: "cidr", Target: svcIP + "/32"},
			{Direction: "output", Action: "allow", TargetType: "cidr", Target: svcIP6 + "/128"},
		},
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if err := st.admin.attachPolicyToGroup(ctx, net.ID, grp.ID, pol.ID); err != nil {
		t.Fatalf("attach policy to group: %v", err)
	}

	jump := st.startJumpAgent(ctx, t, jumpPeer.Token)
	jumpWgIP, jumpWgIP6 := stripPrefix(jumpPeer.Address), stripPrefix(jumpPeer.AddressV6)

	// ==== Subtest 1: server policy => agent ip6tables ======================
	t.Run("policy_to_ip6tables", func(t *testing.T) {
		requireRule6Eventually(ctx, t, jump, "WIRETY6_JUMP", "-j DROP")

		dump := requireRule6Eventually(ctx, t, jump, "WIRETY6_POLICY",
			"-s "+regPeerIP6+"/128", "-d "+svcIP6+"/128", "-j ACCEPT")
		if strings.Contains(dump, deniedIP6) {
			t.Fatalf("WIRETY6_POLICY references the denied service %s:\n%s", deniedIP6, dump)
		}
		if !strings.Contains(dump, "-A WIRETY6_POLICY -j DROP") {
			t.Fatalf("WIRETY6_POLICY is not default-deny:\n%s", dump)
		}
		t.Logf("WIRETY6_POLICY:\n%s", dump)
	})

	// ==== Subtest 2: dual-stack captive portal + gated connectivity ========
	t.Run("captive_portal_dual_stack", func(t *testing.T) {
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
		// Both families resolve to the real service addresses (DNS is not the
		// gate); the OS probe host is steered to the portal, including when
		// the query reaches the agent over its IPv6 DNS listener.
		digEventually(ctx, t, peer, jumpWgIP, fqdn, "A", svcIP)
		digEventually(ctx, t, peer, jumpWgIP, fqdn, "AAAA", svcIP6)
		digEventually(ctx, t, peer, jumpWgIP6, captiveProbeHost, "A", jumpWgIP)

		// IPv6 must not bypass the gate: HTTP to the service's IPv6 address is
		// intercepted by the captive portal, exactly like IPv4.
		var startURL string
		eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
			status, location, err := httpProbe(ctx, peer, "http://["+svcIP6+"]/")
			if err != nil {
				return err
			}
			if status != "302" || !strings.Contains(location, "/api/v1/captive-portal/start?token=") {
				return fmt.Errorf("want 302 to the captive portal over IPv6, got %s %q", status, location)
			}
			startURL = location
			return nil
		})
		t.Logf("unauthenticated IPv6 HTTP intercepted → %s", startURL)

		gate4 := []string{"-s " + regPeerIP + "/32", "-j WIRETY_POLICY"}
		gate6 := []string{"-s " + regPeerIP6 + "/128", "-j WIRETY6_POLICY"}
		if dump, err := dumpChain6(ctx, jump, "WIRETY6_JUMP"); err != nil {
			t.Fatalf("dump WIRETY6_JUMP: %v", err)
		} else if hasRule(dump, gate6...) {
			t.Fatalf("peer-a's IPv6 is whitelisted before authenticating:\n%s", dump)
		}

		// --- authenticate the token that was issued for the IPv6 address ----
		session, err := st.wiretySession(ctx, userEmail, userPassword)
		if err != nil {
			t.Fatalf("oidc login: %v", err)
		}
		if err := st.captivePortalLogin(ctx, startURL, session); err != nil {
			t.Fatalf("captive portal login: %v", err)
		}

		// --- after authentication: both families are open -------------------
		requireRuleEventually(ctx, t, jump, "WIRETY_JUMP", gate4...)
		requireRule6Eventually(ctx, t, jump, "WIRETY6_JUMP", gate6...)

		for _, url := range []string{"http://[" + svcIP6 + "]/", "http://" + svcIP + "/"} {
			eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
				status, _, err := httpProbe(ctx, peer, url)
				if err != nil {
					return err
				}
				if status != "200" {
					return fmt.Errorf("allowed service %s: want 200, got %s", url, status)
				}
				return nil
			})
		}

		// Records are served over the agent's IPv6 DNS listener too...
		digEventually(ctx, t, peer, jumpWgIP6, fqdn, "AAAA", svcIP6)
		digEventually(ctx, t, peer, jumpWgIP6, fqdn, "A", svcIP)
		// ...and the probe host is released for the now-authenticated peer even
		// when it asks over IPv6: the agent must map the peer's IPv6 source to
		// its IPv4-keyed whitelist entry.
		digEventuallyNot(ctx, t, peer, jumpWgIP6, captiveProbeHost, "A", jumpWgIP)

		// The routed-but-not-allowed service stays blocked over IPv6.
		if status, _, err := httpProbe(ctx, peer, "http://["+deniedIP6+"]/"); err != nil {
			t.Fatalf("probe denied service over IPv6: %v", err)
		} else if status != "000" {
			t.Fatalf("denied service answered HTTP %s over IPv6; WIRETY6_POLICY should drop it", status)
		}
	})
}
