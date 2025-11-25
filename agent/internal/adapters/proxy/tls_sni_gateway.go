package proxy

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// TLSSNIGateway implements a TLS-SNI based gateway that only allows
// connections to whitelisted domains (primarily the server URL)
type TLSSNIGateway struct {
	port             int
	allowedDomains   map[string]bool
	listener         net.Listener
	mu               sync.RWMutex
	nonAgentPeers    map[string]bool // IP -> true for non-agent peers
	whitelistedPeers map[string]bool // IP -> true for authenticated peers
	shutdown         chan struct{}
}

// NewTLSSNIGateway creates a new TLS-SNI gateway
func NewTLSSNIGateway(port int, serverURL string) (*TLSSNIGateway, error) {
	// Parse server URL to extract domain
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	// Extract hostname (remove port if present)
	hostname := parsedURL.Hostname()

	gateway := &TLSSNIGateway{
		port:             port,
		allowedDomains:   make(map[string]bool),
		nonAgentPeers:    make(map[string]bool),
		whitelistedPeers: make(map[string]bool),
		shutdown:         make(chan struct{}),
	}

	// Add the server domain to allowed list
	// This is the ONLY domain allowed for non-authenticated users
	gateway.allowedDomains[strings.ToLower(hostname)] = true

	log.Info().
		Str("allowed_domain", hostname).
		Int("port", port).
		Msg("TLS-SNI gateway initialized - only server domain allowed")

	return gateway, nil
}

// UpdateNonAgentPeers updates the list of non-agent peer IPs
func (g *TLSSNIGateway) UpdateNonAgentPeers(peerIPs []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nonAgentPeers = make(map[string]bool)
	for _, ip := range peerIPs {
		g.nonAgentPeers[ip] = true
	}

	log.Debug().Int("count", len(peerIPs)).Msg("updated non-agent peers for TLS-SNI gateway")
}

// AddWhitelistedPeer adds a peer IP to the whitelist
func (g *TLSSNIGateway) AddWhitelistedPeer(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.whitelistedPeers[ip] = true
}

// RemoveWhitelistedPeer removes a peer IP from the whitelist
func (g *TLSSNIGateway) RemoveWhitelistedPeer(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.whitelistedPeers, ip)
}

// ClearWhitelist clears all whitelisted peers
func (g *TLSSNIGateway) ClearWhitelist() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.whitelistedPeers = make(map[string]bool)
}

// AddAllowedDomain adds a domain to the allowed list (e.g., OAuth issuer)
func (g *TLSSNIGateway) AddAllowedDomain(domain string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Parse URL if it's a full URL
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		if parsedURL, err := url.Parse(domain); err == nil {
			domain = parsedURL.Hostname()
		}
	}

	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain != "" {
		g.allowedDomains[domain] = true
		log.Info().Str("domain", domain).Msg("added allowed domain to TLS-SNI gateway")
	}
}

// isNonAgentPeer checks if an IP is a non-agent peer
func (g *TLSSNIGateway) isNonAgentPeer(ip string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nonAgentPeers[ip]
}

// isWhitelisted checks if an IP is whitelisted (authenticated)
func (g *TLSSNIGateway) isWhitelisted(ip string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.whitelistedPeers[ip]
}

// isDomainAllowed checks if a domain is in the allowed list
func (g *TLSSNIGateway) isDomainAllowed(domain string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.allowedDomains[strings.ToLower(domain)]
}

// Start starts the TLS-SNI gateway
func (g *TLSSNIGateway) Start() error {
	var err error
	g.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", g.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", g.port, err)
	}

	log.Info().Int("port", g.port).Msg("TLS-SNI gateway started")

	go g.acceptLoop()

	return nil
}

// Stop stops the TLS-SNI gateway
func (g *TLSSNIGateway) Stop() error {
	close(g.shutdown)
	if g.listener != nil {
		return g.listener.Close()
	}
	return nil
}

// acceptLoop accepts incoming connections
func (g *TLSSNIGateway) acceptLoop() {
	for {
		select {
		case <-g.shutdown:
			return
		default:
		}

		conn, err := g.listener.Accept()
		if err != nil {
			select {
			case <-g.shutdown:
				return
			default:
				log.Error().Err(err).Msg("failed to accept connection")
				continue
			}
		}

		go g.handleConnection(conn)
	}
}

// handleConnection handles a single TLS connection
func (g *TLSSNIGateway) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set read deadline for ClientHello
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Error().Err(err).Msg("failed to set read deadline")
		return
	}

	// Extract client IP
	clientIP := conn.RemoteAddr().String()
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		clientIP = clientIP[:idx]
	}

	// Check if this is a non-agent peer
	if !g.isNonAgentPeer(clientIP) {
		// Not a non-agent peer, shouldn't reach here
		return
	}

	// Check if peer is whitelisted (authenticated)
	if g.isWhitelisted(clientIP) {
		// Whitelisted peer, shouldn't be redirected anymore
		// This shouldn't happen if firewall rules are updated correctly
		return
	}

	// Read enough bytes for ClientHello
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		log.Debug().Err(err).Str("client_ip", clientIP).Msg("failed to read ClientHello")
		return
	}

	// Extract SNI from ClientHello
	sni, err := extractSNI(buf[:n])
	if err != nil {
		log.Debug().Err(err).Str("client_ip", clientIP).Msg("failed to extract SNI")
		// Close connection for non-TLS or malformed TLS
		return
	}

	log.Debug().
		Str("client_ip", clientIP).
		Str("sni", sni).
		Msg("TLS-SNI gateway received connection")

	// Check if domain is allowed
	if !g.isDomainAllowed(sni) {
		log.Debug().
			Str("client_ip", clientIP).
			Str("sni", sni).
			Msg("blocking connection to non-whitelisted domain")

		// Send a TLS alert to inform the client that the connection is being refused
		// This is more graceful than just closing the connection
		// Alert: Fatal, Access Denied (49)
		alertMsg := []byte{
			0x15,       // Alert protocol
			0x03, 0x03, // TLS 1.2
			0x00, 0x02, // Length: 2 bytes
			0x02, // Fatal
			0x31, // Access denied (49)
		}
		_, _ = conn.Write(alertMsg)

		// Close connection
		return
	}

	// Domain is allowed, establish connection to target and relay traffic
	log.Debug().
		Str("client_ip", clientIP).
		Str("sni", sni).
		Msg("allowing connection to whitelisted domain")

	// Resolve the domain
	target := fmt.Sprintf("%s:443", sni)
	remote, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		log.Error().Err(err).Str("target", target).Msg("failed to connect to target")
		return
	}
	defer remote.Close()

	// Clear read deadline for relay
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		log.Error().Err(err).Msg("failed to clear read deadline")
		return
	}

	// Send the ClientHello we already read to the remote server
	if _, err := remote.Write(buf[:n]); err != nil {
		log.Error().Err(err).Msg("failed to write ClientHello to remote")
		return
	}

	// Relay traffic bidirectionally
	done := make(chan struct{}, 2)

	// Client -> Server
	go func() {
		_, _ = io.Copy(remote, conn)
		done <- struct{}{}
	}()

	// Server -> Client
	go func() {
		_, _ = io.Copy(conn, remote)
		done <- struct{}{}
	}()

	// Wait for one direction to finish
	<-done
}

// extractSNI extracts the SNI hostname from a TLS ClientHello message
func extractSNI(data []byte) (string, error) {
	if len(data) < 5 {
		return "", fmt.Errorf("data too short")
	}

	// Check if this is a TLS handshake record (0x16)
	if data[0] != 0x16 {
		return "", fmt.Errorf("not a TLS handshake")
	}

	// TLS version (we support 1.0, 1.1, 1.2, 1.3)
	// Skip version check for now

	// Record length
	recordLen := int(binary.BigEndian.Uint16(data[3:5]))
	if len(data) < 5+recordLen {
		return "", fmt.Errorf("incomplete TLS record")
	}

	handshake := data[5 : 5+recordLen]

	// Handshake type must be ClientHello (0x01)
	if len(handshake) < 1 || handshake[0] != 0x01 {
		return "", fmt.Errorf("not a ClientHello")
	}

	// Skip handshake header (1 byte type + 3 bytes length)
	if len(handshake) < 4 {
		return "", fmt.Errorf("malformed handshake")
	}

	pos := 4

	// Skip client version (2 bytes)
	pos += 2

	// Skip random (32 bytes)
	if pos+32 > len(handshake) {
		return "", fmt.Errorf("malformed random")
	}
	pos += 32

	// Session ID length
	if pos >= len(handshake) {
		return "", fmt.Errorf("malformed session ID")
	}
	sessionIDLen := int(handshake[pos])
	pos += 1 + sessionIDLen

	// Cipher suites length
	if pos+2 > len(handshake) {
		return "", fmt.Errorf("malformed cipher suites")
	}
	cipherSuitesLen := int(binary.BigEndian.Uint16(handshake[pos : pos+2]))
	pos += 2 + cipherSuitesLen

	// Compression methods length
	if pos >= len(handshake) {
		return "", fmt.Errorf("malformed compression methods")
	}
	compressionMethodsLen := int(handshake[pos])
	pos += 1 + compressionMethodsLen

	// Extensions length
	if pos+2 > len(handshake) {
		// No extensions
		return "", fmt.Errorf("no extensions")
	}
	extensionsLen := int(binary.BigEndian.Uint16(handshake[pos : pos+2]))
	pos += 2

	extensionsEnd := pos + extensionsLen

	// Parse extensions
	for pos+4 <= extensionsEnd {
		extType := binary.BigEndian.Uint16(handshake[pos : pos+2])
		extLen := int(binary.BigEndian.Uint16(handshake[pos+2 : pos+4]))
		pos += 4

		if pos+extLen > extensionsEnd {
			break
		}

		// SNI extension type is 0x0000
		if extType == 0x0000 {
			return parseSNIExtension(handshake[pos : pos+extLen])
		}

		pos += extLen
	}

	return "", fmt.Errorf("SNI extension not found")
}

// parseSNIExtension parses the SNI extension data
func parseSNIExtension(data []byte) (string, error) {
	if len(data) < 2 {
		return "", fmt.Errorf("SNI extension too short")
	}

	// Server name list length
	listLen := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+listLen {
		return "", fmt.Errorf("incomplete SNI list")
	}

	pos := 2

	// Parse server names
	for pos+3 <= 2+listLen {
		nameType := data[pos]
		nameLen := int(binary.BigEndian.Uint16(data[pos+1 : pos+3]))
		pos += 3

		if pos+nameLen > len(data) {
			break
		}

		// Name type 0x00 is host_name
		if nameType == 0x00 {
			return string(data[pos : pos+nameLen]), nil
		}

		pos += nameLen
	}

	return "", fmt.Errorf("hostname not found in SNI")
}
