package agent

import (
	"encoding/json"
	"fmt"
	"time"
	dom "wirety/agent/internal/domain/dns"
	"wirety/agent/internal/ports"

	"github.com/rs/zerolog/log"
)

// WebSocket message shape from server
// DNS optional; only for jump peer
// Peers list contains name + ip

type WSMessage struct {
	Config string         `json:"config"`
	DNS    *dom.DNSConfig `json:"dns,omitempty"`
}

type Runner struct {
	wsClient    ports.WebSocketClientPort
	cfgWriter   ports.ConfigWriterPort
	dnsFactory  func(domain string, peers []dom.DNSPeer) ports.DNSStarterPort // factory to create DNS server instance
	wsURL       string
	backoffBase time.Duration
	backoffMax  time.Duration
}

func NewRunner(wsClient ports.WebSocketClientPort, writer ports.ConfigWriterPort, dnsFactory func(string, []dom.DNSPeer) ports.DNSStarterPort, wsURL string) *Runner {
	return &Runner{wsClient: wsClient, cfgWriter: writer, dnsFactory: dnsFactory, wsURL: wsURL, backoffBase: time.Second, backoffMax: 30 * time.Second}
}

func (r *Runner) Start(stop <-chan struct{}) {
	backoff := r.backoffBase
	for {
		select {
		case <-stop:
			log.Info().Msg("agent runner stopping")
			return
		default:
		}
		if err := r.wsClient.Connect(r.wsURL); err != nil {
			log.Error().Err(err).Dur("retry", backoff).Msg("websocket connect failed")
			select {
			case <-stop:
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > r.backoffMax {
				backoff = r.backoffMax
			}
			continue
		}
		backoff = r.backoffBase
		log.Info().Str("url", r.wsURL).Msg("websocket connected")
		for {
			select {
			case <-stop:
				_ = r.wsClient.Close()
				return
			default:
			}
			msgBytes, err := r.wsClient.ReadMessage()
			if err != nil {
				log.Error().Err(err).Msg("websocket read error; reconnecting")
				_ = r.wsClient.Close()
				break
			}
			var payload WSMessage
			if err := json.Unmarshal(msgBytes, &payload); err != nil {
				log.Error().Err(err).Msg("invalid websocket message")
				continue
			}
			if err := r.cfgWriter.WriteAndApply(payload.Config); err != nil {
				log.Error().Err(err).Msg("failed applying config")
			} else {
				log.Debug().Msg("config applied")
			}
			if payload.DNS != nil {
				log.Info().Str("domain", payload.DNS.Domain).Int("peer_count", len(payload.DNS.Peers)).Msg("starting DNS server")
				server := r.dnsFactory(payload.DNS.Domain, payload.DNS.Peers)
				go func() {
					if err := server.Start(fmt.Sprintf("%s:53", payload.DNS.IP)); err != nil {
						log.Error().Err(err).Msg("dns server exited")
					}
				}()
			}
		}
	}
}
