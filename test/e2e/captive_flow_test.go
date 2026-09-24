//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestCaptivePortalServerFlow exercises the server half of the captive-portal
// flow without WireGuard, so it runs anywhere Docker runs. The test plays the
// jump agent (it mints the captive token with the jump's enrollment token, as
// the agent does when it intercepts a peer's HTTP request) and then the user's
// browser: Dex OIDC login, /start bouncer, /authenticate.
//
// It covers the security checks that gate whitelisting: the browser-binding
// cookie (phishing defense) and peer ownership, before the happy path.
// TestE2E/captive_portal_connectivity runs the same flow with a real peer
// behind a real agent.
func TestCaptivePortalServerFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
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

	net, err := st.admin.createNetwork(ctx, network{Name: "cp", CIDR: "10.92.0.0/24"})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	jump, err := st.admin.createPeer(ctx, net.ID, createPeerReq{
		Name: "jump-1", IsJump: true, UseAgent: true, Endpoint: "10.255.255.254", ListenPort: 51820,
	})
	if err != nil {
		t.Fatalf("create jump peer: %v", err)
	}
	peerA, err := st.admin.createPeer(ctx, net.ID, createPeerReq{Name: "peer-a", OwnerID: owner.ID})
	if err != nil {
		t.Fatalf("create peer: %v", err)
	}

	// Act as the agent: mint a captive token for peer-a.
	agent := newAPIClient(st.apiBaseURL, jump.Token)
	var cpt struct {
		Token string `json:"token"`
	}
	if err := agent.do(ctx, http.MethodPost, "/captive-portal/token", map[string]string{
		"peer_ip": stripPrefix(peerA.Address), "peer_endpoint": "172.30.0.9:40000",
	}, &cpt); err != nil {
		t.Fatalf("create captive token: %v", err)
	}
	startURL := "http://server:8080/api/v1/captive-portal/start?token=" + cpt.Token

	userSession, err := st.wiretySession(ctx, userEmail, userPassword)
	if err != nil {
		t.Fatalf("oidc login (user): %v", err)
	}

	// Phishing defense: a token whose URL never went through /start in this
	// browser (no wirety_cp_state cookie) must be refused.
	if err := st.captivePortalAuthenticate(ctx, cpt.Token, userSession, ""); err == nil ||
		!strings.Contains(err.Error(), "status 401") {
		t.Fatalf("authenticate without browser-binding cookie: want 401, got %v", err)
	}

	token, cpState, err := st.captivePortalStart(ctx, startURL)
	if err != nil {
		t.Fatalf("captive-portal start: %v", err)
	}

	// Ownership: another user (here the admin) cannot whitelist peer-a.
	adminSession, err := st.wiretySession(ctx, adminEmail, adminPassword)
	if err != nil {
		t.Fatalf("oidc login (admin): %v", err)
	}
	if err := st.captivePortalAuthenticate(ctx, token, adminSession, cpState); err == nil ||
		!strings.Contains(err.Error(), "belongs to another user") {
		t.Fatalf("authenticate as non-owner: want ownership refusal, got %v", err)
	}

	// Happy path: the owner, in the browser that went through /start.
	if err := st.captivePortalAuthenticate(ctx, token, userSession, cpState); err != nil {
		t.Fatalf("authenticate as owner: %v", err)
	}
}
