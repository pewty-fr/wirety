package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
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
// Whitelist optional; list of authenticated peer IPs

type WSMessage struct {
	Config    string          `json:"config"`
	DNS       *dom.DNSConfig  `json:"dns,omitempty"`
	Policy    *pol.JumpPolicy `json:"policy,omitempty"`
	PeerID    string          `json:"peer_id,omitempty"`
	PeerName  string          `json:"peer_name,omitempty"`
	Whitelist []string        `json:"whitelist,omitempty"` // IPs of authenticated non-agent peers
}

type Runner struct {
	wsClient          ports.WebSocketClientPort
	cfgWriter         ports.ConfigWriterPort
	dnsFactory        func(domain string, peers []dom.DNSPeer) ports.DNSStarterPort // factory to create DNS server instance
	fwAdapter         ports.FirewallPort
	captivePortal     ports.CaptivePortalPort
	wsURL             string
	wgInterface       string
	currentPeerName   string // Track current peer name to detect changes
	backoffBase       time.Duration
	backoffMax        time.Duration
	heartbeatInterval time.Duration
}

func NewRunner(wsClient ports.WebSocketClientPort, writer ports.ConfigWriterPort, dnsFactory func(string, []dom.DNSPeer) ports.DNSStarterPort, fwAdapter ports.FirewallPort, captivePortal ports.CaptivePortalPort, wsURL string, wgInterface string) *Runner {
	return &Runner{
		wsClient:          wsClient,
		cfgWriter:         writer,
		dnsFactory:        dnsFactory,
		fwAdapter:         fwAdapter,
		captivePortal:     captivePortal,
		wsURL:             wsURL,
		wgInterface:       wgInterface,
		currentPeerName:   "", // Will be set when first message is received
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

		// Start heartbeat goroutine with endpoint change detection
		heartbeatTicker := time.NewTicker(r.heartbeatInterval)
		defer heartbeatTicker.Stop()
		endpointCheckTicker := time.NewTicker(300 * time.Millisecond)
		defer endpointCheckTicker.Stop()
		heartbeatDone := make(chan struct{})

		// Track last known peer endpoints
		var lastPeerEndpoints map[string]string
		var lastPeerEndpointsMu sync.RWMutex

		go func() {
			for {
				select {
				case <-heartbeatDone:
					return
				case <-heartbeatTicker.C:
					// Regular heartbeat every 30 seconds
					r.sendHeartbeat()

					// Update last known endpoints after sending
					sysInfo, err := CollectSystemInfo(r.wgInterface)
					if err == nil {
						lastPeerEndpointsMu.Lock()
						lastPeerEndpoints = sysInfo.PeerEndpoints
						lastPeerEndpointsMu.Unlock()
					}
				case <-endpointCheckTicker.C:
					// Check for endpoint changes every 300ms
					sysInfo, err := CollectSystemInfo(r.wgInterface)
					if err != nil {
						continue
					}

					// Compare with last known endpoints
					lastPeerEndpointsMu.RLock()
					changed := false
					if lastPeerEndpoints == nil {
						// First check, initialize
						changed = false
					} else if len(sysInfo.PeerEndpoints) != len(lastPeerEndpoints) {
						// Number of peers changed
						changed = true
					} else {
						// Check if any endpoint changed
						for pubKey, endpoint := range sysInfo.PeerEndpoints {
							if lastEndpoint, exists := lastPeerEndpoints[pubKey]; !exists || lastEndpoint != endpoint {
								changed = true
								break
							}
						}
					}
					lastPeerEndpointsMu.RUnlock()

					if changed {
						log.Info().Msg("peer endpoints changed, sending immediate heartbeat")
						r.sendHeartbeat()

						// Update last known endpoints
						lastPeerEndpointsMu.Lock()
						lastPeerEndpoints = sysInfo.PeerEndpoints
						lastPeerEndpointsMu.Unlock()
					}
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

			// Handle peer name changes
			if payload.PeerName != "" {
				if err := r.handlePeerNameChange(payload.PeerName); err != nil {
					log.Error().Err(err).Str("new_name", payload.PeerName).Msg("failed to handle peer name change")
					continue
				}
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
			// Handle whitelist updates
			if payload.Whitelist != nil && r.captivePortal != nil {
				log.Info().Int("count", len(payload.Whitelist)).Msg("updating whitelist")
				// Clear existing whitelist and add new ones
				r.captivePortal.ClearWhitelist()
				for _, ip := range payload.Whitelist {
					r.captivePortal.AddWhitelistedPeer(ip)
				}
			}

			if payload.Policy != nil && r.fwAdapter != nil {
				log.Info().Int("peer_count", len(payload.Policy.Peers)).Msg("applying firewall policy")

				// Update captive portal with non-agent peers
				if r.captivePortal != nil {
					nonAgentIPs := make([]string, 0)
					for _, peer := range payload.Policy.Peers {
						if !peer.UseAgent {
							nonAgentIPs = append(nonAgentIPs, peer.IP)
						}
					}
					r.captivePortal.UpdateNonAgentPeers(nonAgentIPs)
				}

				// Get whitelisted IPs for firewall rules
				whitelistedIPs := payload.Whitelist
				if whitelistedIPs == nil {
					whitelistedIPs = []string{}
				}

				if err := r.fwAdapter.Sync(payload.Policy, payload.Policy.IP, whitelistedIPs); err != nil {
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
		"hostname":         sysInfo.Hostname,
		"system_uptime":    sysInfo.SystemUptime,
		"wireguard_uptime": sysInfo.WireGuardUptime,
		"peer_endpoints":   sysInfo.PeerEndpoints,
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
			Int64("wireguard_uptime", sysInfo.WireGuardUptime).
			Interface("peer_endpoints", sysInfo.PeerEndpoints).
			Msg("heartbeat sent")
	}
}

// sanitizeInterfaceName converts a peer name to a valid WireGuard interface name
// Interface names must be alphanumeric, underscore, or dash, max 15 chars
func (r *Runner) sanitizeInterfaceName(peerName string) string {
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

// SetCurrentPeerName sets the current peer name (used for initialization)
func (r *Runner) SetCurrentPeerName(peerName string) {
	r.currentPeerName = peerName
}

// handlePeerNameChange detects if the peer name has changed and handles interface transition
func (r *Runner) handlePeerNameChange(newPeerName string) error {
	// Skip if no name provided or same as current
	if newPeerName == "" || newPeerName == r.currentPeerName {
		return nil
	}

	// Calculate new interface name
	newInterface := r.sanitizeInterfaceName(newPeerName)
	currentInterface := r.cfgWriter.GetInterface()

	// Check if interface name actually changes
	if newInterface == currentInterface {
		r.currentPeerName = newPeerName
		return nil
	}

	log.Info().
		Str("old_peer_name", r.currentPeerName).
		Str("new_peer_name", newPeerName).
		Str("old_interface", currentInterface).
		Str("new_interface", newInterface).
		Msg("peer name changed, transitioning interface")

	// Update the config writer to use the new interface
	// This will handle cleanup of the old interface and config file
	if err := r.cfgWriter.UpdateInterface(newInterface); err != nil {
		return fmt.Errorf("failed to update interface: %w", err)
	}

	// Update our tracking variables
	r.currentPeerName = newPeerName
	r.wgInterface = newInterface

	// Update the firewall adapter to use the new interface
	if r.fwAdapter != nil {
		// Note: This assumes the firewall adapter can handle interface changes
		// The firewall rules will be updated when the policy is next applied
		log.Debug().Str("new_interface", newInterface).Msg("firewall will be updated with new interface")
	}

	return nil
}
