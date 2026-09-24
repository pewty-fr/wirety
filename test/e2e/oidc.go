//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// dexPasswordToken obtains an OIDC id_token from Dex using the resource-owner
// password grant (grant_type=password, enabled by `enablePasswordDB: true` in
// the Dex config). The returned token is used as the admin Bearer for the
// Wirety REST API — the first user the server sees is auto-promoted to
// administrator.
//
// dexTokenURL is reached from the host via Dex's mapped port, but the token's
// `iss` claim is Dex's configured issuer (the in-network URL the server
// validates against), so host-vs-network hostnames don't matter here.
func dexPasswordToken(ctx context.Context, dexTokenURL, clientID, clientSecret, username, password string) (string, error) {
	form := url.Values{
		"grant_type":    {"password"},
		"scope":         {"openid profile email"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"username":      {username},
		"password":      {password},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dexTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("dex token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dex token: status %d: %s", resp.StatusCode, string(body))
	}
	var tok struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("dex token: decode: %w (body=%s)", err, string(body))
	}
	if tok.IDToken == "" {
		return "", fmt.Errorf("dex token: no id_token in response: %s", string(body))
	}
	return tok.IDToken, nil
}

// inNetworkClient returns an HTTP client for the host-side test process that
// speaks the in-network URLs (http://dex:5556, http://server:8080) exactly as a
// browser on a VPN peer would: the dialer rewrites those host:port pairs to the
// containers' mapped ports. This keeps Dex redirects (which carry the issuer
// host) and the agent's captive-portal redirect URL usable verbatim.
//
// Redirects are never followed past stopAt (a URL prefix); the response
// carrying that Location is returned instead. Empty stopAt disables following
// altogether.
func (s *stack) inNetworkClient(stopAt string) *http.Client {
	hostMap := map[string]string{
		"dex:5556":    s.dexHostPort,
		"server:8080": s.serverHostPort,
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if mapped, ok := hostMap[addr]; ok {
					addr = mapped
				}
				return dialer.DialContext(ctx, network, addr)
			},
		},
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if stopAt == "" || strings.HasPrefix(req.URL.String(), stopAt) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// wiretySession performs the browser-less OIDC authorization-code flow against
// Dex and exchanges the code at the Wirety server's /auth/token endpoint, the
// same way the frontend does. It returns the session hash (the value of the
// wirety_session cookie), which captive-portal /authenticate requires.
//
//  1. GET Dex /auth → Dex redirects to its local-password login page.
//  2. POST the credentials to that page (skipApprovalScreen is on), Dex
//     redirects to redirect_uri?code=… — captured, not followed.
//  3. POST {code, redirect_uri} to /api/v1/auth/token; the server exchanges the
//     code with Dex server-to-server and creates the session.
func (s *stack) wiretySession(ctx context.Context, username, password string) (string, error) {
	client := s.inNetworkClient(dexRedirectURI)

	authQ := url.Values{
		"client_id":     {dexClientID},
		"redirect_uri":  {dexRedirectURI},
		"response_type": {"code"},
		"scope":         {"openid profile email"},
		"state":         {"e2e-state"},
		"nonce":         {"e2e-nonce"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dexIssuer+"/auth?"+authQ.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("dex authorize: %w", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dex authorize: expected login page, got status %d", resp.StatusCode)
	}
	loginURL := resp.Request.URL.String() // …/dex/auth/local/login?back=&state=…

	form := url.Values{"login": {username}, "password": {password}}
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("dex login: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	code := codeFromLocation(resp)
	if code == "" {
		return "", fmt.Errorf("dex login: no authorization code (status %d, location %q): %.300s",
			resp.StatusCode, resp.Header.Get("Location"), body)
	}

	tokenBody, _ := json.Marshal(map[string]string{"code": code, "redirect_uri": dexRedirectURI})
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, "http://server:8080/api/v1/auth/token", bytes.NewReader(tokenBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("wirety /auth/token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("wirety /auth/token: status %d: %s", resp.StatusCode, body)
	}
	var tok struct {
		SessionHash string `json:"session_hash"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.SessionHash == "" {
		return "", fmt.Errorf("wirety /auth/token: no session_hash in %s", body)
	}
	return tok.SessionHash, nil
}

// captivePortalLogin plays the browser's part of the captive-portal flow for a
// redirect URL issued by the agent (http://server:8080/api/v1/captive-portal/
// start?token=…): hit /start to receive the browser-binding cookie, then POST
// /authenticate with that cookie + the user's session.
func (s *stack) captivePortalLogin(ctx context.Context, startURL, sessionHash string) error {
	token, cpState, err := s.captivePortalStart(ctx, startURL)
	if err != nil {
		return err
	}
	return s.captivePortalAuthenticate(ctx, token, sessionHash, cpState)
}

// captivePortalStart GETs the /start bouncer and returns the captive token and
// the wirety_cp_state browser-binding cookie it sets.
func (s *stack) captivePortalStart(ctx context.Context, startURL string) (token, cpState string, err error) {
	u, err := url.Parse(startURL)
	if err != nil {
		return "", "", fmt.Errorf("parse start url %q: %w", startURL, err)
	}
	token = u.Query().Get("token")
	if token == "" {
		return "", "", fmt.Errorf("start url %q has no token", startURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, startURL, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := s.inNetworkClient("").Do(req) // never follow: we only want the cookie
	if err != nil {
		return "", "", fmt.Errorf("captive-portal start: %w", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		return "", "", fmt.Errorf("captive-portal start: status %d", resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "wirety_cp_state" {
			cpState = c.Value
		}
	}
	if cpState == "" {
		return "", "", fmt.Errorf("captive-portal start: no wirety_cp_state cookie set")
	}
	return token, cpState, nil
}

// captivePortalAuthenticate POSTs /authenticate. Cookies are set by hand
// because the server marks wirety_cp_state Secure and the harness is plain HTTP
// (a cookie jar would refuse to send it back). An empty cpState or sessionHash
// omits that cookie, for negative tests.
func (s *stack) captivePortalAuthenticate(ctx context.Context, token, sessionHash, cpState string) error {
	body, _ := json.Marshal(map[string]string{"captive_token": token})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://server:8080/api/v1/captive-portal/authenticate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	var cookies []string
	if sessionHash != "" {
		cookies = append(cookies, "wirety_session="+sessionHash)
	}
	if cpState != "" {
		cookies = append(cookies, "wirety_cp_state="+cpState)
	}
	if len(cookies) > 0 {
		req.Header.Set("Cookie", strings.Join(cookies, "; "))
	}
	resp, err := s.inNetworkClient("").Do(req)
	if err != nil {
		return fmt.Errorf("captive-portal authenticate: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("captive-portal authenticate: status %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

func codeFromLocation(resp *http.Response) string {
	loc := resp.Header.Get("Location")
	if loc == "" {
		return ""
	}
	u, err := url.Parse(loc)
	if err != nil {
		return ""
	}
	return u.Query().Get("code")
}
