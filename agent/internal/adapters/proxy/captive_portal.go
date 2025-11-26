package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CaptivePortal implements an HTTP proxy that redirects non-agent peers to a captive portal
type CaptivePortal struct {
	httpPort         int
	portalURL        string
	serverURL        string
	agentToken       string
	server           *http.Server
	mu               sync.RWMutex
	nonAgentPeers    map[string]bool // IP -> true for non-agent peers
	whitelistedPeers map[string]bool // IP -> true for authenticated peers
}

// NewCaptivePortal creates a new captive portal proxy
// Note: httpsPort parameter is kept for backward compatibility but not used
// HTTPS traffic is now handled by the TLS-SNI gateway on port 443
func NewCaptivePortal(httpPort int, portalURL, serverURL, agentToken string) *CaptivePortal {
	return &CaptivePortal{
		httpPort:         httpPort,
		portalURL:        portalURL,
		serverURL:        serverURL,
		agentToken:       agentToken,
		nonAgentPeers:    make(map[string]bool),
		whitelistedPeers: make(map[string]bool),
	}
}

// UpdateNonAgentPeers updates the list of non-agent peer IPs
func (cp *CaptivePortal) UpdateNonAgentPeers(peerIPs []string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Clear and rebuild the map
	cp.nonAgentPeers = make(map[string]bool)
	for _, ip := range peerIPs {
		cp.nonAgentPeers[ip] = true
	}

	log.Info().Int("count", len(peerIPs)).Msg("updated non-agent peers for captive portal")
}

// isNonAgentPeer checks if an IP is a non-agent peer
func (cp *CaptivePortal) isNonAgentPeer(ip string) bool {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.nonAgentPeers[ip]
}

// isWhitelisted checks if an IP is whitelisted (authenticated)
func (cp *CaptivePortal) isWhitelisted(ip string) bool {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.whitelistedPeers[ip]
}

// AddWhitelistedPeer adds a peer IP to the whitelist
func (cp *CaptivePortal) AddWhitelistedPeer(ip string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.whitelistedPeers[ip] = true
	log.Info().Str("ip", ip).Msg("peer whitelisted for internet access")
}

// RemoveWhitelistedPeer removes a peer IP from the whitelist
func (cp *CaptivePortal) RemoveWhitelistedPeer(ip string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	delete(cp.whitelistedPeers, ip)
	log.Info().Str("ip", ip).Msg("peer removed from whitelist")
}

// ClearWhitelist clears all whitelisted peers
func (cp *CaptivePortal) ClearWhitelist() {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.whitelistedPeers = make(map[string]bool)
	log.Info().Msg("whitelist cleared")
}

// Start starts the HTTP proxy server
// Note: HTTPS traffic is now handled by the TLS-SNI gateway on port 443
func (cp *CaptivePortal) Start() error {
	// Start HTTP proxy
	cp.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", cp.httpPort),
		Handler: http.HandlerFunc(cp.handleHTTP),
	}

	// Start HTTP server
	go func() {
		log.Info().Int("port", cp.httpPort).Msg("starting captive portal HTTP proxy")
		if err := cp.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP proxy server error")
		}
	}()

	return nil
}

// Stop stops the HTTP proxy server gracefully
func (cp *CaptivePortal) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if cp.server != nil {
		if err := cp.server.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("error shutting down HTTP server")
			return err
		}
	}

	return nil
}

// handleHTTP handles HTTP requests
func (cp *CaptivePortal) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract client IP (remove port)
	clientIP := r.RemoteAddr
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		clientIP = clientIP[:idx]
	}

	log.Debug().
		Str("client_ip", clientIP).
		Str("method", r.Method).
		Str("host", r.Host).
		Str("url", r.URL.String()).
		Msg("HTTP proxy request")

	// Check if this is a non-agent peer
	if !cp.isNonAgentPeer(clientIP) {
		// Not a non-agent peer, should not reach here
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Check if peer is whitelisted (authenticated)
	if cp.isWhitelisted(clientIP) {
		// Whitelisted peer, should not be redirected anymore
		// This shouldn't happen if firewall rules are updated correctly
		http.Error(w, "Access granted - firewall rules updating", http.StatusOK)
		return
	}

	// Check if this is a captive portal detection request
	// These are special URLs used by operating systems to detect captive portals
	isCaptiveDetection := cp.isCaptivePortalDetection(r.Host, r.URL.Path)

	if isCaptiveDetection {
		log.Debug().
			Str("client_ip", clientIP).
			Str("host", r.Host).
			Msg("captive portal detection request - redirecting")
	}

	// Redirect to front-end captive portal page
	cp.redirectToFrontend(w, r, clientIP)
}

// isCaptivePortalDetection checks if the request is a captive portal detection probe
func (cp *CaptivePortal) isCaptivePortalDetection(host, path string) bool {
	// Common captive portal detection endpoints
	captiveHosts := map[string]bool{
		"captive.apple.com":             true,
		"www.apple.com":                 true,
		"connectivitycheck.gstatic.com": true,
		"www.gstatic.com":               true,
		"clients3.google.com":           true,
		"www.msftconnecttest.com":       true,
		"www.msftncsi.com":              true,
	}

	// Remove port from host if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	return captiveHosts[strings.ToLower(host)]
}

// redirectToFrontend redirects the request to the front-end captive portal page
func (cp *CaptivePortal) redirectToFrontend(w http.ResponseWriter, r *http.Request, clientIP string) {
	// Get a valid captive portal token for this specific peer IP
	captiveToken, err := cp.getCaptiveTokenForPeer(clientIP)
	if err != nil {
		log.Error().Err(err).Msg("failed to get captive portal token")
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	// Always redirect to HTTPS server URL (portalURL should be https://SERVER_URL)
	// This ensures certificate validation works correctly
	// The peer IP is now embedded in the token, not in the URL
	portalURL := fmt.Sprintf("%s/captive-portal?token=%s",
		cp.portalURL,
		captiveToken,
	)

	log.Debug().
		Str("portal_url", portalURL).
		Str("client_ip", clientIP).
		Str("original_url", r.URL.String()).
		Str("original_host", r.Host).
		Msg("redirecting HTTP to HTTPS captive portal")

	// Use 302 redirect to send user to the HTTPS captive portal
	http.Redirect(w, r, portalURL, http.StatusFound)
}

// getCaptiveTokenForPeer gets a captive portal token for a specific peer IP
// Each token is unique and tied to the peer IP for security
func (cp *CaptivePortal) getCaptiveTokenForPeer(peerIP string) (string, error) {
	// Fetch new token from server with peer IP
	url := fmt.Sprintf("%s/api/v1/agent/captive-portal-token?token=%s&peer_ip=%s",
		cp.serverURL,
		cp.agentToken,
		peerIP,
	)
	resp, err := http.Get(url) // #nosec G107 - serverURL is trusted
	if err != nil {
		return "", fmt.Errorf("failed to fetch captive portal token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch captive portal token: status %d", resp.StatusCode)
	}

	var result struct {
		CaptivePortalToken string `json:"captive_portal_token"`
		ExpiresIn          int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	log.Info().
		Str("peer_ip", peerIP).
		Int("expires_in", result.ExpiresIn).
		Msg("fetched new captive portal token for peer")

	return result.CaptivePortalToken, nil
}
