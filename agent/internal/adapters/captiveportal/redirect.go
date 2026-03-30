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

	"github.com/rs/zerolog/log"
)

// Server is a local HTTP server that intercepts unauthenticated peers' HTTP traffic
// and redirects them to the Wirety captive portal authentication page.
type Server struct {
	serverURL string // Wirety server HTTP URL, e.g. "https://wirety.example.com"
	authToken string // Jump peer's enrollment token for API calls
	portalURL string // Captive portal page URL, e.g. "https://wirety.example.com/captive-portal"
	networkID string
	peerID    string
}

// NewServer creates a captive portal redirect server.
func NewServer(serverURL, authToken, portalURL, networkID, peerID string) *Server {
	return &Server{
		serverURL: serverURL,
		authToken: authToken,
		portalURL: portalURL,
		networkID: networkID,
		peerID:    peerID,
	}
}

// Start begins listening on addr (e.g. ":8081"). Blocks until error.
func (s *Server) Start(addr string) error {
	log.Info().Str("addr", addr).Str("portal_url", s.portalURL).Msg("captive portal redirect server starting")
	return http.ListenAndServe(addr, s) // #nosec G114
}

// ServeHTTP handles intercepted HTTP requests. It creates a captive portal token
// for the peer and redirects the browser to the authentication page.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peerIP := host

	log.Info().Str("peer_ip", peerIP).Str("host", r.Host).Str("uri", r.RequestURI).Msg("captive portal: intercepted HTTP request")

	token, err := s.createToken(peerIP)
	if err != nil {
		log.Error().Err(err).Str("peer_ip", peerIP).Msg("captive portal: failed to create token")
		// Return a simple HTML page so at least the user sees something
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintf(w, "<html><body><h1>Network access requires authentication.</h1><p>Please retry in a few seconds.</p></body></html>")
		return
	}

	originalURL := fmt.Sprintf("http://%s%s", r.Host, r.RequestURI)
	redirectTarget := fmt.Sprintf("%s?token=%s&redirect=%s",
		s.portalURL,
		url.QueryEscape(token),
		url.QueryEscape(originalURL),
	)

	log.Info().Str("peer_ip", peerIP).Str("redirect_to", redirectTarget).Msg("captive portal: redirecting peer to auth")
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

	resp, err := http.DefaultClient.Do(req)
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
