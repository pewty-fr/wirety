package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	dnsadapter "wirety/agent/internal/adapters/dns"
	"wirety/agent/internal/adapters/firewall"
	"wirety/agent/internal/adapters/wg"
	"wirety/agent/internal/adapters/ws"
	app "wirety/agent/internal/application/agent"
	"wirety/agent/internal/audit"
	dom "wirety/agent/internal/domain/dns"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	auditEnabled := envOr("AUDIT_LOG", "false") == "true"
	audit.Init(auditEnabled)

	server := envOr("SERVER_URL", "http://localhost:8080")
	token := envOr("TOKEN", "")
	configPath := envOr("WG_CONFIG_PATH", "")
	applyMethod := envOr("WG_APPLY_METHOD", "syncconf")
	natIfacesStr := envOr("NAT_INTERFACES", "") // comma-separated; empty = auto-detect all
	httpPort := envOr("HTTP_PROXY_PORT", "3128")
	httpsPort := envOr("HTTPS_PROXY_PORT", "3129")
	portalURL := envOr("CAPTIVE_PORTAL_URL", "")
	serverHost := envOr("SERVER_HOST", "")                  // optional Host header override for reverse-proxy setups
	skipTLSVerify := envOr("SKIP_TLS_VERIFY", "") == "true" // skip TLS certificate verification

	flag.StringVar(&server, "server", server, "Server base URL (no trailing /)")
	flag.StringVar(&token, "token", token, "Enrollment token")
	flag.StringVar(&configPath, "config", configPath, "Path to wireguard config file")
	flag.StringVar(&applyMethod, "apply", applyMethod, "Apply method: wg-quick|syncconf")
	flag.StringVar(&natIfacesStr, "nat-interfaces", natIfacesStr, "Comma-separated NAT interfaces (empty = auto-detect all egress interfaces)")
	flag.StringVar(&portalURL, "portal-url", portalURL, "Captive portal page URL (default: <server>/captive-portal)")
	flag.StringVar(&serverHost, "server-host", serverHost, "Override HTTP Host header for all requests to the server (useful when accessing via IP behind a reverse proxy)")
	flag.BoolVar(&skipTLSVerify, "skip-tls-verify", skipTLSVerify, "Skip TLS certificate verification (insecure — use only with self-signed certificates in trusted environments)")
	flag.Parse()

	// Default portal URL: captive portal page served by the same Wirety server
	if portalURL == "" {
		portalURL = server + "/captive-portal"
	}

	// Parse comma-separated NAT interfaces; nil slice means auto-detect
	var natIfaces []string
	if natIfacesStr != "" {
		for _, s := range strings.Split(natIfacesStr, ",") {
			if s = strings.TrimSpace(s); s != "" {
				natIfaces = append(natIfaces, s)
			}
		}
	}

	if token == "" {
		log.Fatal().Msg("TOKEN is required (env or flag)")
	}

	if skipTLSVerify {
		log.Warn().Msg("TLS certificate verification is DISABLED (SKIP_TLS_VERIFY=true) — use only in trusted environments")
	}

	// Build a shared HTTP client that injects the Host header on every request
	// when SERVER_HOST is set (reverse-proxy / no-DNS setups).
	httpClient := newHTTPClient(serverHost, skipTLSVerify)

	// Resolve token first: we need the WireGuard config to know our VPN IP,
	// which is the address the DNS server must bind to.
	networkID, peerID, peerName, cfg, err := resolveToken(server, token, httpClient)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to resolve token")
	}
	log.Info().Str("network_id", networkID).Str("peer_id", peerID).Str("peer_name", peerName).Msg("resolved token")

	// Bind the DNS server to the WireGuard interface IP so it is reachable by VPN
	// peers through the tunnel, without conflicting with systemd-resolved (127.0.0.53:53).
	wgIP, err := parseWireGuardAddress(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse WireGuard address from config")
	}
	dnsListenAddr := wgIP + ":53"
	log.Info().Str("addr", dnsListenAddr).Msg("starting DNS server")
	dnsServer := dnsadapter.NewServer("", []dom.DNSPeer{})
	go func() {
		if err := dnsServer.Start(dnsListenAddr); err != nil {
			log.Error().Err(err).Msg("dns server exited")
		}
	}()

	// Use peer name as interface name - sanitize for valid interface names
	iface := sanitizeInterfaceName(peerName)
	writer := wg.NewWriter(configPath, iface, applyMethod)

	// Clean up any old Wirety-managed configs that don't match current peer
	log.Info().Msg("cleaning up old Wirety configurations")
	if err := writer.CleanupOldConfigs(); err != nil {
		log.Fatal().Err(err).Msg("failed to cleanup old configs")
	}

	// Verify ownership of the current config file before proceeding
	if err := writer.VerifyOwnership(); err != nil {
		log.Fatal().Err(err).Msg("config file ownership check failed")
	}
	log.Info().Str("config_path", writer.GetConfigPath()).Str("interface", iface).Msg("config file ownership verified")

	log.Info().Str("config_path", writer.GetConfigPath()).Msg("writing initial configuration with Wirety marker")
	if err := writer.WriteAndApply(cfg); err != nil {
		log.Fatal().Err(err).Msg("failed applying initial config from resolve")
	}
	log.Info().Msg("initial configuration applied successfully")

	wsServer := server
	if len(server) > 7 && server[:7] == "http://" {
		wsServer = "ws://" + server[7:]
	} else if len(server) > 8 && server[:8] == "https://" {
		wsServer = "wss://" + server[8:]
	}
	wsURL := fmt.Sprintf("%s/api/v1/ws", wsServer)
	wsClient := ws.NewClientWithDialer(newWSDialer(skipTLSVerify))

	// Parse proxy ports
	httpPortInt := 3128
	httpsPortInt := 3129
	if p, err := strconv.Atoi(httpPort); err == nil {
		httpPortInt = p
	}
	if p, err := strconv.Atoi(httpsPort); err == nil {
		httpsPortInt = p
	}

	// Initialize firewall adapter with proxy ports
	fwAdapter := firewall.NewAdapter(iface, natIfaces)
	fwAdapter.SetProxyPorts(httpPortInt, httpsPortInt)
	fwAdapter.SetServerURL(server) // Allow peers to reach Wirety server before authentication

	runner := app.NewRunner(wsClient, writer, dnsServer, fwAdapter, wsURL, iface, peerID, networkID)

	// Pass enrollment token as Authorization header (keeps it out of access logs)
	wsHeaders := http.Header{}
	wsHeaders.Set("Authorization", "Bearer "+token)
	if serverHost != "" {
		wsHeaders.Set("Host", serverHost)
	}
	runner.SetHeaders(wsHeaders)
	runner.SetCaptivePortal(server, token, portalURL, httpClient)

	// Set the initial peer name in the runner
	runner.SetCurrentPeerName(peerName)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	stop := make(chan struct{})

	// Handle shutdown gracefully
	go func() {
		<-sigCh
		log.Info().Msg("shutdown signal received, stopping services...")

		close(stop)
	}()

	runner.Start(stop)
	log.Info().Msg("agent stopped")
}

func envOr(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

// sanitizeInterfaceName converts a peer name to a valid WireGuard interface name
// Interface names must be alphanumeric, underscore, or dash, max 15 chars
func sanitizeInterfaceName(peerName string) string {
	// Convert to lowercase for consistency first
	sanitized := strings.ToLower(peerName)

	// Replace invalid characters with underscores
	re := regexp.MustCompile(`[^a-z0-9_-]`)
	sanitized = re.ReplaceAllString(sanitized, "_")

	// Truncate to max 15 characters (Linux interface name limit)
	if len(sanitized) > 15 {
		sanitized = sanitized[:15]
		// Remove trailing underscores or dashes after truncation
		sanitized = strings.TrimRight(sanitized, "_-")
	}

	// If empty after sanitization, use default
	if sanitized == "" {
		sanitized = "wg0"
	}

	return sanitized
}

// hostOverrideTransport is an http.RoundTripper that sets the HTTP Host header
// on every request. Used when the server is accessed by IP behind a reverse proxy.
type hostOverrideTransport struct {
	host string
	base http.RoundTripper
}

func (t *hostOverrideTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Host = t.host
	return t.base.RoundTrip(r)
}

// baseTLSTransport returns an http.RoundTripper with TLS verification optionally disabled.
// When skipTLSVerify is false the standard http.DefaultTransport is returned unchanged.
func baseTLSTransport(skipTLSVerify bool) http.RoundTripper {
	if !skipTLSVerify {
		return http.DefaultTransport
	}
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 — intentional, controlled by SKIP_TLS_VERIFY
	}
}

// newHTTPClient returns an *http.Client configured with the given options:
//   - serverHost: when non-empty, sets the HTTP Host header on every request
//     (reverse-proxy / no-DNS setups).
//   - skipTLSVerify: when true, disables TLS certificate verification.
func newHTTPClient(serverHost string, skipTLSVerify bool) *http.Client {
	base := baseTLSTransport(skipTLSVerify)
	if serverHost == "" && !skipTLSVerify {
		return http.DefaultClient
	}
	transport := base
	if serverHost != "" {
		transport = &hostOverrideTransport{host: serverHost, base: base}
	}
	return &http.Client{Transport: transport}
}

// newWSDialer returns a *websocket.Dialer with TLS verification optionally disabled.
func newWSDialer(skipTLSVerify bool) *websocket.Dialer {
	if !skipTLSVerify {
		return websocket.DefaultDialer
	}
	return &websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true}, // #nosec G402 — intentional, controlled by SKIP_TLS_VERIFY
		HandshakeTimeout: websocket.DefaultDialer.HandshakeTimeout,
	}
}

// parseWireGuardAddress extracts the bare IP address from the "Address = <ip>/<prefix>"
// line of a WireGuard configuration string.
func parseWireGuardAddress(cfg string) (string, error) {
	for _, line := range strings.Split(cfg, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "address") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		// Strip optional CIDR prefix length (e.g. "10.0.0.1/24" → "10.0.0.1")
		addr := strings.TrimSpace(parts[1])
		return strings.Split(addr, "/")[0], nil
	}
	return "", fmt.Errorf("no Address line found in WireGuard config")
}

type resolveResponse struct {
	NetworkID string `json:"network_id"`
	PeerID    string `json:"peer_id"`
	PeerName  string `json:"peer_name"`
	Config    string `json:"config"`
}

func resolveToken(server, token string, client *http.Client) (string, string, string, string, error) {
	resolveURL := fmt.Sprintf("%s/api/v1/agent/resolve", server)
	req, err := http.NewRequest(http.MethodGet, resolveURL, nil) // #nosec G107 - server is trusted input
	if err != nil {
		return "", "", "", "", fmt.Errorf("resolve new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", "", fmt.Errorf("resolve http get: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", fmt.Errorf("resolve unexpected status: %s", resp.Status)
	}
	var rr resolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return "", "", "", "", fmt.Errorf("decode: %w", err)
	}
	return rr.NetworkID, rr.PeerID, rr.PeerName, rr.Config, nil
}

