package main

import (
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
	serverHost := envOr("SERVER_HOST", "") // optional Host header override for reverse-proxy setups

	flag.StringVar(&server, "server", server, "Server base URL (no trailing /)")
	flag.StringVar(&token, "token", token, "Enrollment token")
	flag.StringVar(&configPath, "config", configPath, "Path to wireguard config file")
	flag.StringVar(&applyMethod, "apply", applyMethod, "Apply method: wg-quick|syncconf")
	flag.StringVar(&natIfacesStr, "nat-interfaces", natIfacesStr, "Comma-separated NAT interfaces (empty = auto-detect all egress interfaces)")
	flag.StringVar(&portalURL, "portal-url", portalURL, "Captive portal page URL (default: <server>/captive-portal)")
	flag.StringVar(&serverHost, "server-host", serverHost, "Override HTTP Host header for all requests to the server (useful when accessing via IP behind a reverse proxy)")
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

	log.Info().Msg("starting DNS server")
	dnsServer := dnsadapter.NewServer("", []dom.DNSPeer{})
	go func() {
		if err := dnsServer.Start(":53"); err != nil {
			log.Error().Err(err).Msg("dns server exited")
		}
	}()

	// Build a shared HTTP client that injects the Host header on every request
	// when SERVER_HOST is set (reverse-proxy / no-DNS setups).
	httpClient := newHTTPClient(serverHost)

	// First resolve token to get peer name for interface/config naming
	networkID, peerID, peerName, cfg, err := resolveToken(server, token, httpClient)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to resolve token")
	}
	log.Info().Str("network_id", networkID).Str("peer_id", peerID).Str("peer_name", peerName).Msg("resolved token")

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
	wsClient := ws.NewClient()

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

// newHTTPClient returns an *http.Client that injects the given Host header on
// every request. If serverHost is empty the default client is returned unchanged.
func newHTTPClient(serverHost string) *http.Client {
	if serverHost == "" {
		return http.DefaultClient
	}
	return &http.Client{
		Transport: &hostOverrideTransport{
			host: serverHost,
			base: http.DefaultTransport,
		},
	}
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

