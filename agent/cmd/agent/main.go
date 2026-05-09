package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net"
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
	// Collect defaults from env first; CLI flags override them.
	// Log configuration must be applied after flag.Parse so that flags take
	// precedence over environment variables.
	logLevel := envOr("LOG_LEVEL", "info")
	logFormat := envOr("LOG_FORMAT", "text")
	auditEnabled := envOr("AUDIT_LOG", "false") == "true"

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

	flag.StringVar(&logLevel, "log-level", logLevel, "Log verbosity: trace|debug|info|warn|error|fatal (env: LOG_LEVEL)")
	flag.StringVar(&logFormat, "log-format", logFormat, "Log output format: text|json (env: LOG_FORMAT)")
	flag.BoolVar(&auditEnabled, "audit-log", auditEnabled, "Emit JSON audit events to stdout (env: AUDIT_LOG)")
	flag.StringVar(&server, "server", server, "Server base URL (no trailing /)")
	flag.StringVar(&token, "token", token, "Enrollment token")
	flag.StringVar(&configPath, "config", configPath, "Path to wireguard config file")
	flag.StringVar(&applyMethod, "apply", applyMethod, "Apply method: wg-quick|syncconf")
	flag.StringVar(&natIfacesStr, "nat-interfaces", natIfacesStr, "Comma-separated NAT interfaces (empty = auto-detect all egress interfaces)")
	flag.StringVar(&portalURL, "portal-url", portalURL, "Captive portal page URL (default: <server>/captive-portal)")
	flag.StringVar(&serverHost, "server-host", serverHost, "Override HTTP Host header for all requests to the server (useful when accessing via IP behind a reverse proxy)")
	flag.BoolVar(&skipTLSVerify, "skip-tls-verify", skipTLSVerify, "Skip TLS certificate verification (insecure — use only with self-signed certificates in trusted environments)")
	flag.Parse()

	// Apply log settings now that flags are resolved.
	configureLogger(logLevel, logFormat)
	audit.Init(auditEnabled)

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

	// Bind the DNS server to the WireGuard interface IP(s) so it is reachable by
	// VPN peers through the tunnel, without conflicting with systemd-resolved
	// (127.0.0.53:53).  Dual-stack peers get separate listeners on each family.
	wgIP, wgIPv6, err := parseWireGuardAddresses(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse WireGuard address from config")
	}
	log.Info().Str("ipv4", wgIP).Str("ipv6", wgIPv6).Msg("parsed WireGuard interface addresses")
	dnsServer := dnsadapter.NewServer("", []dom.DNSPeer{})
	if wgIP != "" {
		dnsListenAddr := net.JoinHostPort(wgIP, "53")
		log.Info().Str("addr", dnsListenAddr).Msg("starting DNS server (IPv4)")
		go func() {
			if err := dnsServer.Start(dnsListenAddr); err != nil {
				log.Error().Err(err).Msg("dns server (IPv4) exited")
			}
		}()
	}
	if wgIPv6 != "" {
		dnsListenAddr6 := net.JoinHostPort(wgIPv6, "53")
		log.Info().Str("addr", dnsListenAddr6).Msg("starting DNS server (IPv6)")
		go func() {
			if err := dnsServer.Start(dnsListenAddr6); err != nil {
				log.Error().Err(err).Msg("dns server (IPv6) exited")
			}
		}()
	}

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

	// Load required kernel modules (nf_conntrack, xt_string) before the first
	// iptables sync. Best-effort: failures are logged and the agent continues with
	// degraded vhost isolation rather than refusing to start.
	fwAdapter.EnsureKernelModules()

	runner := app.NewRunner(wsClient, writer, dnsServer, fwAdapter, wsURL, iface, peerID, networkID)
	runner.SetWGIP(wgIP)
	if wgIPv6 != "" {
		runner.SetWGIPv6(wgIPv6)
	}

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

// configureLogger sets the global zerolog level and output format.
// level: trace|debug|info|warn|error|fatal (default: info)
// format: json|text (default: text — coloured console writer)
func configureLogger(level, format string) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	if format == "json" {
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	} else {
		log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	}
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

// parseWireGuardAddresses extracts the bare IPv4 and IPv6 addresses from a
// WireGuard configuration's `Address = ...` line.  The line may contain one
// address (single-stack) or two comma-separated addresses (dual-stack), each
// with an optional `/prefix` suffix that we strip.
//
// Returns the IPv4 address (or "" if none) and the IPv6 address (or "" if none).
// At least one of the two must be set or an error is returned.
//
// Examples of valid input lines (from the server's wireguard.GenerateConfig):
//   Address = 10.0.0.5/22
//   Address = fd12:3456:789a:bcde::5/64
//   Address = 10.0.0.5/22, fd12:3456:789a:bcde::5/64
func parseWireGuardAddresses(cfg string) (ipv4, ipv6 string, err error) {
	for _, line := range strings.Split(cfg, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(strings.ToLower(line), "address") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		// Address line can carry multiple comma-separated addresses for
		// dual-stack peers.  Split per-address before stripping CIDR — splitting
		// on `/` first would chop the "/64" out of an IPv6 address mid-string
		// and produce garbage (or, like the bug we're fixing, leave the whole
		// "ipv4, ipv6" pair concatenated).
		for _, raw := range strings.Split(parts[1], ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			// Strip optional /prefix.  Note: an IPv6 address has many colons;
			// the slash is unambiguous.
			if idx := strings.IndexByte(raw, '/'); idx != -1 {
				raw = raw[:idx]
			}
			parsed := net.ParseIP(raw)
			if parsed == nil {
				continue
			}
			if parsed.To4() != nil {
				if ipv4 == "" {
					ipv4 = raw
				}
			} else {
				if ipv6 == "" {
					ipv6 = raw
				}
			}
		}
		break
	}
	if ipv4 == "" && ipv6 == "" {
		return "", "", fmt.Errorf("no Address line found in WireGuard config (or the line had no parseable IP)")
	}
	return ipv4, ipv6, nil
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

