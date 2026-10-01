//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// TestCaptivePortalSessionLifecycle follows captive-portal sessions from
// sign-in to sign-out, with a short CAPTIVE_PORTAL_SESSION_TTL. The user
// reaches a private host over HTTP and SSH, and the jump host over SSH (opened
// to signed-in peers with -jump-host-ports), keeping SSH sessions open:
//
//   - the peer stays signed in while its tunnel is live, across several jump
//     heartbeats — including a peer flagged use_agent, which used to be signed
//     out on the first heartbeat;
//   - the access ends when the session duration is reached: HTTP goes back to
//     the portal, new SSH connections are dropped, and the SSH sessions opened
//     while signed in stop carrying data;
//   - back on the captive portal, the page tells why the previous session
//     ended;
//   - after signing in again, a revocation from the dashboard cuts HTTP and
//     SSH the same way;
//   - the peer's access history records sign-in → expired → sign-in → revoked.
func TestCaptivePortalSessionLifecycle(t *testing.T) {
	const sessionTTL = 90 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	st := setupStack(ctx, t, withServerEnv("CAPTIVE_PORTAL_SESSION_TTL", sessionTTL.String()))

	userTok, err := dexPasswordToken(ctx, st.dexTokenURL, dexClientID, dexClientSecret, userEmail, userPassword)
	if err != nil {
		t.Fatalf("dex user token: %v", err)
	}
	owner, err := newAPIClient(st.apiBaseURL, userTok).me(ctx)
	if err != nil {
		t.Fatalf("user /me: %v", err)
	}

	net, err := st.admin.createNetwork(ctx, network{Name: "lifecycle", CIDR: "10.97.0.0/24"})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	jumpPeer, err := st.admin.createPeer(ctx, net.ID, createPeerReq{
		Name: "jump-1", IsJump: true, UseAgent: true, Endpoint: "10.255.255.254", ListenPort: 51820,
	})
	if err != nil {
		t.Fatalf("create jump peer: %v", err)
	}
	regPeer, err := st.admin.createPeer(ctx, net.ID, createPeerReq{Name: "peer-a", OwnerID: owner.ID, UseAgent: true})
	if err != nil {
		t.Fatalf("create peer: %v", err)
	}
	svc, svcIP := st.startPrivateService(ctx, t, "private-svc")
	startFakeSSH(ctx, t, svc)
	rt, err := st.admin.createRoute(ctx, net.ID, createRouteReq{Name: "to-svc", DestinationCIDR: svcIP + "/32", JumpPeerID: jumpPeer.ID})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	grp, err := st.admin.createGroup(ctx, net.ID, "users")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := st.admin.addPeerToGroup(ctx, net.ID, grp.ID, regPeer.ID); err != nil {
		t.Fatalf("add peer to group: %v", err)
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

	jump := st.startJumpAgent(ctx, t, jumpPeer.Token, "-server", "http://server:8080", "-jump-host-ports", "22/tcp")
	startFakeSSH(ctx, t, jump)
	jumpWgIP := stripPrefix(jumpPeer.Address)
	jumpIP, err := jump.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("jump container ip: %v", err)
	}
	cfg, err := st.admin.peerConfig(ctx, net.ID, regPeer.ID)
	if err != nil {
		t.Fatalf("fetch peer config: %v", err)
	}
	peer := st.startPeer(ctx, t, "peer-a")
	writeFileInContainer(ctx, t, peer, "/etc/wireguard/wg0.conf", clientConfig(cfg, fmt.Sprintf("%s:%d", jumpIP, jumpPeer.ListenPort)))
	mustExec(ctx, t, peer, "wg-quick", "up", "wg0")

	session, err := st.wiretySession(ctx, userEmail, userPassword)
	if err != nil {
		t.Fatalf("oidc login: %v", err)
	}
	gate := []string{"-s " + stripPrefix(regPeer.Address) + "/32", "-j WIRETY_POLICY"}
	svcURL := "http://" + svcIP + "/"
	signedOut := func() {
		t.Helper()
		eventually(t, 90*time.Second, defaultPollInterval, func() error {
			dump, err := dumpChain(ctx, jump, "WIRETY_JUMP")
			if err != nil {
				return err
			}
			if hasRule(dump, gate...) {
				return fmt.Errorf("still signed in")
			}
			return nil
		})
	}

	// --- first sign-in: HTTP and SSH to the private host, an SSH session open ---
	token, cpState := interceptAndStart(ctx, t, st, peer, svcIP)
	if err := st.captivePortalAuthenticate(ctx, token, session, cpState); err != nil {
		t.Fatalf("captive portal login: %v", err)
	}
	signedInAt := time.Now()
	requireRuleEventually(ctx, t, jump, "WIRETY_JUMP", gate...)
	sessions := requireSignedInAccess(ctx, t, peer, svcURL, svcIP, jumpWgIP)

	// --- the access survives the jump heartbeats (every 30 s) -------------------
	t.Run("access_survives_heartbeats", func(t *testing.T) {
		time.Sleep(time.Until(signedInAt.Add(70 * time.Second)))
		if dump, err := dumpChain(ctx, jump, "WIRETY_JUMP"); err != nil {
			t.Fatalf("dump WIRETY_JUMP: %v", err)
		} else if !hasRule(dump, gate...) {
			t.Fatalf("peer signed out after %s with a live tunnel (session duration %s)", time.Since(signedInAt).Round(time.Second), sessionTTL)
		}
		for _, ssh := range sessions {
			if !ssh.carriesData(ctx, t, 5*time.Second) {
				t.Fatalf("the SSH session to %s stopped while signed in", ssh.host)
			}
		}
	})

	// --- the session duration is reached: HTTP and SSH cut ------------------------
	signedOut()
	t.Logf("signed out %s after sign-in", time.Since(signedInAt).Round(time.Second))
	t.Run("expired_session_cuts_http_and_ssh", func(t *testing.T) {
		requireSignedOutAccess(ctx, t, peer, svcURL, sessions)
	})
	closeSessions(ctx, sessions)

	// --- back on the portal: it says why the previous session ended -------------
	token, cpState = interceptAndStart(ctx, t, st, peer, svcIP)
	preview, err := st.captivePortalPreview(ctx, token, session)
	if err != nil {
		t.Fatalf("captive portal preview: %v", err)
	}
	if preview.LastSignOut == nil || preview.LastSignOut.Event != "expired" {
		t.Fatalf("portal page does not show the expiry as the last sign-out: %+v", preview.LastSignOut)
	}
	if err := st.captivePortalAuthenticate(ctx, token, session, cpState); err != nil {
		t.Fatalf("second captive portal login: %v", err)
	}
	requireRuleEventually(ctx, t, jump, "WIRETY_JUMP", gate...)
	sessions = requireSignedInAccess(ctx, t, peer, svcURL, svcIP, jumpWgIP)

	// --- revoked from the dashboard: HTTP and SSH cut -----------------------------
	if err := st.admin.revokePeerAuth(ctx, net.ID, regPeer.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	signedOut()
	t.Run("revoked_session_cuts_http_and_ssh", func(t *testing.T) {
		requireSignedOutAccess(ctx, t, peer, svcURL, sessions)
	})
	closeSessions(ctx, sessions)

	// --- the history records the whole session ------------------------------------
	events, err := st.admin.captivePortalEvents(ctx, net.ID, regPeer.ID)
	if err != nil {
		t.Fatalf("captive portal events: %v", err)
	}
	var got []string
	for _, e := range events {
		got = append(got, e.Event)
	}
	if strings.Join(got, ",") != "revoked,authenticated,expired,authenticated" {
		t.Fatalf("history (newest first) = %v, want [revoked authenticated expired authenticated]", got)
	}
	// Each sign-in is bound to the device's public IP — including the first
	// one, requested right as the tunnel came up (the jump must not mint a
	// token before it knows the peer's endpoint: the session would be bound to
	// no IP and usable with a stolen config from anywhere).
	peerPublicIP, err := peer.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("peer ip: %v", err)
	}
	for _, e := range events {
		if e.Event == "authenticated" && e.Detail != "from public IP "+peerPublicIP {
			t.Errorf("sign-in not bound to the device's public IP %s: %+v", peerPublicIP, e)
		}
	}
	t.Logf("history: %+v", events)
}

// requireSignedInAccess checks a signed-in peer reaches svcURL over HTTP and
// each of sshHosts over SSH, and returns an SSH session it keeps open to each.
func requireSignedInAccess(ctx context.Context, t *testing.T, peer testcontainers.Container, svcURL string, sshHosts ...string) []*sshSession {
	t.Helper()
	requireHTTPEventually(ctx, t, peer, svcURL, "200")
	var sessions []*sshSession
	for _, host := range sshHosts {
		if banner := sshBanner(ctx, t, peer, host); banner == "" {
			t.Fatalf("signed-in peer cannot reach SSH on %s", host)
		}
		ssh := openSSHSession(ctx, t, peer, host)
		if !ssh.carriesData(ctx, t, 5*time.Second) {
			t.Fatalf("SSH session to %s carries no data while signed in", host)
		}
		sessions = append(sessions, ssh)
	}
	return sessions
}

// requireSignedOutAccess checks a signed-out peer gets nothing but the portal:
// HTTP is redirected to it, new SSH connections are dropped, and the SSH
// sessions opened while signed in no longer carry data.
func requireSignedOutAccess(ctx context.Context, t *testing.T, peer testcontainers.Container, svcURL string, sessions []*sshSession) {
	t.Helper()
	for _, ssh := range sessions {
		if ssh.carriesData(ctx, t, 5*time.Second) {
			t.Errorf("the SSH session to %s opened while signed in still carries data after the sign-out", ssh.host)
		}
		if banner := sshBanner(ctx, t, peer, ssh.host); banner != "" {
			t.Errorf("new SSH connection reached %s after the sign-out: %q", ssh.host, banner)
		}
	}
	if status, loc, err := httpProbe(ctx, peer, svcURL); err != nil {
		t.Error(err)
	} else if status != "302" || !strings.Contains(loc, "/captive-portal/start") {
		t.Errorf("HTTP to the private host after the sign-out: want a redirect to the portal, got %s %q", status, loc)
	}
}

func closeSessions(ctx context.Context, sessions []*sshSession) {
	for _, ssh := range sessions {
		ssh.close(ctx)
	}
}

// interceptAndStart makes the peer's HTTP request get intercepted by the
// captive portal, then follows the redirect to /start like a browser, and
// returns the token and its browser-binding cookie.
func interceptAndStart(ctx context.Context, t *testing.T, st *stack, peer testcontainers.Container, target string) (token, cpState string) {
	t.Helper()
	var startURL string
	eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
		status, location, err := httpProbe(ctx, peer, "http://"+target+"/")
		if err != nil {
			return err
		}
		if status != "302" || !strings.Contains(location, "/api/v1/captive-portal/start?token=") {
			return fmt.Errorf("want 302 to the captive portal, got %s %q", status, location)
		}
		startURL = location
		return nil
	})
	token, cpState, err := st.captivePortalStart(ctx, startURL)
	if err != nil {
		t.Fatalf("captive portal start: %v", err)
	}
	return token, cpState
}
