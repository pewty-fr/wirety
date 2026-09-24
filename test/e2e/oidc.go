//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
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

// wiretySession performs the browser-less OIDC authorization-code flow against
// Dex and exchanges the resulting code at the Wirety server's /auth/token
// endpoint, yielding an authenticated *http.Client (cookie jar holding the
// wirety_session cookie). That session is what the captive-portal /authenticate
// endpoint requires.
//
// The flow emulates a browser:
//  1. GET the server's authorize redirect target on Dex (authorizeURL).
//  2. Dex serves an HTML login form; POST the static credentials to it.
//  3. Dex redirects to the client redirect_uri with ?code=… — we capture the
//     code from the Location header instead of following it.
//  4. POST {code, redirect_uri} to the Wirety server's /auth/token; the server
//     exchanges the code with Dex server-to-server and sets the session cookie.
//
// NOTE: this is the most environment-sensitive helper in the harness (it parses
// Dex's login page). It is exercised only by the captive-portal subtest.
func wiretySession(ctx context.Context, serverBaseURL, dexAuthURL, dexIssuerHostPort, clientID, redirectURI, username, password string) (*http.Client, error) {
	jar, _ := cookiejar.New(nil)
	// Do not auto-follow the final redirect to redirect_uri (which is not a real
	// page) — we want to read the `code` from the Location header.
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if strings.HasPrefix(req.URL.String(), redirectURI) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	// 1. Kick off the authorization request directly against Dex.
	authQ := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"openid profile email"},
		"state":         {"e2e-state"},
		"nonce":         {"e2e-nonce"},
	}
	authReq, err := http.NewRequestWithContext(ctx, http.MethodGet, dexAuthURL+"?"+authQ.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(authReq)
	if err != nil {
		return nil, fmt.Errorf("dex authorize: %w", err)
	}
	loginHTML, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	loginActionURL := resp.Request.URL // Dex redirected to its /auth/local?req=… login page

	// 2. POST credentials to the login form's action. Dex's local login form
	// posts back to the same URL (the ?req=… carries the auth state).
	loginForm := url.Values{"login": {username}, "password": {password}}
	code, err := dexSubmitLoginAndCaptureCode(ctx, client, loginActionURL.String(), string(loginHTML), loginForm, redirectURI)
	if err != nil {
		return nil, err
	}

	// 4. Exchange the code for a Wirety session.
	tokenBody, _ := json.Marshal(map[string]string{"code": code, "redirect_uri": redirectURI})
	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, serverBaseURL+"/auth/token", strings.NewReader(string(tokenBody)))
	if err != nil {
		return nil, err
	}
	tokenReq.Header.Set("Content-Type", "application/json")
	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return nil, fmt.Errorf("wirety /auth/token: %w", err)
	}
	defer func() { _ = tokenResp.Body.Close() }()
	if tokenResp.StatusCode < 200 || tokenResp.StatusCode >= 300 {
		b, _ := io.ReadAll(tokenResp.Body)
		return nil, fmt.Errorf("wirety /auth/token: status %d: %s", tokenResp.StatusCode, string(b))
	}
	return client, nil
}

var dexApproveRe = regexp.MustCompile(`action="([^"]*/approval[^"]*)"`)

// dexSubmitLoginAndCaptureCode submits the Dex login form, follows the optional
// approval step, and returns the authorization `code` captured from the redirect
// to redirectURI.
func dexSubmitLoginAndCaptureCode(ctx context.Context, client *http.Client, loginURL, _loginHTML string, form url.Values, redirectURI string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("dex login submit: %w", err)
	}
	// If Dex redirected straight to the client with a code, capture it.
	if code := codeFromLocation(resp); code != "" {
		_ = resp.Body.Close()
		return code, nil
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// Otherwise an approval page may be shown; approve it.
	if m := dexApproveRe.FindStringSubmatch(string(bodyBytes)); m != nil {
		approveURL := resolveRef(resp.Request.URL, m[1])
		areq, err := http.NewRequestWithContext(ctx, http.MethodPost, approveURL, strings.NewReader(url.Values{"approval": {"approve"}}.Encode()))
		if err != nil {
			return "", err
		}
		areq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		aresp, err := client.Do(areq)
		if err != nil {
			return "", fmt.Errorf("dex approval: %w", err)
		}
		defer func() { _ = aresp.Body.Close() }()
		if code := codeFromLocation(aresp); code != "" {
			return code, nil
		}
		return "", fmt.Errorf("dex approval: no code in redirect to %s", redirectURI)
	}
	return "", fmt.Errorf("dex login: no code and no approval form (unexpected login response)")
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

func resolveRef(base *url.URL, ref string) string {
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(r).String()
}
