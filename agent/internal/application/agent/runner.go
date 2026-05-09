package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"wirety/agent/internal/adapters/captiveportal"
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
	wgIP              string // WireGuard interface IP of this peer
	currentPeerName   string // Track current peer name to detect changes
	peerID            string // for audit logging
	networkID         string // for audit logging
	// peerNames maps WireGuard public key → peer name (updated on each WSMessage).
	peerNames   map[string]string
	peerNamesMu sync.RWMutex
	backoffBase       time.Duration
	backoffMax        time.Duration
	heartbeatInterval time.Duration
	// Captive portal HTTP server (jump peer only)
	serverURL        string
	authToken        string
	captivePortalURL string
	captiveStarted   bool
	httpClient       *http.Client // shared client (may override Host header)
	vpnDomain        string       // VPN DNS domain (e.g. "wg.example.com"); used for TLS SAN
	// whitelist maps authenticated peer WireGuard IPs to the public endpoint IP
	// that was recorded at authentication time (empty string = no endpoint check,
	// used for legacy entries or when the jump peer could not resolve the endpoint).
	// Kept in sync with every firewall policy update so the captive portal server
	// can return OS probe success responses for already-authenticated peers.
	whitelist   map[string]string
	whitelistMu sync.RWMutex
	// ipv4ToIPv6 maps each peer's IPv4 WireGuard address to its IPv6 WireGuard address.
	// Built from DNS peer config and used to extend the ip6tables whitelist when an
	// IPv4 address is authenticated via the captive portal.
	ipv4ToIPv6   map[string]string
	ipv4ToIPv6Mu sync.RWMutex
	// wgIPToEndpoint maps each peer's WireGuard private IP to its current public
	// endpoint as reported by `wg show endpoints` ("ip:port", no stripping).
	// Refreshed every 300 ms by the heartbeat goroutine so that isAuthenticated
	// can compare stored vs. live endpoint without spawning a wg command on
	// every HTTP request.
	wgIPToEndpoint   map[string]string
	wgIPToEndpointMu sync.RWMutex
	// captivePortalSrv is the running captive portal HTTP server (jump peer only).
	// Set once by startCaptivePortalServer; protected by captivePortalSrvMu.
	captivePortalSrv   *captiveportal.Server
	captivePortalSrvMu sync.Mutex
	// ifaceMu protects wgInterface which can be updated by handlePeerNameChange
	// while being read concurrently by the heartbeat and tunnel-monitor goroutines.
	ifaceMu sync.RWMutex
	// lastPolicy / lastWhitelistRaw / lastFwState cache the most recent policy
	// and whitelist so resyncFirewall can re-apply iptables when a whitelisted
	// peer's public endpoint changes (without waiting for the next server push).
	// lastWhitelistRaw holds entries in "wgIP@endpointIP" format; lastFwState
	// holds the wgIPs that survived the live endpoint check on the last sync,
	// so we can short-circuit the iptables call when nothing has changed.
	lastPolicy       *pol.JumpPolicy
	lastWhitelistRaw []string
	lastFwState      []string
	lastSyncMu       sync.Mutex
}

func NewRunner(wsClient ports.WebSocketClientPort, writer ports.ConfigWriterPort, dnsServer ports.DNSStarterPort, fwAdapter ports.FirewallPort, wsURL string, wgInterface string, peerID string, networkID string) *Runner {
	return &Runner{
		wsClient:          wsClient,
		cfgWriter:         writer,
		dnsServer:         dnsServer,
		fwAdapter:         fwAdapter,
		wsURL:             wsURL,
		wgInterface:       wgInterface,
		currentPeerName:   "",
		peerID:            peerID,
		networkID:         networkID,
		peerNames:         make(map[string]string),
		whitelist:         make(map[string]string),
		ipv4ToIPv6:        make(map[string]string),
		wgIPToEndpoint:    make(map[string]string),
		backoffBase:       time.Second,
		backoffMax:        30 * time.Second,
		heartbeatInterval: 30 * time.Second,
	}
}

// SetWGIP sets the WireGuard interface IP of this peer. Used to bind the captive
// portal HTTP server and to configure DNS probe interception.
func (r *Runner) SetWGIP(ip string) {
	r.wgIP = ip
}

// isAuthenticated reports whether the given peer WireGuard IP has completed
// captive portal authentication AND is connecting from the same public endpoint
// IP that was recorded at authentication time.
//
// If no endpoint IP was stored (empty string — legacy entry or SSO disabled),
// only the whitelist membership is checked for backward compatibility.
func (r *Runner) isAuthenticated(peerIP string) bool {
	r.whitelistMu.RLock()
	expectedEndpoint, ok := r.whitelist[peerIP]
	r.whitelistMu.RUnlock()
	if !ok {
		return false
	}
	if expectedEndpoint == "" {
		// No endpoint constraint stored — legacy / non-SSO entry.
		return true
	}
	// Verify the peer's current public endpoint matches the one stored at
	// authentication time.  A mismatch means the WireGuard config is being
	// used from a different network — require re-authentication.
	currentEndpoint := r.getCurrentEndpointForWgIP(peerIP)
	return currentEndpoint != "" && currentEndpoint == expectedEndpoint
}

// getCurrentEndpointForWgIP returns the most-recently-observed public endpoint
// endpoint ("ip:port", strict) for the given WireGuard private IP, or "" if unknown.
func (r *Runner) getCurrentEndpointForWgIP(wgIP string) string {
	r.wgIPToEndpointMu.RLock()
	defer r.wgIPToEndpointMu.RUnlock()
	return r.wgIPToEndpoint[wgIP]
}

// updateWGIPEndpointMap rebuilds the wgIP → public endpoint ("ip:port") lookup
// table from the current WireGuard state.  Called every 300 ms by the heartbeat
// goroutine so isAuthenticated can do a cheap map lookup without shelling out.
//
// We deliberately keep the port: any change to the public source port (e.g. a
// fresh WireGuard handshake after a tunnel restart, or NAT rebinding) counts
// as an endpoint change and forces re-authentication via the captive portal.
// This is the strictest possible check.
func (r *Runner) updateWGIPEndpointMap() {
	iface := r.getInterface()
	allowedIPs := GetWireGuardAllowedIPs(iface) // pubkey → []CIDR
	endpoints := getWireGuardEndpoints(iface)    // pubkey → "ip:port"

	m := make(map[string]string, len(allowedIPs))
	for pubkey, cidrs := range allowedIPs {
		ep, ok := endpoints[pubkey]
		if !ok || ep == "" || ep == "(none)" {
			continue
		}
		for _, cidr := range cidrs {
			// Only map host routes (/32 for IPv4, /128 for IPv6) to avoid
			// mapping summary routes (e.g. 0.0.0.0/0) to an endpoint.
			if !strings.HasSuffix(cidr, "/32") && !strings.HasSuffix(cidr, "/128") {
				continue
			}
			ip := cidr[:strings.IndexByte(cidr, '/')]
			if ip != "" {
				m[ip] = ep // full "ip:port" — strict match
			}
		}
	}

	r.wgIPToEndpointMu.Lock()
	r.wgIPToEndpoint = m
	r.wgIPToEndpointMu.Unlock()
}

// updateIPv4ToIPv6Map rebuilds the IPv4 WireGuard IP → IPv6 WireGuard IP lookup
// from the DNS peer list. Called whenever a DNS config update is received.
func (r *Runner) updateIPv4ToIPv6Map(peers []dom.DNSPeer) {
	m := make(map[string]string, len(peers))
	for _, p := range peers {
		if p.IP != "" && p.IPv6 != "" {
			m[p.IP] = p.IPv6
		}
	}
	r.ipv4ToIPv6Mu.Lock()
	r.ipv4ToIPv6 = m
	r.ipv4ToIPv6Mu.Unlock()
}

// extendWhitelistWithIPv6 appends IPv6 WireGuard addresses for each whitelisted
// IPv4 address. This is how authenticated peers get their IPv6 address whitelisted
// in ip6tables without requiring a separate captive portal authentication flow.
func (r *Runner) extendWhitelistWithIPv6(wgIPs []string) []string {
	r.ipv4ToIPv6Mu.RLock()
	mapping := r.ipv4ToIPv6
	r.ipv4ToIPv6Mu.RUnlock()
	if len(mapping) == 0 {
		return wgIPs
	}
	out := make([]string, len(wgIPs), len(wgIPs)+len(mapping))
	copy(out, wgIPs)
	for _, ipv4 := range wgIPs {
		if ipv6, ok := mapping[ipv4]; ok {
			out = append(out, ipv6)
		}
	}
	return out
}

// updateWhitelist replaces the in-memory whitelist with the given entries.
// Each entry is either "wgIP@endpointIP" (endpoint-checked) or plain "wgIP"
// (legacy / no endpoint check).
func (r *Runner) updateWhitelist(ips []string) {
	r.whitelistMu.Lock()
	defer r.whitelistMu.Unlock()
	r.whitelist = make(map[string]string, len(ips))
	for _, entry := range ips {
		if idx := strings.IndexByte(entry, '@'); idx != -1 {
			r.whitelist[entry[:idx]] = entry[idx+1:]
		} else {
			r.whitelist[entry] = "" // no endpoint constraint
		}
	}
}

// filterWhitelistByEndpoint returns the wgIPs whose live public endpoint matches
// the endpoint stored at authentication time.  Entries without a stored endpoint
// (legacy / non-SSO) are passed through unchanged.  Entries whose live endpoint
// is unknown OR differs from the stored endpoint are dropped — this is what
// causes the firewall to revoke access when a stolen WireGuard config tries to
// connect from a different network.
func (r *Runner) filterWhitelistByEndpoint(entries []string) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		wgIP, expectedEP := entry, ""
		if idx := strings.IndexByte(entry, '@'); idx != -1 {
			wgIP = entry[:idx]
			expectedEP = entry[idx+1:]
		}
		if expectedEP == "" {
			out = append(out, wgIP) // legacy entry — no constraint
			continue
		}
		if currentEP := r.getCurrentEndpointForWgIP(wgIP); currentEP != "" && currentEP == expectedEP {
			out = append(out, wgIP)
		}
		// else: drop — endpoint mismatch or unknown
	}
	return out
}

// resyncFirewall re-applies the most recent policy with a freshly-filtered
// whitelist.  Called from the 300 ms ticker so an endpoint change for a
// whitelisted peer immediately revokes its iptables ACCEPT rules without waiting
// for the next server push.  No-op if nothing has changed (cheap to call often).
func (r *Runner) resyncFirewall() {
	if r.fwAdapter == nil {
		return
	}
	r.lastSyncMu.Lock()
	policy := r.lastPolicy
	whitelist := r.lastWhitelistRaw
	r.lastSyncMu.Unlock()
	if policy == nil {
		return
	}

	filtered := r.filterWhitelistByEndpoint(whitelist)
	filteredWithIPv6 := r.extendWhitelistWithIPv6(filtered)

	r.lastSyncMu.Lock()
	if stringSliceEqual(r.lastFwState, filtered) {
		r.lastSyncMu.Unlock()
		return // no change since last sync
	}
	r.lastFwState = filtered
	r.lastSyncMu.Unlock()

	if err := r.fwAdapter.Sync(policy, policy.IP, filteredWithIPv6); err != nil {
		log.Error().Err(err).Msg("firewall re-sync after endpoint change failed")
	} else {
		log.Info().Strs("whitelist", filtered).Msg("firewall re-synced after endpoint change")
	}
}

// stringSliceEqual reports whether two whitelist slices contain the same set of
// IPs (order-insensitive — the firewall is keyed on set membership).
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]struct{}, len(a))
	for _, x := range a {
		seen[x] = struct{}{}
	}
	for _, x := range b {
		if _, ok := seen[x]; !ok {
			return false
		}
	}
	return true
}

// SetHeaders sets HTTP headers to send on WebSocket connection (e.g. Authorization).
func (r *Runner) SetHeaders(header http.Header) {
	r.wsHeaders = header
}

// SetCaptivePortal configures the captive portal redirect server for jump peers.
// serverURL: Wirety server HTTP URL (e.g. "https://wirety.example.com")
// authToken: enrollment token used to call the captive-portal/token API
// captivePortalURL: full URL of the captive portal page the peer will be redirected to
// httpClient: shared HTTP client (may carry a Host-override transport for reverse-proxy setups)
func (r *Runner) SetCaptivePortal(serverURL, authToken, captivePortalURL string, httpClient *http.Client) {
	r.serverURL = serverURL
	r.authToken = authToken
	r.captivePortalURL = captivePortalURL
	r.httpClient = httpClient
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

		// Reset the in-memory whitelist and the policy-received flag on every new
		// WebSocket connection. The server will push the current state in the first
		// policy message. Without this, a stale whitelist from a previous connection
		// would cause the captive portal server to treat formerly-authenticated peers
		// as authenticated, serving OS probe success responses instead of redirecting
		// them to the portal page.
		r.updateWhitelist([]string{})
		r.captivePortalSrvMu.Lock()
		if r.captivePortalSrv != nil {
			r.captivePortalSrv.ResetPolicyReceived()
		}
		r.captivePortalSrvMu.Unlock()

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
					sysInfo, err := CollectSystemInfo(r.getInterface())
					if err == nil {
						lastPeerEndpointsMu.Lock()
						lastPeerEndpoints = sysInfo.PeerEndpoints
						lastPeerEndpointsMu.Unlock()
					}
				case <-endpointCheckTicker.C:
					// Check for endpoint changes every 300ms
					sysInfo, err := CollectSystemInfo(r.getInterface())
					if err != nil {
						continue
					}

					// Keep the wgIP → public endpoint IP cache fresh so that
					// isAuthenticated can verify endpoints without shelling out.
					r.updateWGIPEndpointMap()

					// Re-apply iptables if any whitelisted peer's live endpoint
					// no longer matches its stored endpoint.  This is what
					// revokes firewall ACCEPT for a stolen config trying to
					// reach the network from a different public IP.
					r.resyncFirewall()

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

			// Update pubkey → name map and IPv4→IPv6 address mapping from DNS peers.
			if payload.DNS != nil {
				r.updatePeerNames(payload.Config, payload.DNS.Peers)
				r.updateIPv4ToIPv6Map(payload.DNS.Peers)
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
				// Keep vpnDomain in sync for the HTTPS captive portal TLS cert SAN.
				if payload.DNS.Domain != "" {
					r.vpnDomain = payload.DNS.Domain
				}
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

				// Whitelist entries arrive in "wgIP@endpointIP" format (or plain
				// "wgIP" for legacy / non-SSO entries).
				whitelistedIPs := payload.Whitelist
				if whitelistedIPs == nil {
					whitelistedIPs = []string{}
				}

				// Keep in-memory whitelist in sync so the captive portal HTTP server
				// can return OS probe success responses for authenticated peers.
				r.updateWhitelist(whitelistedIPs)

				// Cache the policy + raw whitelist so resyncFirewall() (called from
				// the 300 ms ticker) can re-apply iptables when a peer's endpoint
				// changes between server pushes.
				r.lastSyncMu.Lock()
				r.lastPolicy = payload.Policy
				r.lastWhitelistRaw = whitelistedIPs
				r.lastSyncMu.Unlock()

				// Notify the captive portal server that at least one policy has been
				// received. Until this point the server treats all peers as
				// unauthenticated to avoid serving stale whitelist data.
				r.captivePortalSrvMu.Lock()
				cpSrv := r.captivePortalSrv
				r.captivePortalSrvMu.Unlock()
				if cpSrv != nil {
					cpSrv.NotifyPolicyReceived()
				}

				// Filter the whitelist by live endpoint match before handing it to
				// the firewall adapter.  A wgIP whose current public endpoint does
				// not match the IP recorded at authentication time (i.e. stolen
				// config from a different network) is dropped from iptables, so
				// non-HTTP traffic is also denied — not just captive-portal HTTP.
				filtered := r.filterWhitelistByEndpoint(whitelistedIPs)
				// For dual-stack networks, also whitelist each peer's IPv6 WireGuard
				// address when their IPv4 is authenticated. This avoids a separate
				// captive portal flow for IPv6 traffic from the same peer.
				filteredWithIPv6 := r.extendWhitelistWithIPv6(filtered)
				r.lastSyncMu.Lock()
				r.lastFwState = filtered
				r.lastSyncMu.Unlock()

				if err := r.fwAdapter.Sync(payload.Policy, payload.Policy.IP, filteredWithIPv6); err != nil {
					log.Error().Err(err).Msg("failed applying firewall policy update")
				} else {
					log.Info().
						Int("iptables_rule_count", len(payload.Policy.IPTablesRules)).
						Int("whitelisted_peers", len(filtered)).
						Msg("firewall policy update applied successfully")
					audit.Agent(r.peerID, r.networkID).
						Str("action", "firewall.sync").
						Int("rule_count", len(payload.Policy.IPTablesRules)).
						Msg("audit")
				}

				// Start the captive portal redirect server the first time we get a
				// policy — that's when we know this peer is a jump peer.
				if !r.captiveStarted && r.serverURL != "" {
					r.captiveStarted = true
					go r.startCaptivePortalServer()
				}
			}
		}
	}
}

// sendHeartbeat sends system information to the server
func (r *Runner) sendHeartbeat() {
	sysInfo, err := CollectSystemInfo(r.getInterface())
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

// getInterface returns the current WireGuard interface name safely.
func (r *Runner) getInterface() string {
	r.ifaceMu.RLock()
	defer r.ifaceMu.RUnlock()
	return r.wgInterface
}

// setInterface updates the WireGuard interface name safely.
func (r *Runner) setInterface(iface string) {
	r.ifaceMu.Lock()
	defer r.ifaceMu.Unlock()
	r.wgInterface = iface
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
	r.setInterface(newInterface)

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

		handshakes := GetWireGuardHandshakes(r.getInterface())
		endpoints := getWireGuardEndpoints(r.getInterface())
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

// captivePortalExcludedHosts returns the set of hostnames that must not be
// redirected to the captive portal IP for unauthenticated peers. This includes
// the Wirety server hostname and the captive portal page hostname. Without these
// exclusions the redirect target URL itself would resolve to the captive portal
// IP and cause an infinite redirect loop.
func (r *Runner) captivePortalExcludedHosts() []string {
	seen := make(map[string]struct{})
	var hosts []string
	for _, rawURL := range []string{r.serverURL, r.captivePortalURL} {
		if rawURL == "" {
			continue
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		h := u.Hostname()
		if h == "" {
			continue
		}
		if _, ok := seen[h]; !ok {
			seen[h] = struct{}{}
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// startCaptivePortalServer starts the HTTP server that handles captive portal
// authentication. It listens directly on the WireGuard interface IP:80, so no
// DNAT rule is required. Unauthenticated peers are redirected to the authentication
// page; authenticated peers receive OS-specific probe success responses.
func (r *Runner) startCaptivePortalServer() {
	srv := captiveportal.NewServer(r.serverURL, r.authToken, r.captivePortalURL, r.networkID, r.peerID, r.httpClient)
	srv.SetAuthChecker(r.isAuthenticated)

	// Store the reference so the policy-sync path can call NotifyPolicyReceived.
	r.captivePortalSrvMu.Lock()
	r.captivePortalSrv = srv
	r.captivePortalSrvMu.Unlock()

	// Configure DNS:
	//  - Probe domains (captive.apple.com, connectivitycheck.gstatic.com, …) always
	//    resolve to the WG IP so OS captive portal detection fires through the tunnel.
	//  - Internal VPN domain queries from unauthenticated peers also resolve to the
	//    WG IP, so any attempt to reach a private resource triggers the redirect.
	//    External DNS is untouched — internet keeps working while unauthenticated.
	if r.wgIP != "" {
		type dnsPortalConfigurer interface {
			SetCaptivePortalIP(string)
			SetAuthChecker(func(string) bool)
			SetRedirectExclusions([]string)
		}
		if dns, ok := r.dnsServer.(dnsPortalConfigurer); ok {
			dns.SetCaptivePortalIP(r.wgIP)
			dns.SetAuthChecker(r.isAuthenticated)
			dns.SetRedirectExclusions(r.captivePortalExcludedHosts())
		}

		// Wire up peer-IP lookup so the captive portal can proxy authenticated
		// peers directly to the real backend while the browser's DNS cache is
		// stale (Firefox ignores TTL=1 and keeps entries for up to 60 s, causing
		// an infinite "Connecting…" loop without this proxy).
		type dnsPeerLookup interface {
			LookupPeerIP(host string) string
		}
		if lookup, ok := r.dnsServer.(dnsPeerLookup); ok {
			srv.SetPeerIPLookup(func(host string) string {
				return lookup.LookupPeerIP(host)
			})
		}

		// Wire up endpoint lookup so the captive portal server can include the
		// peer's full current public endpoint ("ip:port") in the token request.
		// The server stores this alongside the WireGuard IP; the jump peer
		// later checks that a peer trying to reach the network is still
		// connecting from the same source IP+port — any change (NAT rebind,
		// tunnel restart, different network) forces re-authentication.
		srv.SetEndpointLookup(r.getCurrentEndpointForWgIP)
	}

	// Start the HTTPS server in a background goroutine. It uses a self-signed cert
	// covering the WG IP and a wildcard for the VPN domain so that internal peer
	// hostnames match the cert. Internal VPN domains are not HSTS-preloaded, so
	// browsers allow the user to bypass the self-signed warning and follow the
	// redirect — unlike public domains (google.com, etc.) which are hard-blocked.
	go func() {
		tlsAddr := r.wgIP + ":443"
		if err := srv.StartTLS(tlsAddr, r.wgIP, r.vpnDomain); err != nil {
			log.Error().Err(err).Msg("captive portal HTTPS server stopped")
		}
	}()

	addr := r.wgIP + ":80"
	log.Info().Str("addr", addr).Str("portal_url", r.captivePortalURL).Msg("starting captive portal HTTP server")
	if err := srv.Start(addr); err != nil {
		log.Error().Err(err).Msg("captive portal HTTP server stopped")
	}
}
