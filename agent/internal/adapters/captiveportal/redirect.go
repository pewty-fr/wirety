// Package captiveportal provides the HTTP server for the captive portal flow.
// The server listens directly on the WireGuard interface IP on port 80, replacing
// the previous DNAT-to-localhost approach. This allows it to:
//   - Intercept unauthenticated peers' HTTP requests and redirect them to the
//     Wirety captive portal authentication page.
//   - Intercept well-known OS captive-portal probe requests (even on VPNs with
//     restricted AllowedIPs) and return the expected success responses once a peer
//     has authenticated, so the OS dismisses its captive portal notification.
package captiveportal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// tokenTTL is how long a cached token is reused before a fresh one is fetched.
// Slightly shorter than the server-side 10-minute token lifetime so we never
// present an already-expired token to the captive portal page.
const tokenTTL = 9 * time.Minute

// captiveProbeHosts is the set of well-known OS captive-portal probe hostnames.
// The agent's DNS server resolves these to the jump peer's WireGuard IP so that
// the probes are routed through the tunnel even when AllowedIPs is restricted to
// a private range. The HTTP server then intercepts them here.
var captiveProbeHosts = map[string]struct{}{
	"connectivitycheck.gstatic.com": {},
	"clients3.google.com":           {},
	"clients1.google.com":           {},
	"captive.apple.com":             {},
	"www.apple.com":                 {},
	"www.msftconnecttest.com":       {},
	"ipv6.msftconnecttest.com":      {},
	"detectportal.firefox.com":      {},
	"nmcheck.gnome.org":             {},
	"network-test.debian.org":       {},
}

type cachedToken struct {
	value     string
	expiresAt time.Time
}

// tokenCache is a simple in-memory per-peer-IP token cache. Without it, every
// HTTP request from an unauthenticated peer (browser fetches, keepalives, etc.)
// would create a new captive portal token, flooding the database.
type tokenCache struct {
	mu      sync.Mutex
	entries map[string]cachedToken
}

func (c *tokenCache) get(peerIP string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[peerIP]
	if !ok || time.Now().After(e.expiresAt) {
		return "", false
	}
	return e.value, true
}

func (c *tokenCache) set(peerIP, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[peerIP] = cachedToken{value: token, expiresAt: time.Now().Add(tokenTTL)}
}

// retain drops cache entries for peer IPs not present in keep.  The cache
// invariant we want to enforce is "a cached token only ever maps to a peer
// whose server-side row is still pending_auth"; anything outside that set is
// stale (token was consumed by authentication, by an admin "Revoke Auth", or
// by a re-issue) and must not be handed back to the next browser request.
func (c *tokenCache) retain(keep map[string]struct{}) (dropped []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for ip := range c.entries {
		if _, ok := keep[ip]; !ok {
			delete(c.entries, ip)
			dropped = append(dropped, ip)
		}
	}
	return dropped
}

// Server is the captive portal HTTP server. It listens directly on the WireGuard
// interface IP on port 80 (e.g. "10.255.0.1:80"), replacing the previous approach
// of DNATing port-80 traffic to a localhost port.
type Server struct {
	serverURL       string
	authToken       string
	portalURL       string
	networkID       string
	peerID          string
	httpClient      *http.Client
	cache           tokenCache
	isAuthenticated func(peerIP string) bool // nil = treat all peers as unauthenticated
	// policyReceived gates probe-success responses: we never serve them before
	// receiving at least one policy message from the server, because the whitelist
	// could be stale from a previous connection (e.g. DB cleared without a push).
	policyReceived bool
	policyMu       sync.RWMutex
	// lookupEndpoint resolves a peer's WireGuard private IP to its current
	// public endpoint ("ip:port", strict).  When set, the token request sent
	// to the server includes the peer's full public endpoint so the server
	// can store it in the whitelist entry.  The jump peer later verifies that
	// the peer's current endpoint still matches the one recorded at
	// authentication time — any change (different IP, NAT port rebinding,
	// tunnel restart) triggers re-authentication.
	lookupEndpoint func(wgIP string) string
}

// NewServer creates a captive portal HTTP server.
// httpClient may be nil, in which case http.DefaultClient is used.
func NewServer(serverURL, authToken, portalURL, networkID, peerID string, httpClient *http.Client) *Server {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Server{
		serverURL:  serverURL,
		authToken:  authToken,
		portalURL:  portalURL,
		networkID:  networkID,
		peerID:     peerID,
		httpClient: httpClient,
		cache:      tokenCache{entries: make(map[string]cachedToken)},
	}
}

// SetAuthChecker sets a function that reports whether a peer IP has completed
// captive portal authentication. Authenticated peers receive OS-specific probe
// success responses so the OS dismisses the captive portal notification after login.
func (s *Server) SetAuthChecker(fn func(peerIP string) bool) {
	s.isAuthenticated = fn
}

// SetEndpointLookup sets a function that returns the current public endpoint
// ("ip:port", strict) for a given WireGuard private IP.  When set, the captive
// portal token request includes the peer's full public endpoint so the server
// can bind the whitelist entry to a specific source IP+port.
func (s *Server) SetEndpointLookup(fn func(wgIP string) string) {
	s.lookupEndpoint = fn
}

// RetainPendingTokens drops cached redirect tokens for peers that are no
// longer in the server-reported pending_auth set.  Called by the agent's WS
// payload handler on every policy push so that:
//
//   - After an admin "Revoke Auth", the next browser request from the affected
//     peer creates a fresh token instead of being redirected to a now-consumed
//     one (which would produce the "persist state: token not found" error at
//     /captive-portal/start).
//   - After a successful authentication, the cached token (now in the whitelist
//     tier) is also dropped — we won't need it again until the peer is logged
//     out, and keeping it around would only widen the window for a stale hit.
func (s *Server) RetainPendingTokens(pendingPeerIPs []string) {
	keep := make(map[string]struct{}, len(pendingPeerIPs))
	for _, ip := range pendingPeerIPs {
		keep[ip] = struct{}{}
	}
	if dropped := s.cache.retain(keep); len(dropped) > 0 {
		log.Debug().Strs("peer_ips", dropped).Msg("captive portal: dropped cached tokens after policy push")
	}
}

// NotifyPolicyReceived marks the server as having received at least one policy
// message from the Wirety server. Until this is called, all peers are treated as
// unauthenticated, even if the in-memory whitelist was non-empty (e.g. from a
// previous connection where the DB was cleared without a WebSocket push).
func (s *Server) NotifyPolicyReceived() {
	s.policyMu.Lock()
	s.policyReceived = true
	s.policyMu.Unlock()
}

// ResetPolicyReceived clears the policy-received flag. Called on every new
// WebSocket connection so that the server waits for a fresh policy sync before
// serving probe-success responses, preventing stale whitelist data from a
// previous connection from leaking through.
func (s *Server) ResetPolicyReceived() {
	s.policyMu.Lock()
	s.policyReceived = false
	s.policyMu.Unlock()
}

// isPolicyReceived reports whether at least one policy sync has been received on
// the current connection.
func (s *Server) isPolicyReceived() bool {
	s.policyMu.RLock()
	defer s.policyMu.RUnlock()
	return s.policyReceived
}

// Start begins listening on addr (e.g. "10.255.0.1:80"). Blocks until error.
func (s *Server) Start(addr string) error {
	log.Info().Str("addr", addr).Str("portal_url", s.portalURL).Msg("captive portal HTTP server starting")
	return http.ListenAndServe(addr, s) // #nosec G114
}

// NOTE: the captive portal used to also run an HTTPS (:443) listener with a
// self-signed cert (StartTLS + generateSelfSignedCert). That was a TLS MITM —
// it served the portal's cert under whatever SNI the browser requested — and it
// produced unrecoverable HSTS errors for preloaded hosts and http:// returns for
// HTTPS-only apps. It has been removed: the portal is HTTP-only. Unauthenticated
// HTTPS to an internal resource now fails fast with a TCP reset instead of being
// intercepted, and portal discovery is carried by the OS captive-portal probes
// (HTTP) plus the dashboard sign-in popup. See runner.go for the rationale.

// ServeHTTP handles all HTTP requests arriving on the WireGuard interface port 80.
//
// Authenticated peers: return OS-specific probe success responses for known
// captive-portal probe URLs so the OS dismisses the portal notification.
// All other requests from authenticated peers return 204.
//
// Unauthenticated peers: create a short-lived captive portal token and redirect
// the browser to the Wirety authentication page.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	peerIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerIP = r.RemoteAddr
	}

	// Only serve probe-success responses after the first policy sync.
	// Before that, the whitelist could be stale from a previous connection
	// (e.g. the DB was cleared without a WebSocket push between the old and new
	// connection), so we fall through and treat the peer as unauthenticated.
	if s.isPolicyReceived() && s.isAuthenticated != nil && s.isAuthenticated(peerIP) {
		// A known OS captive-portal probe → return the success response so the OS
		// dismisses its "Sign in to network" banner. This still matters even
		// though internal domains no longer resolve to the portal IP: the OS may
		// have cached a probe host → portal IP from before the peer authenticated.
		if serveProbeSuccess(w, r) {
			log.Debug().Str("peer_ip", peerIP).Str("host", r.Host).Str("path", r.URL.Path).
				Msg("captive portal: authenticated peer probe — returning success")
			return
		}
		// Any other request from an already-authenticated peer that reached the
		// portal directly — the dashboard "Sign in" popup or a bookmarked portal
		// URL. There is nothing to proxy: internal domains now resolve to their
		// real IP and an authenticated peer's traffic is forwarded straight to the
		// backend (never DNAT'd here). So we just show a terminal "you're
		// connected" page. We deliberately do NOT meta-refresh/redirect — doing so
		// looped forever when the target was the jump peer's own IP.
		serveConnectedPage(w, r)
		return
	}

	token, ok := s.cache.get(peerIP)
	if !ok {
		log.Info().Str("peer_ip", peerIP).Str("host", r.Host).Msg("captive portal: intercepted HTTP request, creating token")
		token, err = s.createToken(peerIP)
		if err != nil {
			log.Error().Err(err).Str("peer_ip", peerIP).Msg("captive portal: failed to create token")
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, "<html><body><h1>Network access requires authentication.</h1><p>Please retry in a few seconds.</p></body></html>")
			return
		}
		s.cache.set(peerIP, token)
	} else {
		log.Debug().Str("peer_ip", peerIP).Msg("captive portal: reusing cached token")
	}

	originalURL := fmt.Sprintf("http://%s%s", r.Host, r.RequestURI)

	// Redirect through the central server's /start bouncer instead of going
	// straight to the captive-portal page.  The bouncer sets a same-origin
	// cookie binding the token to THIS browser session — the auth endpoint
	// later requires the cookie to match, which prevents the phishing attack
	// where an attacker on a stolen WG config generates a token URL and
	// sends it to the legitimate owner.  See the server-side migration 026
	// and api/captive_portal_handlers.go::CaptivePortalStart for details.
	//
	// We build the URL from --portal-url's scheme+host rather than --server, so
	// the browser is sent to the user-facing hostname (and its load balancer)
	// rather than the raw IP the agent uses internally for API traffic.  The
	// /start endpoint and the /captive-portal page MUST be on the same origin
	// for the bouncer's same-origin cookie to be readable by the page.
	//
	// Falls back to serverURL when portalURL is missing a scheme+host (shouldn't
	// happen in practice — main.go always defaults portalURL to <server>/captive-portal).
	startURL := strings.TrimRight(s.serverURL, "/") + "/api/v1/captive-portal/start"
	if parsed, err := url.Parse(s.portalURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		startURL = parsed.Scheme + "://" + parsed.Host + "/api/v1/captive-portal/start"
	}
	redirectTarget := fmt.Sprintf("%s?token=%s&redirect=%s",
		startURL,
		url.QueryEscape(token),
		url.QueryEscape(originalURL),
	)
	// Prevent the browser from caching this redirect. Without no-store the
	// browser would replay the 302 to the captive portal even after the peer
	// has authenticated and DNS has returned to the real service IP.
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, redirectTarget, http.StatusFound)
}

// serveConnectedPage renders a terminal page for an already-authenticated peer
// that reached the captive portal directly (the dashboard "Sign in" popup, or a
// bookmarked portal URL). There is nothing to do — internal apps resolve to
// their real IP and are forwarded straight to the backend — so we just tell the
// user they're connected. We do NOT meta-refresh or redirect: when the target
// was the jump peer's own IP that looped forever.
func serveConnectedPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Connected</title></head>`+
		`<body style="font-family:sans-serif;text-align:center;padding-top:4em">`+
		`<h1>✓ You are connected</h1>`+
		`<p>You can close this tab and access the network.</p>`+
		`</body></html>`)
}

// serveProbeSuccess writes the OS-specific "connected" response for known
// captive-portal probe URLs. Returns true if the request was a recognised probe
// and a response was written, false otherwise.
//
// Each OS expects a specific response to confirm internet connectivity:
//   - Google/Android: GET /generate_204 → 204 No Content
//   - Apple:          GET /hotspot-detect.html → 200 with "Success" body
//   - Windows:        GET /connecttest.txt → 200 with "Microsoft Connect Test"
//   - Firefox:        GET /success.txt → 200 with "success\n"
//   - GNOME/Debian:   any probe → 204
func serveProbeSuccess(w http.ResponseWriter, r *http.Request) bool {
	host := strings.ToLower(r.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	path := r.URL.Path

	switch {
	case strings.Contains(host, "apple.com") &&
		(path == "/hotspot-detect.html" || path == "/library/test/success.html"):
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, "<HTML><HEAD><TITLE>Success</TITLE></HEAD><BODY>Success</BODY></HTML>")
		return true

	case strings.Contains(host, "msftconnecttest.com") && path == "/connecttest.txt":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "Microsoft Connect Test")
		return true

	case strings.Contains(host, "msftconnecttest.com") && path == "/redirect":
		http.Redirect(w, r, "http://go.microsoft.com/fwlink/?LinkID=219472&clcid=0x409", http.StatusFound)
		return true

	case path == "/generate_204":
		w.WriteHeader(http.StatusNoContent)
		return true

	case strings.Contains(host, "detectportal.firefox.com") && path == "/success.txt":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, "success\n")
		return true

	case strings.Contains(host, "nmcheck.gnome.org") ||
		strings.Contains(host, "network-test.debian.org"):
		w.WriteHeader(http.StatusNoContent)
		return true
	}

	// Catch-all for any request to a known probe host.
	if _, ok := captiveProbeHosts[host]; ok {
		w.WriteHeader(http.StatusNoContent)
		return true
	}

	return false
}

type createTokenRequest struct {
	PeerIP       string `json:"peer_ip"`
	PeerEndpoint string `json:"peer_endpoint,omitempty"` // full "ip:port" at connect time
}

func (s *Server) createToken(peerIP string) (string, error) {
	var endpoint string
	if s.lookupEndpoint != nil {
		endpoint = s.lookupEndpoint(peerIP)
	}
	body, _ := json.Marshal(createTokenRequest{PeerIP: peerIP, PeerEndpoint: endpoint})
	req, err := http.NewRequest(http.MethodPost, s.serverURL+"/api/v1/captive-portal/token", bytes.NewReader(body)) // #nosec G107
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.authToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("API returned %d", resp.StatusCode)
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return tokenResp.Token, nil
}
