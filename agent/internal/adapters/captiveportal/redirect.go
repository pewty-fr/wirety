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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
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
	// lookupPeerIP resolves an internal VPN hostname to its real WireGuard IP.
	// When set, authenticated peers are proxied directly to the real backend
	// instead of being served the meta-refresh "Connecting…" page, which avoids
	// the infinite-loop caused by browsers (especially Firefox) ignoring TTL=1
	// and caching DNS for up to 60 seconds.
	lookupPeerIP func(host string) string
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

// SetPeerIPLookup sets a function that resolves an internal VPN hostname to its
// real WireGuard IP. When set, authenticated peers whose browser DNS cache still
// points to the jump peer are transparently proxied to the real backend instead
// of being shown the "Connecting…" meta-refresh page.
func (s *Server) SetPeerIPLookup(fn func(host string) string) {
	s.lookupPeerIP = fn
}

// SetEndpointLookup sets a function that returns the current public endpoint
// ("ip:port", strict) for a given WireGuard private IP.  When set, the captive
// portal token request includes the peer's full public endpoint so the server
// can bind the whitelist entry to a specific source IP+port.
func (s *Server) SetEndpointLookup(fn func(wgIP string) string) {
	s.lookupEndpoint = fn
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

// StartTLS begins listening on addr with a self-signed certificate covering the
// given IP and optional VPN domain wildcard. It uses the same ServeHTTP handler
// as the plain HTTP server.
//
// Unlike external domains (google.com, etc.) which are HSTS-preloaded and hard-
// blocked by browsers, internal VPN domains are not preloaded — browsers show a
// "certificate not trusted" warning that the user can bypass. This is sufficient
// to redirect unauthenticated peers that attempt HTTPS access to a private resource.
func (s *Server) StartTLS(addr, ip, vpnDomain string) error {
	cert, err := generateSelfSignedCert(ip, vpnDomain)
	if err != nil {
		return fmt.Errorf("generate self-signed cert: %w", err)
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	ln, err := tls.Listen("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{Handler: s, TLSConfig: tlsCfg}
	log.Info().Str("addr", addr).Str("portal_url", s.portalURL).Msg("captive portal HTTPS server starting (self-signed)")
	return srv.Serve(ln)
}

// generateSelfSignedCert creates an ECDSA P-256 certificate valid for 10 years.
// The cert covers the WG IP as a SAN IP, and — if vpnDomain is non-empty — a
// wildcard DNS SAN (*.<vpnDomain>) so that internal peer hostnames match without
// a domain-mismatch warning (the cert is still self-signed and untrusted, but the
// browser's "proceed anyway" path becomes available for internal VPN domains that
// are not in the HSTS preload list).
func generateSelfSignedCert(ip, vpnDomain string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Wirety Captive Portal"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		tmpl.IPAddresses = []net.IP{parsed}
	}
	if vpnDomain != "" {
		tmpl.DNSNames = []string{vpnDomain, "*." + vpnDomain}
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create cert: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return tls.X509KeyPair(certPEM, keyPEM)
}


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
		if serveProbeSuccess(w, r) {
			log.Debug().Str("peer_ip", peerIP).Str("host", r.Host).Str("path", r.URL.Path).
				Msg("captive portal: authenticated peer probe — returning success")
			return
		}
		// The peer is authenticated but the browser's DNS cache may still point to
		// this jump peer (browsers like Firefox ignore TTL=1 and cache for up to 60 s).
		//
		// Strategy: if we know the real backend IP, proxy the request there directly
		// so the user gets their content immediately.  The browser will eventually
		// resolve DNS to the real IP and bypass us entirely.
		//
		// Fallback: if no lookup is configured or the hostname is unknown, serve the
		// old meta-refresh page and hope the DNS cache expires before the next hit.
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if s.lookupPeerIP != nil {
			if realIP := s.lookupPeerIP(host); realIP != "" {
				log.Debug().Str("peer_ip", peerIP).Str("host", r.Host).Str("real_ip", realIP).
					Msg("captive portal: authenticated peer — proxying to real backend")
				s.proxyToBackend(w, r, realIP)
				return
			}
		}
		// Fallback: serve meta-refresh (DNS lookup unavailable or host not found).
		log.Debug().Str("peer_ip", peerIP).Str("host", r.Host).
			Msg("captive portal: authenticated peer — serving DNS-flush refresh page (no real IP known)")
		serveConnectingPage(w, r)
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
	redirectTarget := fmt.Sprintf("%s?token=%s&redirect=%s",
		s.portalURL,
		url.QueryEscape(token),
		url.QueryEscape(originalURL),
	)
	// Prevent the browser from caching this redirect. Without no-store the
	// browser would replay the 302 to the captive portal even after the peer
	// has authenticated and DNS has returned to the real service IP.
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, redirectTarget, http.StatusFound)
}

// hop-by-hop headers that must not be forwarded through a proxy.
var hopByHopHeaders = map[string]struct{}{
	"connection": {}, "keep-alive": {}, "proxy-authenticate": {},
	"proxy-authorization": {}, "te": {}, "trailers": {},
	"transfer-encoding": {}, "upgrade": {},
}

// proxyToBackend forwards the request transparently to realIP, following any
// same-host redirects (e.g. HTTP→HTTPS) internally so the browser never sees
// an intermediate redirect that would loop back through the captive portal.
//
// Problem without this: the real backend often redirects HTTP→HTTPS.
// httputil.ReverseProxy forwards that 301 to the browser. The browser follows
// it back to the jump peer (DNS still cached), gets another 301, detects a loop.
//
// Solution: use a custom http.Client whose CheckRedirect rewrites same-host
// redirect URLs to use realIP directly.  The client follows the full redirect
// chain and returns the final content to the browser in one shot.
func (s *Server) proxyToBackend(w http.ResponseWriter, r *http.Request, realIP string) {
	originalHost := r.Host
	originalHostname := originalHost
	if h, _, err := net.SplitHostPort(originalHost); err == nil {
		originalHostname = h
	}

	// Allow HTTPS connections to the real backend even when its TLS cert is
	// issued for the VPN hostname rather than the raw WireGuard IP.
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 — internal VPN
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			// Rewrite same-host redirects (e.g. http://vpn-host/ → https://vpn-host/)
			// to always target realIP so they never loop back through the captive portal.
			if req.URL.Hostname() == originalHostname {
				port := req.URL.Port()
				if port != "" {
					req.URL.Host = net.JoinHostPort(realIP, port)
				} else {
					req.URL.Host = realIP
				}
				req.Host = originalHost // preserve vhost routing on the real backend
			}
			return nil
		},
	}

	// Send the initial request to the real backend over HTTP (or HTTPS when the
	// incoming request was already HTTPS).
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	targetURL := fmt.Sprintf("%s://%s%s", scheme, realIP, r.RequestURI)

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		log.Debug().Err(err).Msg("captive portal: could not build proxy request, falling back to meta-refresh")
		serveConnectingPage(w, r)
		return
	}
	proxyReq.Host = originalHost
	for k, vv := range r.Header {
		if _, skip := hopByHopHeaders[strings.ToLower(k)]; !skip {
			proxyReq.Header[k] = vv
		}
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Debug().Err(err).Str("real_ip", realIP).
			Msg("captive portal: proxy to real backend failed, falling back to meta-refresh")
		serveConnectingPage(w, r)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Forward the final response (after all redirects have been followed).
	for k, vv := range resp.Header {
		if _, skip := hopByHopHeaders[strings.ToLower(k)]; !skip {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// serveConnectingPage renders a meta-refresh fallback for authenticated peers
// when the real backend IP is not known or the proxy attempt failed.
func serveConnectingPage(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	target := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><head>`+
		`<meta http-equiv="refresh" content="2; url=%s">`+
		`<title>Connecting…</title></head>`+
		`<body style="font-family:sans-serif;text-align:center;padding-top:4em">`+
		`<p>Authenticated. Connecting to your destination…</p>`+
		`<p><a href="%s">Click here if not redirected automatically.</a></p>`+
		`</body></html>`, target, target)
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
