//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// TestAttackScenarios plays an attacker against a live deployment and checks
// each defense holds. The topology: a private service behind the jump peer,
// the victim's device (peer-a, signed in through the captive portal), a second
// device that never signs in (peer-b), and an attacker device holding a copy
// of peer-a's WireGuard config (stolen or shared config).
func TestAttackScenarios(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	st := setupStack(ctx, t)
	userTok, err := dexPasswordToken(ctx, st.dexTokenURL, dexClientID, dexClientSecret, userEmail, userPassword)
	if err != nil {
		t.Fatalf("dex user token: %v", err)
	}
	owner, err := newAPIClient(st.apiBaseURL, userTok).me(ctx)
	if err != nil {
		t.Fatalf("user /me: %v", err)
	}

	net, err := st.admin.createNetwork(ctx, network{Name: "attack", CIDR: "10.98.0.0/24"})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	jumpPeer, err := st.admin.createPeer(ctx, net.ID, createPeerReq{
		Name: "jump-1", IsJump: true, UseAgent: true, Endpoint: "10.255.255.254", ListenPort: 51820,
	})
	if err != nil {
		t.Fatalf("create jump peer: %v", err)
	}
	peerA, err := st.admin.createPeer(ctx, net.ID, createPeerReq{Name: "peer-a", OwnerID: owner.ID})
	if err != nil {
		t.Fatalf("create peer-a: %v", err)
	}
	peerB, err := st.admin.createPeer(ctx, net.ID, createPeerReq{Name: "peer-b", OwnerID: owner.ID})
	if err != nil {
		t.Fatalf("create peer-b: %v", err)
	}
	// A regular device that runs the agent: its enrollment token is an API
	// credential an attacker could lift from the device.
	agentPeer, err := st.admin.createPeer(ctx, net.ID, createPeerReq{Name: "agent-laptop", OwnerID: owner.ID, UseAgent: true})
	if err != nil {
		t.Fatalf("create agent peer: %v", err)
	}

	svc, svcIP := st.startPrivateService(ctx, t, "private-svc")
	rt, err := st.admin.createRoute(ctx, net.ID, createRouteReq{Name: "to-svc", DestinationCIDR: svcIP + "/32", JumpPeerID: jumpPeer.ID})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	grp, err := st.admin.createGroup(ctx, net.ID, "users")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	for _, p := range []peer{peerA, peerB} {
		if err := st.admin.addPeerToGroup(ctx, net.ID, grp.ID, p.ID); err != nil {
			t.Fatalf("add peer to group: %v", err)
		}
	}
	if err := st.admin.attachRouteToGroup(ctx, net.ID, grp.ID, rt.ID); err != nil {
		t.Fatalf("attach route: %v", err)
	}
	pol, err := st.admin.createPolicy(ctx, net.ID, createPolicyReq{
		Name:  "allow-svc",
		Rules: []policyRule{{Direction: "output", Action: "allow", TargetType: "cidr", Target: svcIP + "/32"}},
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if err := st.admin.attachPolicyToGroup(ctx, net.ID, grp.ID, pol.ID); err != nil {
		t.Fatalf("attach policy: %v", err)
	}

	// SSH on the jump host is opened to signed-in peers.
	jump := st.startJumpAgent(ctx, t, jumpPeer.Token, "-server", "http://server:8080", "-jump-host-ports", "22/tcp")
	jumpIP, err := jump.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("jump container ip: %v", err)
	}
	jumpEndpoint := fmt.Sprintf("%s:%d", jumpIP, jumpPeer.ListenPort)
	jumpWgIP := stripPrefix(jumpPeer.Address)

	cfgA, err := st.admin.peerConfig(ctx, net.ID, peerA.ID)
	if err != nil {
		t.Fatalf("peer-a config: %v", err)
	}
	cfgB, err := st.admin.peerConfig(ctx, net.ID, peerB.ID)
	if err != nil {
		t.Fatalf("peer-b config: %v", err)
	}
	victim := st.startPeer(ctx, t, "victim")
	writeFileInContainer(ctx, t, victim, "/etc/wireguard/wg0.conf", clientConfig(cfgA, jumpEndpoint))
	mustExec(ctx, t, victim, "wg-quick", "up", "wg0")
	other := st.startPeer(ctx, t, "other-device")
	writeFileInContainer(ctx, t, other, "/etc/wireguard/wg0.conf", clientConfig(cfgB, jumpEndpoint))
	mustExec(ctx, t, other, "wg-quick", "up", "wg0")
	// The attacker holds a copy of the victim's config (same private key and VPN IP).
	attacker := st.startPeer(ctx, t, "attacker")
	writeFileInContainer(ctx, t, attacker, "/etc/wireguard/wg0.conf", clientConfig(cfgA, jumpEndpoint))
	attackerIP, err := attacker.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("attacker ip: %v", err)
	}

	session, err := st.wiretySession(ctx, userEmail, userPassword)
	if err != nil {
		t.Fatalf("oidc login: %v", err)
	}
	peerAIP := stripPrefix(peerA.Address)
	gateA := []string{"-s " + peerAIP + "/32", "-j WIRETY_POLICY"}
	svcURL := "http://" + svcIP + "/"

	// The victim signs in.
	token, cpState := interceptAndStart(ctx, t, st, victim, svcIP)
	if err := st.captivePortalAuthenticate(ctx, token, session, cpState); err != nil {
		t.Fatalf("victim sign-in: %v", err)
	}
	requireRuleEventually(ctx, t, jump, "WIRETY_JUMP", gateA...)
	requireHTTPEventually(ctx, t, victim, svcURL, "200")

	// ==== An unauthenticated device reaches nothing but the portal ==========
	t.Run("unauthenticated_device_blocked", func(t *testing.T) {
		if status, loc, err := httpProbe(ctx, other, svcURL); err != nil {
			t.Fatal(err)
		} else if status != "302" || !strings.Contains(loc, "/captive-portal/start") {
			t.Fatalf("HTTP to the private service: want a redirect to the portal, got %s %q", status, loc)
		}
		// Anything that is not port-80 HTTP is dropped or reset.
		if code, _, _ := execInContainer(ctx, other, "ping", "-c", "2", "-W", "2", svcIP); code == 0 {
			t.Fatal("ICMP to the private service went through before authentication")
		}
		if status, _, _ := httpProbe(ctx, other, "https://"+svcIP+"/"); status != "000" {
			t.Fatalf("HTTPS to the private service answered %s before authentication", status)
		}
	})

	// ==== Another service on another port: SSH ============================
	// An SSH server runs on the private host and on the jump host itself; the
	// jump also runs a metrics exporter (port 9100) that is not opened.
	startFakeSSH(ctx, t, svc)
	startFakeSSH(ctx, t, jump)
	startBannerServer(ctx, t, jump, 9100, "metrics")
	// Controls, so an absent banner below means "blocked", not "not
	// listening": the jump's servers answer locally, and the signed-in device
	// reaches the private host's SSH (its policy allows the whole host).
	if sshBanner(ctx, t, jump, "127.0.0.1") == "" || readBanner(ctx, t, jump, "127.0.0.1", 9100, "metrics") == "" {
		t.Fatal("fake servers on the jump do not answer locally")
	}
	if banner := sshBanner(ctx, t, victim, svcIP); banner == "" {
		t.Fatal("signed-in device cannot reach SSH on the private host its policy allows")
	}

	t.Run("ssh_on_private_host_unauthenticated", func(t *testing.T) {
		if banner := sshBanner(ctx, t, other, svcIP); banner != "" {
			t.Fatalf("unauthenticated device reached SSH on the private host: %q", banner)
		}
	})
	// The jump host is reached through its WireGuard address: its own
	// services (sshd, exporters…) must not be exposed to devices that have
	// not signed in.
	t.Run("ssh_on_jump_host_unauthenticated", func(t *testing.T) {
		if banner := sshBanner(ctx, t, other, jumpWgIP); banner != "" {
			t.Fatalf("unauthenticated device reached SSH on the jump host: %q", banner)
		}
	})
	// -jump-host-ports 22/tcp opens the jump's SSH to signed-in devices only.
	t.Run("ssh_on_jump_host_authenticated", func(t *testing.T) {
		if banner := sshBanner(ctx, t, victim, jumpWgIP); banner == "" {
			t.Fatal("signed-in device cannot reach SSH on the jump host (opened with -jump-host-ports)")
		}
	})
	// Policies govern what is routed through the jump, not the jump itself:
	// a port that is not opened stays closed even to a signed-in device.
	t.Run("jump_host_port_not_opened", func(t *testing.T) {
		if banner := readBanner(ctx, t, victim, jumpWgIP, 9100, "metrics"); banner != "" {
			t.Fatalf("signed-in device reached port 9100 on the jump host: %q", banner)
		}
	})
	// Before signing in, a device still gets what it needs from the jump:
	// DNS, the captive portal (checked above), ping.
	t.Run("jump_services_reachable_unauthenticated", func(t *testing.T) {
		if code, out, _ := execInContainer(ctx, other, "dig", "+short", "+time=3", "+tries=1", "@"+jumpWgIP, "captive.apple.com"); code != 0 || strings.TrimSpace(out) == "" {
			t.Fatalf("DNS on the jump unreachable before sign-in (exit %d): %q", code, out)
		}
		if code, _, _ := execInContainer(ctx, other, "ping", "-c", "1", "-W", "3", jumpWgIP); code != 0 {
			t.Fatal("cannot ping the jump before sign-in")
		}
	})
	// No lock-out: the lockdown only filters the WireGuard interface. An admin
	// on the private network (VPC, through a bastion) keeps SSH access to the
	// jump — here from the private host, which shares the jump's LAN.
	t.Run("ssh_on_jump_host_from_private_network", func(t *testing.T) {
		if banner := sshBanner(ctx, t, svc, jumpIP); banner == "" {
			t.Fatal("SSH on the jump is unreachable from the private network")
		}
	})

	// ==== Spoofing a signed-in device's VPN address ==========================
	// peer-b claims peer-a's VPN IP as its source. WireGuard's cryptokey
	// routing on the jump only accepts, from peer-b's key, packets sourced from
	// peer-b's own address: the forged packets are dropped before iptables.
	t.Run("spoofed_vpn_source_dropped", func(t *testing.T) {
		mustExec(ctx, t, other, "ip", "address", "add", peerAIP+"/32", "dev", "wg0")
		defer func() { _, _, _ = execInContainer(ctx, other, "ip", "address", "del", peerAIP+"/32", "dev", "wg0") }()
		_, out, err := execInContainer(ctx, other, "curl", "-s", "-o", "/dev/null", "--max-time", "5",
			"--interface", peerAIP, "-w", "%{http_code}", svcURL)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(out) != "000" {
			t.Fatalf("request forged with the signed-in peer's source got HTTP %s", strings.TrimSpace(out))
		}
	})

	// ==== A lifted agent enrollment token cannot mint portal tokens ==========
	// Only jump peers issue captive-portal tokens; a regular device's
	// enrollment token must not be usable to forge a sign-in for any peer.
	t.Run("regular_agent_token_cannot_mint_portal_tokens", func(t *testing.T) {
		err := newAPIClient(st.apiBaseURL, agentPeer.Token).do(ctx, http.MethodPost, "/captive-portal/token",
			map[string]string{"peer_ip": peerAIP, "peer_endpoint": attackerIP + ":51820"}, nil)
		if err == nil || !strings.Contains(err.Error(), "status 403") {
			t.Fatalf("want 403, got %v", err)
		}
	})

	// ==== Stolen config, owner offline ======================================
	// The victim's session is still valid, but the attacker connects from
	// another public IP: the jump binds the session to the endpoint that
	// signed in, so the attacker only gets the captive portal — whose SSO it
	// cannot pass (it does not own the peer's account).
	t.Run("stolen_config_owner_offline", func(t *testing.T) {
		mustExec(ctx, t, victim, "wg-quick", "down", "wg0")
		mustExec(ctx, t, attacker, "wg-quick", "up", "wg0")
		defer mustExec(ctx, t, attacker, "wg-quick", "down", "wg0")

		eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
			status, loc, err := httpProbe(ctx, attacker, svcURL)
			if err != nil {
				return err
			}
			if status == "200" {
				return fmt.Errorf("attacker reached the private service with the victim's session")
			}
			if status != "302" || !strings.Contains(loc, "/captive-portal/start") {
				return fmt.Errorf("want the captive portal, got %s %q", status, loc)
			}
			return nil
		})
		// Never 200 while it stays connected.
		for i := 0; i < 5; i++ {
			if status, _, _ := httpProbe(ctx, attacker, svcURL); status == "200" {
				t.Fatal("attacker reached the private service with the victim's session")
			}
			time.Sleep(time.Second)
		}
	})

	// ==== Phishing with a portal link =======================================
	// The attacker triggers the portal from the victim's config and sends the
	// resulting link to the victim. The link carries a token bound to the
	// attacker's endpoint: the portal page must show the victim that public IP
	// (not theirs) so they can refuse.
	t.Run("phishing_link_shows_attacker_ip", func(t *testing.T) {
		mustExec(ctx, t, attacker, "wg-quick", "up", "wg0")
		defer mustExec(ctx, t, attacker, "wg-quick", "down", "wg0")
		token, _ := interceptAndStart(ctx, t, st, attacker, svcIP)
		preview, err := st.captivePortalPreview(ctx, token, session)
		if err != nil {
			t.Fatalf("preview: %v", err)
		}
		if preview.EndpointIP != attackerIP {
			t.Fatalf("portal page shows public IP %q for a token requested from %q", preview.EndpointIP, attackerIP)
		}
	})

	// ==== Stolen config used while the owner is online ======================
	// Two devices with the same key make the endpoint seen by the jump flip
	// back and forth. The agent reports it as a takeover; the attacker's
	// source is then dropped at the jump's physical interface, and the owner
	// keeps working.
	t.Run("stolen_config_concurrent_use_denylisted", func(t *testing.T) {
		mustExec(ctx, t, victim, "wg-quick", "up", "wg0")
		requireHTTPEventually(ctx, t, victim, svcURL, "200")
		mustExec(ctx, t, attacker, "wg-quick", "up", "wg0")
		// Both devices keep talking to the jump, as two live devices would.
		for _, c := range []testcontainers.Container{victim, attacker} {
			mustExec(ctx, t, c, "sh", "-c", "ping -i 1 "+jumpWgIP+" >/dev/null 2>&1 &")
		}

		eventually(t, 3*time.Minute, defaultPollInterval, func() error {
			dump, err := dumpChain(ctx, jump, "WIRETY_WGDENY")
			if err != nil {
				return err
			}
			if !hasRule(dump, "-s "+attackerIP+"/32", "-j DROP") {
				return fmt.Errorf("attacker source not denylisted yet:\n%s", dump)
			}
			return nil
		})
		// The attacker is cut off; the owner keeps (or regains) access.
		requireHTTPEventually(ctx, t, victim, svcURL, "200")
		if status, _, _ := httpProbe(ctx, attacker, svcURL); status == "200" {
			t.Fatal("denylisted attacker still reaches the private service")
		}
	})
}

// requireHTTPEventually polls url from inside c until it answers want.
func requireHTTPEventually(ctx context.Context, t *testing.T, c testcontainers.Container, url, want string) {
	t.Helper()
	eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
		status, _, err := httpProbe(ctx, c, url)
		if err != nil {
			return err
		}
		if status != want {
			return fmt.Errorf("GET %s: want %s, got %s", url, want, status)
		}
		return nil
	})
}
