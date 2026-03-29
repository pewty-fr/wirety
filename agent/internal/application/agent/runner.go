package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	dom "wirety/agent/internal/domain/dns"
	pol "wirety/agent/internal/domain/policy"
	"wirety/agent/internal/ports"

	"github.com/rs/zerolog/log"

	"wirety/agent/internal/audit"
)

// WebSocket message shape from server
// DNS optional; only for jump peer
// Peers list contains name + ip
// Policy optional; only for jump peer
// Whitelist optional; list of authenticated peer IPs

type WSMessage struct {
	Config      string          `json:"config"`
	DNS         *dom.DNSConfig  `json:"dns,omitempty"`
	Policy      *pol.JumpPolicy `json:"policy,omitempty"`
	PeerID      string          `json:"peer_id,omitempty"`
	PeerName    string          `json:"peer_name,omitempty"`
	Whitelist   []string        `json:"whitelist,omitempty"`    // IPs of authenticated non-agent peers
	OAuthIssuer string          `json:"oauth_issuer,omitempty"` // OAuth issuer URL for TLS-SNI gateway
}

// tunnelActive is the threshold below which a peer is considered connected.
// WireGuard sends a keepalive every 25s and considers a peer inactive after ~180s.
const tunnelActiveThreshold = 180 * time.Second

// tunnelPollInterval is how often the runner checks for handshake changes.
const tunnelPollInterval = 15 * time.Second

type Runner struct {
	wsClient          ports.WebSocketClientPort
	cfgWriter         ports.ConfigWriterPort
	dnsServer         ports.DNSStarterPort // active DNS server instance
	dnsServerMu       sync.Mutex           // protects dnsServer
	fwAdapter         ports.FirewallPort
	wsURL             string
	wsHeaders         http.Header
	wgInterface       string
	currentPeerName   string // Track current peer name to detect changes
	peerID            string // for audit logging
	networkID         string // for audit logging
	// peerNames maps WireGuard public key → peer name (updated on each WSMessage).
	peerNames   map[string]string
	peerNamesMu sync.RWMutex
	backoffBase       time.Duration
	backoffMax        time.Duration
	heartbeatInterval time.Duration
}

func NewRunner(wsClient ports.WebSocketClientPort, writer ports.ConfigWriterPort, dnsServer ports.DNSStarterPort, fwAdapter ports.FirewallPort, wsURL string, wgInterface string, peerID string, networkID string) *Runner {
	return &Runner{
		wsClient:          wsClient,
		cfgWriter:         writer,
		dnsServer:         dnsServer,
		fwAdapter:         fwAdapter,
		wsURL:             wsURL,
		wgInterface:       wgInterface,
		currentPeerName:   "", // Will be set when first message is received
		peerID:            peerID,
		networkID:         networkID,
		peerNames:         make(map[string]string),
		backoffBase:       time.Second,
		backoffMax:        30 * time.Second,
		heartbeatInterval: 30 * time.Second, // Send heartbeat every 30 seconds
	}
}

// SetHeaders sets HTTP headers to send on WebSocket connection (e.g. Authorization).
func (r *Runner) SetHeaders(header http.Header) {
	r.wsHeaders = header
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
		if err := r.wsClient.Connect(r.wsURL, r.wsHeaders); err != nil {
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
		var heartbeatWg sync.WaitGroup

		// Start tunnel connection monitor
		heartbeatWg.Add(1)
		go func() {
			defer heartbeatWg.Done()
			r.monitorTunnels(heartbeatDone)
		}()

		// Track last known peer endpoints
		var lastPeerEndpoints map[string]string
		var lastPeerEndpointsMu sync.RWMutex

		heartbeatWg.Add(1)
		go func() {
			defer heartbeatWg.Done()
			for {
				select {
				case <-heartbeatDone:
					log.Debug().Msg("heartbeat goroutine stopping")
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
				log.Debug().Msg("stop signal received, closing heartbeat")
				close(heartbeatDone)
				heartbeatWg.Wait() // Wait for heartbeat goroutine to finish
				_ = r.wsClient.Close()
				return
			default:
			}
			msgBytes, err := r.wsClient.ReadMessage()
			if err != nil {
				log.Error().Err(err).Msg("websocket read error; reconnecting")
				close(heartbeatDone)
				heartbeatWg.Wait() // Wait for heartbeat goroutine to finish
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

			// Update pubkey → name map used by the tunnel monitor
			if payload.DNS != nil {
				r.updatePeerNames(payload.Config, payload.DNS.Peers)
			}

			if err := r.cfgWriter.WriteAndApply(payload.Config); err != nil {
				log.Error().Err(err).Msg("failed applying config")
			} else {
				log.Debug().Msg("config applied")
				audit.Agent(r.peerID, r.networkID).
					Str("action", "config.sync").
					Msg("audit")
			}

			// Handle DNS server: start once, update on subsequent messages
			if payload.DNS != nil {
				r.dnsServerMu.Lock()
				if r.dnsServer == nil {

					// Set upstream DNS servers for forwarding
					if len(payload.DNS.UpstreamServers) > 0 {
						r.dnsServer.SetUpstreamServers(payload.DNS.UpstreamServers)
					}

				} else {
					// Subsequent times: update existing DNS server
					log.Info().
						Str("domain", payload.DNS.Domain).
						Int("peer_count", len(payload.DNS.Peers)).
						Strs("upstream_servers", payload.DNS.UpstreamServers).
						Msg("updating DNS server configuration")
					r.dnsServer.Update(payload.DNS.Domain, payload.DNS.Peers)
					audit.Agent(r.peerID, r.networkID).
						Str("action", "dns.update").
						Str("domain", payload.DNS.Domain).
						Int("peer_count", len(payload.DNS.Peers)).
						Msg("audit")

					// Update upstream DNS servers
					if len(payload.DNS.UpstreamServers) > 0 {
						r.dnsServer.SetUpstreamServers(payload.DNS.UpstreamServers)
					}
				}
				r.dnsServerMu.Unlock()
			}

			if payload.Policy != nil && r.fwAdapter != nil {
				log.Info().
					Int("iptables_rule_count", len(payload.Policy.IPTablesRules)).
					Msg("applying firewall policy update")

				// Get whitelisted IPs for firewall rules
				whitelistedIPs := payload.Whitelist
				if whitelistedIPs == nil {
					whitelistedIPs = []string{}
				}

				// Apply policy-based iptables rules atomically
				// The Sync method flushes the chain and applies all rules in order
				if err := r.fwAdapter.Sync(payload.Policy, payload.Policy.IP, whitelistedIPs); err != nil {
					log.Error().Err(err).Msg("failed applying firewall policy update")
				} else {
					log.Info().
						Int("iptables_rule_count", len(payload.Policy.IPTablesRules)).
						Msg("firewall policy update applied successfully")
					audit.Agent(r.peerID, r.networkID).
						Str("action", "firewall.sync").
						Int("rule_count", len(payload.Policy.IPTablesRules)).
						Msg("audit")
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

	oldName := r.currentPeerName

	// Update our tracking variables
	r.currentPeerName = newPeerName
	r.wgInterface = newInterface

	audit.Agent(r.peerID, r.networkID).
		Str("action", "peer.rename").
		Str("old_name", oldName).
		Str("new_name", newPeerName).
		Str("new_interface", newInterface).
		Msg("audit")

	// Update the firewall adapter to use the new interface
	if r.fwAdapter != nil {
		// Note: This assumes the firewall adapter can handle interface changes
		// The firewall rules will be updated when the policy is next applied
		log.Debug().Str("new_interface", newInterface).Msg("firewall will be updated with new interface")
	}

	return nil
}

// updatePeerNames rebuilds the pubkey → peer name map from the WireGuard config and
// DNS peers list. Called each time a new WSMessage arrives with DNS data.
func (r *Runner) updatePeerNames(wgConfig string, dnsPeers []dom.DNSPeer) {
	// Build IP → name from DNS peers.
	ipToName := make(map[string]string, len(dnsPeers))
	for _, p := range dnsPeers {
		ip := strings.TrimSuffix(p.IP, "/32")
		ipToName[ip] = p.Name
	}

	// Parse [Peer] sections from the WireGuard config to get pubkey → allowedIP.
	names := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(wgConfig))
	var currentKey string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "PublicKey") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				currentKey = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(line, "AllowedIPs") && currentKey != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				for _, cidr := range strings.Split(parts[1], ",") {
					ip := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(cidr), "/32"))
					if name, ok := ipToName[ip]; ok {
						names[currentKey] = name
						break
					}
				}
			}
			currentKey = ""
		}
	}

	r.peerNamesMu.Lock()
	r.peerNames = names
	r.peerNamesMu.Unlock()
}

// peerNameFor returns the human-readable name for a WireGuard public key, or a
// truncated key as fallback.
func (r *Runner) peerNameFor(pubkey string) string {
	r.peerNamesMu.RLock()
	name := r.peerNames[pubkey]
	r.peerNamesMu.RUnlock()
	if name != "" {
		return name
	}
	if len(pubkey) > 12 {
		return pubkey[:12] + "..."
	}
	return pubkey
}

// monitorTunnels polls WireGuard handshake timestamps and emits log/audit events
// whenever a peer transitions between connected and disconnected.
func (r *Runner) monitorTunnels(done <-chan struct{}) {
	ticker := time.NewTicker(tunnelPollInterval)
	defer ticker.Stop()

	// pubkey → was connected on last poll
	states := make(map[string]bool)
	// pubkey → time the current session started
	connectedSince := make(map[string]time.Time)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
		}

		handshakes := GetWireGuardHandshakes(r.wgInterface)
		endpoints := getWireGuardEndpoints(r.wgInterface)
		now := time.Now()

		// Mark peers that still appear in handshakes as seen.
		seen := make(map[string]bool, len(handshakes))
		for pubkey, ts := range handshakes {
			seen[pubkey] = true
			isActive := now.Sub(ts) < tunnelActiveThreshold
			wasActive := states[pubkey]

			if isActive && !wasActive {
				// Peer just connected.
				connectedSince[pubkey] = ts
				name := r.peerNameFor(pubkey)
				endpoint := endpoints[pubkey]
				log.Info().
					Str("event", "tunnel.connected").
					Str("peer_name", name).
					Str("peer_pubkey", pubkey).
					Str("endpoint", endpoint).
					Time("handshake_at", ts).
					Msg("WireGuard tunnel connected")
				audit.Agent(r.peerID, r.networkID).
					Str("action", "tunnel.connected").
					Str("peer_name", name).
					Str("peer_pubkey", pubkey).
					Str("endpoint", endpoint).
					Time("handshake_at", ts).
					Msg("audit")
			} else if !isActive && wasActive {
				// Peer just disconnected.
				name := r.peerNameFor(pubkey)
				endpoint := endpoints[pubkey]
				since := connectedSince[pubkey]
				duration := now.Sub(since)
				log.Info().
					Str("event", "tunnel.disconnected").
					Str("peer_name", name).
					Str("peer_pubkey", pubkey).
					Str("endpoint", endpoint).
					Time("last_handshake", ts).
					Dur("session_duration", duration).
					Msg("WireGuard tunnel disconnected")
				audit.Agent(r.peerID, r.networkID).
					Str("action", "tunnel.disconnected").
					Str("peer_name", name).
					Str("peer_pubkey", pubkey).
					Str("endpoint", endpoint).
					Time("last_handshake", ts).
					Dur("session_duration", duration).
					Msg("audit")
				delete(connectedSince, pubkey)
			}
			states[pubkey] = isActive
		}

		// Detect peers that disappeared entirely from handshakes (removed from config).
		for pubkey, wasActive := range states {
			if !seen[pubkey] {
				if wasActive {
					name := r.peerNameFor(pubkey)
					since := connectedSince[pubkey]
					log.Info().
						Str("event", "tunnel.disconnected").
						Str("peer_name", name).
						Str("peer_pubkey", pubkey).
						Dur("session_duration", now.Sub(since)).
						Msg("WireGuard tunnel disconnected (peer removed)")
					audit.Agent(r.peerID, r.networkID).
						Str("action", "tunnel.disconnected").
						Str("peer_name", name).
						Str("peer_pubkey", pubkey).
						Dur("session_duration", now.Sub(since)).
						Msg("audit")
					delete(connectedSince, pubkey)
				}
				delete(states, pubkey)
			}
		}
	}
}
