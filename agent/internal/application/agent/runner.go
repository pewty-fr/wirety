package agent

import (
	"encoding/json"
	"fmt"
	"time"
	dom "wirety/agent/internal/domain/dns"
	pol "wirety/agent/internal/domain/policy"
	"wirety/agent/internal/ports"

	"github.com/rs/zerolog/log"
)

// WebSocket message shape from server
// DNS optional; only for jump peer
// Peers list contains name + ip
// Policy optional; only for jump peer

type WSMessage struct {
	Config string          `json:"config"`
	DNS    *dom.DNSConfig  `json:"dns,omitempty"`
	Policy *pol.JumpPolicy `json:"policy,omitempty"`
}

type Runner struct {
	wsClient          ports.WebSocketClientPort
	cfgWriter         ports.ConfigWriterPort
	dnsFactory        func(domain string, peers []dom.DNSPeer) ports.DNSStarterPort // factory to create DNS server instance
	fwAdapter         ports.FirewallPort
	wsURL             string
	wgInterface       string
	backoffBase       time.Duration
	backoffMax        time.Duration
	heartbeatInterval time.Duration
}

func NewRunner(wsClient ports.WebSocketClientPort, writer ports.ConfigWriterPort, dnsFactory func(string, []dom.DNSPeer) ports.DNSStarterPort, fwAdapter ports.FirewallPort, wsURL string, wgInterface string) *Runner {
	return &Runner{
		wsClient:          wsClient,
		cfgWriter:         writer,
		dnsFactory:        dnsFactory,
		fwAdapter:         fwAdapter,
		wsURL:             wsURL,
		wgInterface:       wgInterface,
		backoffBase:       time.Second,
		backoffMax:        30 * time.Second,
		heartbeatInterval: 30 * time.Second, // Send heartbeat every 30 seconds
	}
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

		// Start heartbeat goroutine
		heartbeatTicker := time.NewTicker(r.heartbeatInterval)
		defer heartbeatTicker.Stop()
		heartbeatDone := make(chan struct{})

		go func() {
			for {
				select {
				case <-heartbeatDone:
					return
				case <-heartbeatTicker.C:
					r.sendHeartbeat()
				}
			}
		}()

		for {
			select {
			case <-stop:
				close(heartbeatDone)
				_ = r.wsClient.Close()
				return
			default:
			}
			msgBytes, err := r.wsClient.ReadMessage()
			if err != nil {
				log.Error().Err(err).Msg("websocket read error; reconnecting")
				close(heartbeatDone)
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
			if payload.Policy != nil && r.fwAdapter != nil {
				log.Info().Int("peer_count", len(payload.Policy.Peers)).Msg("applying firewall policy")
				if err := r.fwAdapter.Sync(payload.Policy, payload.Policy.IP); err != nil {
					log.Error().Err(err).Msg("failed applying firewall policy")
				} else {
					log.Debug().Msg("firewall policy applied")
				}
			}
		}
	}
}

// sendHeartbeat sends system information to the server
func (r *Runner) sendHeartbeat() {
	sysInfo, err := CollectSystemInfo(r.wgInterface)
	if err != nil {
		log.Error().Err(err).Msg("failed to collect system info for heartbeat")
		return
	}

	heartbeat := map[string]interface{}{
		"hostname":          sysInfo.Hostname,
		"system_uptime":     sysInfo.SystemUptime,
		"wireguard_uptime":  sysInfo.WireGuardUptime,
		"reported_endpoint": sysInfo.ReportedEndpoint,
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal heartbeat")
		return
	}

	if err := r.wsClient.WriteMessage(data); err != nil {
		log.Debug().Err(err).Msg("failed to send heartbeat (will retry)")
	} else {
		log.Trace().
			Str("hostname", sysInfo.Hostname).
			Int64("system_uptime", sysInfo.SystemUptime).
			Msg("heartbeat sent")
	}
}
