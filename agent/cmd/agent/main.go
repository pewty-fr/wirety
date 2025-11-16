package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	cfgwriter "wirety/agent/internal/config"
	"wirety/agent/internal/dns"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	server := envOr("SERVER_URL", "http://localhost:8080")
	token := envOr("TOKEN", "")
	iface := envOr("WG_INTERFACE", "wg0")
	configPath := envOr("WG_CONFIG_PATH", "")
	applyMethod := envOr("WG_APPLY_METHOD", "wg-quick")

	flag.StringVar(&server, "server", server, "Server base URL (no trailing /)")
	flag.StringVar(&token, "token", token, "Enrollment token")
	flag.StringVar(&iface, "interface", iface, "WireGuard interface name")
	flag.StringVar(&configPath, "config", configPath, "Path to wireguard config file")
	flag.StringVar(&applyMethod, "apply", applyMethod, "Apply method: wg-quick|syncconf")
	flag.Parse()

	if token == "" {
		log.Fatal().Msg("TOKEN is required (env or flag)")
	}

	writer := cfgwriter.NewWriter(configPath, iface, applyMethod)

	// Resolve token & apply initial config
	networkID, peerID, cfg, err := resolveToken(server, token)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to resolve token")
	}
	log.Info().Str("network_id", networkID).Str("peer_id", peerID).Msg("resolved token")
	if err := writer.WriteAndApply(cfg); err != nil {
		log.Error().Err(err).Msg("failed applying initial config from resolve")
	}

	type wsConfigMsg struct {
		Config string `json:"config"`
		DNS    *struct {
			Domain string     `json:"domain"`
			Peers  []dns.Peer `json:"peers"`
		} `json:"dns,omitempty"`
	}

	wsServer := server
	if len(server) > 7 && server[:7] == "http://" {
		wsServer = "ws://" + server[7:]
	} else if len(server) > 8 && server[:8] == "https://" {
		wsServer = "wss://" + server[8:]
	}

	wsURL := fmt.Sprintf("%s/api/v1/ws?token=%s", wsServer, token)
	log.Info().Str("url", wsURL).Msg("connecting websocket (token mode)")

	backoff := time.Second
	maxBackoff := 30 * time.Second

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigCh:
			log.Info().Msg("shutdown signal received")
			return
		default:
		}

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Error().Err(err).Dur("retry", backoff).Msg("websocket dial failed")
			select {
			case <-sigCh:
				log.Info().Msg("shutdown signal received during backoff")
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = time.Second // reset on success

		log.Info().Msg("websocket connected; waiting for config messages")
	readLoop:
		for {
			select {
			case <-sigCh:
				log.Info().Msg("shutdown signal received")
				conn.Close()
				return
			default:
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Error().Err(err).Msg("websocket read error, reconnecting")
				conn.Close()
				break readLoop
			}

			var wsMsg wsConfigMsg
			if err := json.Unmarshal(msg, &wsMsg); err != nil {
				log.Error().Err(err).Msg("invalid config message format")
				continue
			}
			log.Info().Msg("received config update")
			if err := writer.WriteAndApply(wsMsg.Config); err != nil {
				log.Error().Err(err).Msg("failed applying config")
			} else {
				log.Debug().Msg("config applied successfully")
			}

			// Start DNS server if DNS config is present
			if wsMsg.DNS != nil {
				dnsServer := dns.NewServer(wsMsg.DNS.Domain, wsMsg.DNS.Peers)
				go func() {
					if err := dnsServer.Start(":5354"); err != nil {
						log.Error().Err(err).Msg("DNS server exited")
					}
				}()
			}
		}
	}
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
