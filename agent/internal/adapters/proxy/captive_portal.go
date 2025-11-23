package proxy

import (
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
	httpsPort        int
	portalURL        string
	serverURL        string
	agentToken       string
	server           *http.Server
	httpsServer      *http.Server
	mu               sync.RWMutex
	nonAgentPeers    map[string]bool // IP -> true for non-agent peers
	whitelistedPeers map[string]bool // IP -> true for authenticated peers
	captiveToken     string          // Current captive portal token
	tokenExpiry      time.Time       // When current token expires
}

// NewCaptivePortal creates a new captive portal proxy
func NewCaptivePortal(httpPort, httpsPort int, portalURL, serverURL, agentToken string) *CaptivePortal {
	return &CaptivePortal{
		httpPort:         httpPort,
		httpsPort:        httpsPort,
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

// Start starts the HTTP and HTTPS proxy servers
func (cp *CaptivePortal) Start() error {
	// Start HTTP proxy
	cp.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", cp.httpPort),
		Handler: http.HandlerFunc(cp.handleHTTP),
	}

	// Start HTTPS proxy (for CONNECT method)
	cp.httpsServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cp.httpsPort),
		Handler: http.HandlerFunc(cp.handleHTTPS),
	}

	// Start HTTP server
	go func() {
		log.Info().Int("port", cp.httpPort).Msg("starting captive portal HTTP proxy")
		if err := cp.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTP proxy server error")
		}
	}()

	// Start HTTPS server
	go func() {
		log.Info().Int("port", cp.httpsPort).Msg("starting captive portal HTTPS proxy")
		if err := cp.httpsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("HTTPS proxy server error")
		}
	}()

	return nil
}

// Stop stops the proxy servers
func (cp *CaptivePortal) Stop() error {
	if cp.server != nil {
		if err := cp.server.Close(); err != nil {
			return err
		}
	}
	if cp.httpsServer != nil {
		if err := cp.httpsServer.Close(); err != nil {
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

	// Redirect to front-end captive portal page
	cp.redirectToFrontend(w, r)
}

// handleHTTPS handles HTTPS CONNECT requests
func (cp *CaptivePortal) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	// Extract client IP (remove port)
	clientIP := r.RemoteAddr
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		clientIP = clientIP[:idx]
	}

	log.Debug().
		Str("client_ip", clientIP).
		Str("method", r.Method).
		Str("host", r.Host).
		Msg("HTTPS proxy request")

	// Check if this is a non-agent peer
	if !cp.isNonAgentPeer(clientIP) {
		// Not a non-agent peer, should not reach here
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Check if peer is whitelisted (authenticated)
	if cp.isWhitelisted(clientIP) {
		// Whitelisted peer, should not be redirected anymore
		http.Error(w, "Access granted - firewall rules updating", http.StatusOK)
		return
	}

	// For HTTPS, we can't easily redirect, so we return a 403 with a message
	// The client will see an error and may try HTTP
	w.WriteHeader(http.StatusForbidden)

	// Get captive token for the message
	captiveToken, err := cp.getCaptiveToken()
	if err != nil {
		fmt.Fprintf(w, "HTTPS access restricted. Please visit %s to authenticate.", cp.portalURL)
		return
	}

	portalURL := fmt.Sprintf("%s/captive-portal?token=%s", cp.portalURL, captiveToken)
	fmt.Fprintf(w, "HTTPS access restricted. Please visit %s to authenticate.", portalURL)
}

// redirectToFrontend redirects the request to the front-end captive portal page
func (cp *CaptivePortal) redirectToFrontend(w http.ResponseWriter, r *http.Request) {
	// Get a valid captive portal token
	captiveToken, err := cp.getCaptiveToken()
	if err != nil {
		log.Error().Err(err).Msg("failed to get captive portal token")
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	// Build the captive portal URL with captive portal token (NOT agent token)
	portalURL := fmt.Sprintf("%s/captive-portal?token=%s&redirect=%s",
		cp.portalURL,
		captiveToken,
		r.URL.String())

	log.Debug().
		Str("portal_url", portalURL).
		Str("original_url", r.URL.String()).
		Msg("redirecting to front-end captive portal")

	http.Redirect(w, r, portalURL, http.StatusFound)
}

// getCaptiveToken gets a valid captive portal token, fetching a new one if needed
func (cp *CaptivePortal) getCaptiveToken() (string, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Check if current token is still valid (with 1 minute buffer)
	if cp.captiveToken != "" && time.Now().Before(cp.tokenExpiry.Add(-1*time.Minute)) {
		return cp.captiveToken, nil
	}

	// Fetch new token from server
	url := fmt.Sprintf("%s/api/v1/agent/captive-portal-token?token=%s", cp.serverURL, cp.agentToken)
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

	// Store the new token
	cp.captiveToken = result.CaptivePortalToken
	cp.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)

	log.Info().
		Str("expires_at", cp.tokenExpiry.Format(time.RFC3339)).
		Msg("fetched new captive portal token")

	return cp.captiveToken, nil
}
