package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
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
	Whitelist   []string        `json:"whitelist,omitempty"`    // authenticated peers ("wgIP@endpoint" format)
	OAuthIssuer string          `json:"oauth_issuer,omitempty"` // OAuth issuer URL for TLS-SNI gateway

	// New fields (3-tier captive portal gate, jump peer only):
	PendingAuth      []PendingAuthEntry      `json:"pending_auth,omitempty"`
	EndpointDenylist []EndpointDenylistEntry `json:"endpoint_denylist,omitempty"`
	Quarantined      []string                `json:"quarantined,omitempty"`
	PeerRoutes       map[string][]string     `json:"peer_routes,omitempty"` // wgIP -> AllowedIPs
}

// PendingAuthEntry mirrors the server-side type: a peer that has been issued a
// captive portal token (active OIDC flow) but has not yet completed SSO.  The
// jump peer adds a temporary HTTPS-only iptables ACCEPT rule for these wgIPs
// so the OIDC redirect chain can reach external providers.
type PendingAuthEntry struct {
	WgIP      string `json:"wg_ip"`
	Endpoint  string `json:"endpoint,omitempty"`
	ExpiresAt string `json:"expires_at"`
}

// EndpointDenylistEntry is a rogue WireGuard UDP source that the jump peer
// must drop at the physical interface to prevent it from completing further
// WireGuard handshakes for any peer in the network.
type EndpointDenylistEntry struct {
	BlockedIP   string `json:"blocked_ip"`
	BlockedPort int    `json:"blocked_port"`
}

// tunnelActive is the threshold below which a peer is considered connected.
// WireGuard sends a keepalive every 25s and considers a peer inactive after ~180s.
const tunnelActiveThreshold = 180 * time.Second

// tunnelPollInterval is how often the runner checks for handshake changes.
const tunnelPollInterval = 15 * time.Second

// endpointStabilityWindow is how long a peer's WireGuard endpoint must remain
// unchanged before it is (re-)added to the iptables whitelist after a change.
//
// When two devices share the same WireGuard private key (config sharing / theft),
// they compete for the same WireGuard peer slot on the jump peer.  Each device's
// keepalive overrides the peer's endpoint in `wg show`, causing the endpoint to
// oscillate between the two public IP:port combinations every ~25 s.  Without this
// stability check, the legitimate device could regain iptables access during the
// brief window when `wg show` happens to report its endpoint — giving both devices
// intermittent access.
//
// With the stability window:
//   • When an endpoint changes, a timer starts.
//   • Even if the current endpoint matches the stored (authenticated) one, the
//     peer is excluded from the iptables whitelist until the endpoint has been
//     stable for endpointStabilityWindow seconds.
//   • Two competing devices therefore get NO iptables access during the oscillation
//     phase, forcing both to re-authenticate via the captive portal.
const endpointStabilityWindow = 10 * time.Second

type Runner struct {
	wsClient          ports.WebSocketClientPort
	cfgWriter         ports.ConfigWriterPort
	dnsServer         ports.DNSStarterPort // active DNS server instance
	dnsServerMu       sync.Mutex           // protects dnsServer
	fwAdapter         ports.FirewallPort
	wsURL             string
	wsHeaders         http.Header
	wgInterface       string
	wgIP              string // WireGuard interface IPv4 of this peer
	wgIPv6            string // WireGuard interface IPv6 of this peer (optional, dual-stack)
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
	// endpointChangedAt records the last time each peer's WireGuard endpoint
	// changed (keyed by WireGuard private IP).  Used by filterWhitelistByEndpoint
	// to enforce the endpointStabilityWindow: a peer is not re-added to the
	// iptables whitelist until its endpoint has been stable long enough.  This
	// prevents two devices sharing the same WireGuard config from getting
	// intermittent access by oscillating the WireGuard endpoint between their
	// respective public IP:port combinations.
	endpointChangedAt   map[string]time.Time
	endpointChangedMu   sync.RWMutex
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
	// Latest pending-auth / quarantine / denylist state from the server (jump
	// peer only).  Reapplied alongside the whitelist by resyncFirewall().
	lastPendingAuthIPs   []string
	lastQuarantinedIPs   []string
	lastEndpointDenylist []ports.DenylistEntry
	lastWGListenPort     int

	// Pending takeover reports — populated by detectEndpointTakeovers() when
	// an authenticated peer's WireGuard endpoint flips to a foreign source.
	// Drained on the next heartbeat into AgentHeartbeat.EndpointTakeovers.
	pendingTakeovers   []endpointTakeoverReport
	pendingTakeoversMu sync.Mutex
	// reportedTakeovers prevents the same (wgIP, observedEndpoint) pair from
	// being reported on every poll cycle.  Cleared when the endpoint changes
	// to something else, or after takeoverReportCooldown.
	reportedTakeovers   map[string]time.Time // key = wgIP+"|"+observedEP
	reportedTakeoversMu sync.Mutex
	// takeoverFlips tracks per-peer endpoint history to distinguish a single
	// legitimate endpoint change (NAT rebind / roam) from an oscillating
	// takeover (two devices competing for the same WireGuard peer slot).
	// See takeoverFlipState comment for details.
	takeoverFlips   map[string]*takeoverFlipState
	takeoverFlipsMu sync.Mutex
	// localAllowedIPs is THIS peer's WireGuard AllowedIPs as parsed from the
	// applied wg config.  Reported in every heartbeat so the jump peer's DNS
	// server can decide whether to redirect external queries from this peer.
	localAllowedIPs   []string
	localAllowedIPsMu sync.RWMutex
}

// endpointTakeoverReport is the agent-internal mirror of
// network.EndpointTakeoverReport (server domain).  Kept lightweight to avoid
// cross-package import.
type endpointTakeoverReport struct {
	WgIP            string
	AuthenticatedAt string
	ObservedAt      string
}

// takeoverReportCooldown prevents flooding the server with duplicate takeover
// reports for the same rogue source within a short window.  300 ms ticker × 60
// = 18 s of spam suppression for the same observation; after that we re-arm.
const takeoverReportCooldown = 60 * time.Second

// flipDetectionWindow is how long the per-peer flip counter is retained.
// Flips older than this are forgotten — a single endpoint change followed by
// stability is treated as a legitimate roam (NAT rebind, network handover,
// fresh tunnel), not as a takeover.
const flipDetectionWindow = 60 * time.Second

// flipsRequiredForDenylist is how many independent "flip TO foreign" events
// the agent must observe within flipDetectionWindow before denylisting the
// foreign source.  A single flip is indistinguishable from a NAT rebind, so
// we never denylist on the first one — we wait for the second.  Two flips
// implies the endpoint has bounced back to the authenticated value at least
// once in between, which is the signature of two devices competing.
const flipsRequiredForDenylist = 2

// takeoverFlipState tracks per-peer endpoint history so the agent can
// distinguish a single legitimate endpoint change from an oscillating
// takeover.  Conceptually:
//
//   • lastWasStored — was the most recent observation the authenticated
//                     endpoint?  Used to detect transitions FROM stored
//                     TO foreign (which is what we count).
//   • flipsToForeign — count of stored→foreign transitions inside the
//                      detection window.  ≥ flipsRequiredForDenylist means
//                      the endpoint has bounced back at least once: an
//                      unambiguous signature of two simultaneously-active
//                      devices.
//   • firstFlipAt   — timestamp of the first counted flip; the counter
//                     resets if we go past flipDetectionWindow without
//                     reaching the threshold.
//   • lastForeignEP — the most recent foreign endpoint (this is the one we
//                     denylist when the threshold trips).
type takeoverFlipState struct {
	lastWasStored  bool
	flipsToForeign int
	firstFlipAt    time.Time
	lastForeignEP  string
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
		endpointChangedAt: make(map[string]time.Time),
		reportedTakeovers: make(map[string]time.Time),
		takeoverFlips:     make(map[string]*takeoverFlipState),
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

// SetWGIPv6 sets the WireGuard interface IPv6 address of this peer (when the
// network is dual-stack).  Captive portal HTTP/HTTPS listeners are spawned for
// both families so IPv6 peers can reach the portal too.
func (r *Runner) SetWGIPv6(ip string) {
	r.wgIPv6 = ip
}

// extractEndpointIP returns the host portion of an "ip:port" or "[ipv6]:port"
// endpoint string, or the input unchanged if it doesn't parse.  Used to compare
// peer endpoints at IP granularity only — NAT port rebinds shouldn't kick a
// legitimate peer out of the whitelist, while a stolen config used from a
// genuinely different network (different IP) still fails the check.
func extractEndpointIP(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return endpoint
	}
	return host
}

// isAuthenticated reports whether the given peer WireGuard IP has completed
// captive portal authentication AND is connecting from the same public endpoint
// IP that was recorded at authentication time.
//
// Comparison is at IP granularity, NOT IP:port.  A stolen WireGuard config used
// from a different network has a different public IP and is rejected; a
// legitimate peer whose NAT rebinds its source port (changing IP:port without
// changing the IP) keeps its whitelist entry.  Oscillation between two devices
// sharing the same key is handled separately by the takeover-detection denylist.
//
// If no endpoint constraint was stored (empty string — legacy entry or SSO
// disabled), only whitelist membership is checked.
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
	currentEndpoint := r.getCurrentEndpointForWgIP(peerIP)
	if currentEndpoint == "" {
		return false
	}
	expectedIP := extractEndpointIP(expectedEndpoint)
	currentIP := extractEndpointIP(currentEndpoint)
	return expectedIP != "" && expectedIP == currentIP
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
// as an endpoint change and triggers the endpointStabilityWindow before the peer
// is re-admitted to the iptables whitelist.  This prevents two devices sharing
// the same WireGuard private key from getting intermittent access by oscillating
// the jump-peer's recorded endpoint between their two public IP:port pairs.
func (r *Runner) updateWGIPEndpointMap() {
	iface := r.getInterface()
	allowedIPs := GetWireGuardAllowedIPs(iface) // pubkey → []CIDR
	endpoints := getWireGuardEndpoints(iface)    // pubkey → "ip:port"

	newMap := make(map[string]string, len(allowedIPs))
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
				newMap[ip] = ep // full "ip:port" — strict match
			}
		}
	}

	// Snapshot the current map before replacing it so we can detect changes.
	r.wgIPToEndpointMu.RLock()
	oldMap := r.wgIPToEndpoint
	r.wgIPToEndpointMu.RUnlock()

	// Record the timestamp for any WireGuard IP whose endpoint just changed.
	// This is used by filterWhitelistByEndpoint to enforce the stability window.
	now := time.Now()
	r.endpointChangedMu.Lock()
	for ip, newEP := range newMap {
		if oldEP, existed := oldMap[ip]; existed && oldEP != newEP {
			r.endpointChangedAt[ip] = now
			log.Warn().
				Str("wg_ip", ip).
				Str("old_endpoint", oldEP).
				Str("new_endpoint", newEP).
				Dur("stability_window", endpointStabilityWindow).
				Msg("WireGuard endpoint changed — stability window started; peer temporarily removed from iptables whitelist")

			// If this wgIP is currently authenticated AND the new endpoint is
			// NOT the one recorded at authentication time, this is a "takeover"
			// — a rogue source completing WireGuard handshakes for a peer that
			// has already authenticated through someone else.  Queue a report
			// so the next heartbeat asks the server to denylist the rogue source.
			r.queueTakeoverIfRogue(ip, newEP)
		}
	}
	r.endpointChangedMu.Unlock()

	r.wgIPToEndpointMu.Lock()
	r.wgIPToEndpoint = newMap
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

// filterWhitelistByEndpoint returns the wgIPs whose live public endpoint IP
// matches the endpoint IP stored at authentication time AND has been stable
// long enough.
//
// Two checks are applied in order:
//  1. Endpoint-IP match — the IP portion of the current endpoint (from
//     `wg show`) must equal the IP portion of the endpoint recorded at
//     authentication time.  A mismatch means the WireGuard config is being
//     used from a different network (e.g. stolen config).  The port is NOT
//     compared: NAT rebinds change the source port frequently and a legitimate
//     peer would otherwise lose its whitelist entry on every reconnect.
//  2. Stability window — even when the IP matches, the peer is excluded if
//     its endpoint (IP or port) changed within the last endpointStabilityWindow.
//     This prevents two devices sharing the same WireGuard private key from
//     getting intermittent iptables access: while they oscillate the jump
//     peer's recorded endpoint every ~25 s (WireGuard keepalive interval),
//     neither has a stable endpoint and neither is whitelisted.  Once one
//     device "wins" for endpointStabilityWindow, it gets back in.
//
// Oscillation between two devices behind the same NAT (which would share the
// same source IP and only differ in port) is caught by the takeover-detection
// denylist instead — see queueTakeoverIfRogue.
//
// Entries without a stored endpoint (legacy / non-SSO) are passed through unchanged.
func (r *Runner) filterWhitelistByEndpoint(entries []string) []string {
	out := make([]string, 0, len(entries))
	now := time.Now()
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

		// Check 1: endpoint IP must match.
		currentEP := r.getCurrentEndpointForWgIP(wgIP)
		if currentEP == "" {
			continue
		}
		expectedIP := extractEndpointIP(expectedEP)
		currentIP := extractEndpointIP(currentEP)
		if expectedIP == "" || expectedIP != currentIP {
			// IP mismatch — config is being used from a different network.
			continue
		}

		// Check 2: endpoint must have been stable for at least endpointStabilityWindow.
		r.endpointChangedMu.RLock()
		changedAt, hasRecentChange := r.endpointChangedAt[wgIP]
		r.endpointChangedMu.RUnlock()
		if hasRecentChange && now.Sub(changedAt) < endpointStabilityWindow {
			log.Debug().
				Str("wg_ip", wgIP).
				Str("endpoint", currentEP).
				Dur("stable_for", now.Sub(changedAt)).
				Dur("required", endpointStabilityWindow).
				Msg("endpoint recently changed — holding peer out of iptables whitelist until stable")
			continue
		}

		out = append(out, wgIP)
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
	pendingAuth := append([]string(nil), r.lastPendingAuthIPs...)
	quarantined := append([]string(nil), r.lastQuarantinedIPs...)
	denylist := append([]ports.DenylistEntry(nil), r.lastEndpointDenylist...)
	wgListenPort := r.lastWGListenPort
	r.lastSyncMu.Unlock()
	if policy == nil {
		return
	}

	filtered := r.filterWhitelistByEndpoint(whitelist)
	filteredWithIPv6 := r.extendWhitelistWithIPv6(filtered)
	pendingWithIPv6 := r.extendWhitelistWithIPv6(pendingAuth)

	r.lastSyncMu.Lock()
	if stringSliceEqual(r.lastFwState, filtered) {
		r.lastSyncMu.Unlock()
		return // no change since last sync
	}
	r.lastFwState = filtered
	r.lastSyncMu.Unlock()

	if err := r.fwAdapter.Sync(ports.SyncRequest{
		Policy:              policy,
		SelfIP:              policy.IP,
		AuthenticatedIPs:    filteredWithIPv6,
		PendingAuthIPs:      pendingWithIPv6,
		QuarantinedIPs:      quarantined,
		EndpointDenylist:    denylist,
		WireGuardListenPort: wgListenPort,
	}); err != nil {
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

		// Send an immediate client→server frame the moment the upgrade is up.
		// Some reverse proxies / load balancers (notably Scaleway's managed LB)
		// apply a short "post-response client-idle" timer to upgraded HTTP/1.1
		// connections and tear the TCP socket down within a few seconds when
		// the client stays silent.  Without this the first heartbeat is 30 s
		// away and the agent's connection dies inside that window.  A protocol
		// Ping is enough — the server's default PingHandler responds with Pong
		// transparently, so there's no application-level cost.
		if err := r.wsClient.Ping(); err != nil {
			log.Warn().Err(err).Msg("initial websocket ping failed")
		}

		// Start heartbeat goroutine with endpoint change detection
		heartbeatTicker := time.NewTicker(r.heartbeatInterval)
		defer heartbeatTicker.Stop()
		endpointCheckTicker := time.NewTicker(300 * time.Millisecond)
		defer endpointCheckTicker.Stop()
		// Keepalive ping at a much shorter cadence than the heartbeat — its only
		// job is to keep traffic visible to any stateful intermediary that
		// would otherwise close an "idle" upgraded connection.  5 s is well
		// inside the typical 10–30 s LB defaults while staying lightweight
		// (a Ping frame is 6 bytes on the wire).
		keepalivePingTicker := time.NewTicker(5 * time.Second)
		defer keepalivePingTicker.Stop()
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
				case <-keepalivePingTicker.C:
					// Lightweight WS protocol Ping — gorilla auto-responds
					// with Pong server-side.  Lives in the same goroutine as
					// the heartbeat to keep "one writer at a time" semantics
					// (gorilla disallows concurrent writers on a connection).
					if err := r.wsClient.Ping(); err != nil {
						log.Debug().Err(err).Msg("keepalive ping failed (will retry)")
					}
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
				// Refresh the local AllowedIPs cache so the next heartbeat
				// reports them to the server (used by the jump peer's DNS to
				// decide route-aware whether to redirect external queries from
				// this peer when it is unauthenticated).
				r.SetLocalAllowedIPs(parseLocalAllowedIPsFromConfig(payload.Config))
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
					Int("pending_auth", len(payload.PendingAuth)).
					Int("denylist", len(payload.EndpointDenylist)).
					Int("quarantined", len(payload.Quarantined)).
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

				// Translate pending-auth, denylist, and quarantine into agent-side
				// firewall types.  Pending-auth wgIPs are filtered the same way as
				// the whitelist — IP-only match (NAT port rebinds OK, different
				// IP rejected) so a stolen config from a different network can't
				// hold a pending-auth grant either.
				pendingWgIPs := make([]string, 0, len(payload.PendingAuth))
				for _, e := range payload.PendingAuth {
					if e.Endpoint != "" {
						currentEP := r.getCurrentEndpointForWgIP(e.WgIP)
						if currentEP == "" {
							continue
						}
						if extractEndpointIP(currentEP) != extractEndpointIP(e.Endpoint) {
							continue
						}
					}
					pendingWgIPs = append(pendingWgIPs, e.WgIP)
				}
				denylistEntries := make([]ports.DenylistEntry, 0, len(payload.EndpointDenylist))
				for _, e := range payload.EndpointDenylist {
					denylistEntries = append(denylistEntries, ports.DenylistEntry{
						BlockedIP:   e.BlockedIP,
						BlockedPort: e.BlockedPort,
					})
				}
				quarantinedIPs := append([]string(nil), payload.Quarantined...)

				wgListenPort := getWireGuardListenPort(r.getInterface())

				// Cache the policy + raw whitelist so resyncFirewall() (called from
				// the 300 ms ticker) can re-apply iptables when a peer's endpoint
				// changes between server pushes.
				r.lastSyncMu.Lock()
				r.lastPolicy = payload.Policy
				r.lastWhitelistRaw = whitelistedIPs
				r.lastPendingAuthIPs = pendingWgIPs
				r.lastQuarantinedIPs = quarantinedIPs
				r.lastEndpointDenylist = denylistEntries
				r.lastWGListenPort = wgListenPort
				r.lastSyncMu.Unlock()

				// Notify the captive portal server that at least one policy has been
				// received. Until this point the server treats all peers as
				// unauthenticated to avoid serving stale whitelist data.
				r.captivePortalSrvMu.Lock()
				cpSrv := r.captivePortalSrv
				r.captivePortalSrvMu.Unlock()
				if cpSrv != nil {
					cpSrv.NotifyPolicyReceived()
					// Drop cached redirect tokens for any peer that the server
					// no longer lists as pending_auth.  This is what makes
					// "Revoke Auth" take immediate effect for the next browser
					// hit: without it, the agent would keep handing out the
					// old token URL for up to tokenTTL (≈ 9 min) and the
					// browser would land on /start → "persist state: token
					// not found".
					cpSrv.RetainPendingTokens(pendingWgIPs)
				}

				// Push per-peer routes to the DNS server so it can decide route-aware
				// whether to redirect external queries from unauthenticated peers
				// (full-tunnel = redirect everything; split-tunnel = leave external alone).
				r.applyPeerRoutesToDNS(payload.PeerRoutes)

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
				pendingWithIPv6 := r.extendWhitelistWithIPv6(pendingWgIPs)
				r.lastSyncMu.Lock()
				r.lastFwState = filtered
				r.lastSyncMu.Unlock()

				if err := r.fwAdapter.Sync(ports.SyncRequest{
					Policy:              payload.Policy,
					SelfIP:              payload.Policy.IP,
					AuthenticatedIPs:    filteredWithIPv6,
					PendingAuthIPs:      pendingWithIPv6,
					QuarantinedIPs:      quarantinedIPs,
					EndpointDenylist:    denylistEntries,
					WireGuardListenPort: wgListenPort,
				}); err != nil {
					log.Error().Err(err).Msg("failed applying firewall policy update")
				} else {
					log.Info().
						Int("iptables_rule_count", len(payload.Policy.IPTablesRules)).
						Int("authenticated_peers", len(filtered)).
						Int("pending_auth_peers", len(pendingWgIPs)).
						Int("quarantined_peers", len(quarantinedIPs)).
						Int("denylist_entries", len(denylistEntries)).
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

// queueTakeoverIfRogue inspects an endpoint change for an authenticated peer
// and decides whether it represents a *rogue takeover* (which should be
// denylisted) or a *legitimate single roam* (which should be left to the
// captive-portal flow to re-authenticate).
//
// The distinction is made by counting OSCILLATIONS, not endpoint changes:
//
//   • A single change (stored=A, then live=B forever) is a legitimate roam:
//     NAT port rebinding, mobile network handover, fresh tunnel handshake from
//     the legitimate user, etc.  Denylisting B in this case would lock the
//     legitimate user out of the network for 24 h with no recovery path
//     (their UDP packets get DROPed before reaching WireGuard, so they can't
//     even reach the captive portal to re-authenticate).
//
//   • Multiple A→foreign flips within flipDetectionWindow is the unambiguous
//     signature of TWO simultaneously-active devices: each device's keepalive
//     overrides the other's recorded endpoint, so we see the live endpoint
//     bounce repeatedly between two values.  Only at this point do we have
//     high confidence one of the values is rogue.
//
// We track per-peer endpoint history (lastWasStored, flipsToForeign) to count
// stored→foreign transitions only.  After flipsRequiredForDenylist (2) such
// transitions inside flipDetectionWindow (60 s), the most recent foreign
// endpoint is queued as a takeover report.  Then we reset and start fresh —
// if a NEW rogue source shows up later it has to earn its denylist entry the
// same way.
//
// Must be called with r.endpointChangedMu held.
func (r *Runner) queueTakeoverIfRogue(wgIP, newEP string) {
	r.whitelistMu.RLock()
	storedEP, isAuthed := r.whitelist[wgIP]
	r.whitelistMu.RUnlock()
	if !isAuthed || storedEP == "" {
		// Peer is not authenticated — captive portal flow handles this case
		// from scratch; nothing to defend.
		return
	}

	r.takeoverFlipsMu.Lock()
	state, ok := r.takeoverFlips[wgIP]
	if !ok {
		// First observation for this peer.  Assume the previous live endpoint
		// was the authenticated one (otherwise it wouldn't be in the whitelist
		// in the first place).
		state = &takeoverFlipState{lastWasStored: true}
		r.takeoverFlips[wgIP] = state
	}
	now := time.Now()

	// Forget stale flips: if the first counted flip was longer ago than the
	// detection window, reset the counter — a rebind that happened a minute
	// ago and has held since is, by definition, a legitimate roam.
	if !state.firstFlipAt.IsZero() && now.Sub(state.firstFlipAt) > flipDetectionWindow {
		state.flipsToForeign = 0
		state.firstFlipAt = time.Time{}
	}

	if newEP == storedEP {
		// Endpoint flipped back to the authenticated value.  This in itself
		// is normal (legitimate user's keepalive arrived); we just record
		// "the last reading was at stored" so the next foreign reading
		// counts as a fresh stored→foreign transition.
		state.lastWasStored = true
		r.takeoverFlipsMu.Unlock()
		return
	}

	// newEP is foreign.  Count it as a flip ONLY if the previous reading was
	// at the stored endpoint — successive foreign readings with the SAME
	// foreign value (no bounce-back to stored in between) is just one event.
	if state.lastWasStored {
		state.flipsToForeign++
		if state.firstFlipAt.IsZero() {
			state.firstFlipAt = now
		}
		log.Debug().
			Str("wg_ip", wgIP).
			Str("authenticated_endpoint", storedEP).
			Str("observed_endpoint", newEP).
			Int("flips_to_foreign", state.flipsToForeign).
			Int("required", flipsRequiredForDenylist).
			Msg("endpoint flipped from authenticated to foreign — counting toward takeover threshold")
	}
	state.lastWasStored = false
	state.lastForeignEP = newEP

	if state.flipsToForeign < flipsRequiredForDenylist {
		// One flip: indistinguishable from a NAT rebind / roam.  Don't denylist.
		// The stability window (10 s) plus the captive portal flow take care
		// of the rest: if the legitimate user just moved networks they will
		// re-authenticate from the new endpoint and continue working.
		r.takeoverFlipsMu.Unlock()
		return
	}

	// Threshold crossed: we've seen the endpoint bounce stored→foreign at least
	// twice within flipDetectionWindow.  This is concrete evidence of two
	// simultaneously-active devices.  Reset the counter so that, if the rogue
	// later moves to yet a different IP:port, it has to re-cross the threshold
	// before being denylisted again.
	state.flipsToForeign = 0
	state.firstFlipAt = time.Time{}
	r.takeoverFlipsMu.Unlock()

	// Spam-suppress repeated reports for the same (wgIP, foreign) pair.
	key := wgIP + "|" + newEP
	r.reportedTakeoversMu.Lock()
	if last, ok := r.reportedTakeovers[key]; ok && now.Sub(last) < takeoverReportCooldown {
		r.reportedTakeoversMu.Unlock()
		return
	}
	r.reportedTakeovers[key] = now
	r.reportedTakeoversMu.Unlock()

	r.pendingTakeoversMu.Lock()
	r.pendingTakeovers = append(r.pendingTakeovers, endpointTakeoverReport{
		WgIP:            wgIP,
		AuthenticatedAt: storedEP,
		ObservedAt:      newEP,
	})
	r.pendingTakeoversMu.Unlock()

	log.Warn().
		Str("wg_ip", wgIP).
		Str("authenticated_endpoint", storedEP).
		Str("observed_endpoint", newEP).
		Msg("captive portal: rogue takeover confirmed (endpoint oscillated ≥2× to foreign source within detection window) — queued for server denylist")
}

// applyPeerRoutesToDNS forwards the per-peer AllowedIPs map to the DNS server,
// which uses it to decide route-aware whether to redirect external queries from
// unauthenticated peers (full-tunnel = redirect everything; split-tunnel = only
// internal/probe redirection).  No-op if the DNS server doesn't implement the
// optional interface (older builds, non-jump peers).
func (r *Runner) applyPeerRoutesToDNS(routes map[string][]string) {
	if routes == nil {
		return
	}
	r.dnsServerMu.Lock()
	dns := r.dnsServer
	r.dnsServerMu.Unlock()
	if dns == nil {
		return
	}
	type peerRoutesSetter interface {
		SetPeerRoutes(routes map[string][]string)
	}
	if setter, ok := dns.(peerRoutesSetter); ok {
		setter.SetPeerRoutes(routes)
	}
}

// drainPendingTakeovers atomically returns and clears the takeover queue.
func (r *Runner) drainPendingTakeovers() []endpointTakeoverReport {
	r.pendingTakeoversMu.Lock()
	defer r.pendingTakeoversMu.Unlock()
	if len(r.pendingTakeovers) == 0 {
		return nil
	}
	out := r.pendingTakeovers
	r.pendingTakeovers = nil
	return out
}

// SetLocalAllowedIPs records this peer's locally-configured WireGuard AllowedIPs
// so they can be reported in every heartbeat.  Called after each successful
// config apply by parseLocalAllowedIPsFromConfig.
func (r *Runner) SetLocalAllowedIPs(cidrs []string) {
	cp := append([]string(nil), cidrs...)
	r.localAllowedIPsMu.Lock()
	r.localAllowedIPs = cp
	r.localAllowedIPsMu.Unlock()
}

func (r *Runner) getLocalAllowedIPs() []string {
	r.localAllowedIPsMu.RLock()
	defer r.localAllowedIPsMu.RUnlock()
	cp := append([]string(nil), r.localAllowedIPs...)
	return cp
}

// parseLocalAllowedIPsFromConfig extracts the union of all "AllowedIPs = ..."
// entries from the [Peer] sections of the WireGuard config text.  This is what
// the local kernel will route through the VPN — i.e. the peer's effective
// "what goes through the tunnel" set.
func parseLocalAllowedIPsFromConfig(cfg string) []string {
	var out []string
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(strings.NewReader(cfg))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(strings.ToLower(line), "allowedips") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx == -1 {
			continue
		}
		for _, cidr := range strings.Split(line[idx+1:], ",") {
			cidr = strings.TrimSpace(cidr)
			if cidr == "" {
				continue
			}
			if _, ok := seen[cidr]; ok {
				continue
			}
			seen[cidr] = struct{}{}
			out = append(out, cidr)
		}
	}
	return out
}

// sendHeartbeat sends system information to the server
func (r *Runner) sendHeartbeat() {
	sysInfo, err := CollectSystemInfo(r.getInterface())
	if err != nil {
		log.Error().Err(err).Msg("failed to collect system info for heartbeat")
		return
	}

	takeovers := r.drainPendingTakeovers()
	// Convert to the wire-form expected by the server.  We use a generic
	// map[string]interface{} to avoid pulling the server domain package into
	// the agent module.
	var takeoverWire []map[string]string
	if len(takeovers) > 0 {
		takeoverWire = make([]map[string]string, 0, len(takeovers))
		for _, t := range takeovers {
			takeoverWire = append(takeoverWire, map[string]string{
				"wg_ip":            t.WgIP,
				"authenticated_at": t.AuthenticatedAt,
				"observed_at":      t.ObservedAt,
			})
		}
	}

	heartbeat := map[string]interface{}{
		"hostname":         sysInfo.Hostname,
		"system_uptime":    sysInfo.SystemUptime,
		"wireguard_uptime": sysInfo.WireGuardUptime,
		"peer_endpoints":   sysInfo.PeerEndpoints,
	}

	// Include WireGuard handshake timestamps so the server can use real
	// data-plane liveness (not just endpoint presence) for connectivity detection.
	// wg show latest-handshakes returns a timestamp-per-peer map; zero-valued
	// entries (no handshake yet) are already filtered out by GetWireGuardHandshakes.
	if handshakes := GetWireGuardHandshakes(r.getInterface()); len(handshakes) > 0 {
		handshakeUnix := make(map[string]int64, len(handshakes))
		for pubKey, t := range handshakes {
			handshakeUnix[pubKey] = t.Unix()
		}
		heartbeat["peer_handshakes"] = handshakeUnix
	}

	if local := r.getLocalAllowedIPs(); len(local) > 0 {
		heartbeat["local_allowed_ips"] = local
	}
	if len(takeoverWire) > 0 {
		heartbeat["endpoint_takeovers"] = takeoverWire
	}

	data, err := json.Marshal(heartbeat)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal heartbeat")
		return
	}

	if err := r.wsClient.WriteMessage(data); err != nil {
		log.Debug().Err(err).Msg("failed to send heartbeat (will retry)")
		// On send failure, re-queue the takeovers so we retry next time.
		if len(takeovers) > 0 {
			r.pendingTakeoversMu.Lock()
			r.pendingTakeovers = append(takeovers, r.pendingTakeovers...)
			r.pendingTakeoversMu.Unlock()
		}
	} else {
		log.Trace().
			Str("hostname", sysInfo.Hostname).
			Int64("system_uptime", sysInfo.SystemUptime).
			Int64("wireguard_uptime", sysInfo.WireGuardUptime).
			Interface("peer_endpoints", sysInfo.PeerEndpoints).
			Int("takeovers", len(takeovers)).
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
	//
	// We use net.JoinHostPort because IPv6 addresses contain colons and the
	// "ipv6:port" form is ambiguous; net.JoinHostPort produces "[ipv6]:port".
	if r.wgIP != "" {
		tlsAddr := net.JoinHostPort(r.wgIP, "443")
		go func() {
			if err := srv.StartTLS(tlsAddr, r.wgIP, r.vpnDomain); err != nil {
				log.Error().Str("addr", tlsAddr).Err(err).Msg("captive portal HTTPS server (IPv4) stopped")
			}
		}()

		addr := net.JoinHostPort(r.wgIP, "80")
		log.Info().Str("addr", addr).Str("portal_url", r.captivePortalURL).Msg("starting captive portal HTTP server (IPv4)")
		go func() {
			if err := srv.Start(addr); err != nil {
				log.Error().Str("addr", addr).Err(err).Msg("captive portal HTTP server (IPv4) stopped")
			}
		}()
	}

	// Spawn the same captive portal endpoints on the IPv6 address (dual-stack
	// deployments).  The same Server instance handles both — the captive portal
	// is stateless w.r.t. listening address, and `r.RemoteAddr` in handlers
	// already gives the per-connection peer IP.
	if r.wgIPv6 != "" {
		tlsAddr := net.JoinHostPort(r.wgIPv6, "443")
		go func() {
			if err := srv.StartTLS(tlsAddr, r.wgIPv6, r.vpnDomain); err != nil {
				log.Error().Str("addr", tlsAddr).Err(err).Msg("captive portal HTTPS server (IPv6) stopped")
			}
		}()

		addr := net.JoinHostPort(r.wgIPv6, "80")
		log.Info().Str("addr", addr).Str("portal_url", r.captivePortalURL).Msg("starting captive portal HTTP server (IPv6)")
		go func() {
			if err := srv.Start(addr); err != nil {
				log.Error().Str("addr", addr).Err(err).Msg("captive portal HTTP server (IPv6) stopped")
			}
		}()
	}

	// Block forever (one of the goroutines holds the actual listener); this
	// goroutine is intentionally kept alive so the caller's `go startCaptivePortalServer()`
	// continues to "own" the lifecycle of the server.
	select {}
}
