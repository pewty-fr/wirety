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
	"wirety/agent/internal/adapters/proxy"
	"wirety/agent/internal/adapters/wg"
	"wirety/agent/internal/adapters/ws"
	app "wirety/agent/internal/application/agent"
	dom "wirety/agent/internal/domain/dns"
	"wirety/agent/internal/ports"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	server := envOr("SERVER_URL", "http://localhost:8080")
	token := envOr("TOKEN", "")
	configPath := envOr("WG_CONFIG_PATH", "")
	applyMethod := envOr("WG_APPLY_METHOD", "wg-quick")
	natIface := envOr("NAT_INTERFACE", "eth0")
	httpPort := envOr("HTTP_PROXY_PORT", "3128")
	httpsPort := envOr("HTTPS_PROXY_PORT", "3129")
	portalURL := envOr("CAPTIVE_PORTAL_URL", "https://portal.example.com")

	flag.StringVar(&server, "server", server, "Server base URL (no trailing /)")
	flag.StringVar(&token, "token", token, "Enrollment token")
	flag.StringVar(&configPath, "config", configPath, "Path to wireguard config file")
	flag.StringVar(&applyMethod, "apply", applyMethod, "Apply method: wg-quick|syncconf")
	flag.StringVar(&natIface, "nat", natIface, "NAT interface (eth0, etc.)")
	flag.StringVar(&httpPort, "http-port", httpPort, "HTTP proxy port for captive portal")
	flag.StringVar(&httpsPort, "https-port", httpsPort, "HTTPS proxy port for captive portal")
	flag.StringVar(&portalURL, "portal-url", portalURL, "Captive portal URL")
	flag.Parse()

	if token == "" {
		log.Fatal().Msg("TOKEN is required (env or flag)")
	}

	// First resolve token to get peer name for interface/config naming
	networkID, peerID, peerName, cfg, err := resolveToken(server, token)
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
	wsURL := fmt.Sprintf("%s/api/v1/ws?token=%s", wsServer, token)

	wsClient := ws.NewClient()
	dnsFactory := func(domain string, peers []dom.DNSPeer) ports.DNSStarterPort {
		return dnsadapter.NewServer(domain, peers)
	}

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
	fwAdapter := firewall.NewAdapter(iface, natIface)
	fwAdapter.SetProxyPorts(httpPortInt, httpsPortInt)

	// Initialize captive portal
	// Use server URL as portal URL if not explicitly set
	if portalURL == "https://portal.example.com" {
		portalURL = server
	}
	captivePortal := proxy.NewCaptivePortal(httpPortInt, httpsPortInt, portalURL, server, token)
	if err := captivePortal.Start(); err != nil {
		log.Fatal().Err(err).Msg("failed to start captive portal")
	}
	log.Info().
		Int("http_port", httpPortInt).
		Int("https_port", httpsPortInt).
		Str("portal_url", portalURL).
		Msg("captive portal started")

	// Initialize TLS-SNI gateway for HTTPS filtering
	// This gateway only allows connections to the server domain for non-authenticated users
	tlsGateway, err := proxy.NewTLSSNIGateway(443, server)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create TLS-SNI gateway")
	}
	if err := tlsGateway.Start(); err != nil {
		log.Fatal().Err(err).Msg("failed to start TLS-SNI gateway")
	}
	log.Info().
		Int("port", 443).
		Str("allowed_domain", server).
		Msg("TLS-SNI gateway started")

	runner := app.NewRunner(wsClient, writer, dnsFactory, fwAdapter, captivePortal, tlsGateway, wsURL, iface)

	// Set the initial peer name in the runner
	runner.SetCurrentPeerName(peerName)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	stop := make(chan struct{})
	go func() { <-sigCh; close(stop) }()
	runner.Start(stop)
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
	// Replace invalid characters with underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := re.ReplaceAllString(peerName, "_")

	// Convert to lowercase for consistency
	sanitized = strings.ToLower(sanitized)

	// Truncate to max 15 characters (Linux interface name limit)
	if len(sanitized) > 15 {
		sanitized = sanitized[:15]
	}

	// If empty after sanitization, use default
	if sanitized == "" {
		sanitized = "wg0"
	}

	return sanitized
}

type resolveResponse struct {
	NetworkID string `json:"network_id"`
	PeerID    string `json:"peer_id"`
	PeerName  string `json:"peer_name"`
	Config    string `json:"config"`
}

func resolveToken(server, token string) (string, string, string, string, error) {
	url := fmt.Sprintf("%s/api/v1/agent/resolve?token=%s", server, token)
	resp, err := http.Get(url) // #nosec G107 - server is trusted input
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
