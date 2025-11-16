package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	dnsadapter "wirety/agent/internal/adapters/dns"
	"wirety/agent/internal/adapters/firewall"
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
	iface := envOr("WG_INTERFACE", "wg0")
	configPath := envOr("WG_CONFIG_PATH", "")
	applyMethod := envOr("WG_APPLY_METHOD", "wg-quick")
	natIface := envOr("NAT_INTERFACE", "eth0")

	flag.StringVar(&server, "server", server, "Server base URL (no trailing /)")
	flag.StringVar(&token, "token", token, "Enrollment token")
	flag.StringVar(&iface, "interface", iface, "WireGuard interface name")
	flag.StringVar(&configPath, "config", configPath, "Path to wireguard config file")
	flag.StringVar(&applyMethod, "apply", applyMethod, "Apply method: wg-quick|syncconf")
	flag.StringVar(&natIface, "nat", natIface, "NAT interface (eth0, etc.)")
	flag.Parse()

	if token == "" {
		log.Fatal().Msg("TOKEN is required (env or flag)")
	}

	writer := wg.NewWriter(configPath, iface, applyMethod)

	networkID, peerID, cfg, err := resolveToken(server, token)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to resolve token")
	}
	log.Info().Str("network_id", networkID).Str("peer_id", peerID).Msg("resolved token")
	if err := writer.WriteAndApply(cfg); err != nil {
		log.Error().Err(err).Msg("failed applying initial config from resolve")
	}

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
	fwAdapter := firewall.NewAdapter(iface, natIface)
	runner := app.NewRunner(wsClient, writer, dnsFactory, fwAdapter, wsURL)

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

type resolveResponse struct {
	NetworkID string `json:"network_id"`
	PeerID    string `json:"peer_id"`
	Config    string `json:"config"`
}

func resolveToken(server, token string) (string, string, string, error) {
	url := fmt.Sprintf("%s/api/v1/agent/resolve?token=%s", server, token)
	resp, err := http.Get(url) // #nosec G107 - server is trusted input
	if err != nil {
		return "", "", "", fmt.Errorf("resolve http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("resolve unexpected status: %s", resp.Status)
	}
	var rr resolveResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return "", "", "", fmt.Errorf("decode: %w", err)
	}
	return rr.NetworkID, rr.PeerID, rr.Config, nil
}
