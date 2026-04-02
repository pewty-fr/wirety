// Package captiveportal provides a local HTTP redirect server for the captive portal flow.
// When a new peer connects to the WireGuard tunnel, their HTTP traffic (port 80) is
// redirected via iptables DNAT to this server. The server creates a short-lived captive
// portal token via the Wirety API and sends the peer to the authentication page.
package captiveportal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// tokenTTL is how long a cached token is reused before a fresh one is fetched.
// Slightly shorter than the server-side 10-minute token lifetime so we never
// present an already-expired token to the captive portal page.
const tokenTTL = 9 * time.Minute

type cachedToken struct {
	value     string
	expiresAt time.Time
}

// tokenCache is a simple in-memory per-peer-IP token cache. Without it, every
// HTTP request intercepted by the DNAT rule (browser fetches, keepalives, etc.)
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

// Server is a local HTTP server that intercepts unauthenticated peers' HTTP traffic
// and redirects them to the Wirety captive portal authentication page.
type Server struct {
	serverURL  string // Wirety server HTTP URL, e.g. "https://wirety.example.com"
	authToken  string // Jump peer's enrollment token for API calls
	portalURL  string // Captive portal page URL, e.g. "https://wirety.example.com/captive-portal"
	networkID  string
	peerID     string
	httpClient *http.Client // shared client (may override Host header for reverse-proxy setups)
	cache      tokenCache
}

// NewServer creates a captive portal redirect server.
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

// Start begins listening on addr (e.g. ":8081"). Blocks until error.
func (s *Server) Start(addr string) error {
	log.Info().Str("addr", addr).Str("portal_url", s.portalURL).Msg("captive portal redirect server starting")
	return http.ListenAndServe(addr, s) // #nosec G114
}

// ServeHTTP handles intercepted HTTP requests. It returns the cached token for
// the peer if one is still valid, otherwise creates a new one, then redirects
// the browser to the captive portal authentication page.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peerIP := host

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

	http.Redirect(w, r, redirectTarget, http.StatusFound)
}

type createTokenRequest struct {
	PeerIP string `json:"peer_ip"`
}

func (s *Server) createToken(peerIP string) (string, error) {
	body, _ := json.Marshal(createTokenRequest{PeerIP: peerIP})

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
