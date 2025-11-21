package wireguard

import (
	"fmt"
	"net"
	"strings"

	domain "wirety/internal/domain/network"
)

// GenerateConfig generates a WireGuard configuration file for a peer
func GenerateConfig(peer *domain.Peer, allowedPeers []*domain.Peer, network *domain.Network, presharedKeys map[string]string) string {
	var sb strings.Builder

	// [Interface] section
	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("# Name: %s\n", peer.Name))
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", peer.PrivateKey))
	sb.WriteString(fmt.Sprintf("Address = %s\n", peer.Address))
	if peer.ListenPort > 0 {
		sb.WriteString(fmt.Sprintf("ListenPort = %d\n", peer.ListenPort))
	}

	// Add DNS if domain is configured
	domain := network.GetDomain()
	if domain != "" {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", extractDNSServer(network.CIDR)))
	}

	// Jump server packet filtering & forwarding now handled dynamically by agent firewall adapter.
	// (No PostUp/PostDown iptables rules embedded in config.)

	sb.WriteString("\n")

	// [Peer] sections for each allowed peer
	for _, allowedPeer := range allowedPeers {
		sb.WriteString("[Peer]\n")
		sb.WriteString(fmt.Sprintf("# Name: %s\n", allowedPeer.Name))
		sb.WriteString(fmt.Sprintf("PublicKey = %s\n", allowedPeer.PublicKey))

		// Look up preshared key for this connection
		if psk, exists := presharedKeys[allowedPeer.ID]; exists && psk != "" {
			sb.WriteString(fmt.Sprintf("PresharedKey = %s\n", psk))
		}

		// Determine AllowedIPs
		allowedIPs := determineAllowedIPs(peer, allowedPeer, network)
		sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(allowedIPs, ", ")))

		// Add endpoint if the allowed peer is a jump server or has an endpoint
		if allowedPeer.Endpoint != "" {
			sb.WriteString(fmt.Sprintf("Endpoint = %s:%d\n", allowedPeer.Endpoint, allowedPeer.ListenPort))
			sb.WriteString("PersistentKeepalive = 25\n")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// determineAllowedIPs determines the AllowedIPs for a peer connection
func determineAllowedIPs(peer, allowedPeer *domain.Peer, network *domain.Network) []string {
	var allowedIPs []string

	// If the peer wants full encapsulation and the allowed peer is a jump server
	if peer.FullEncapsulation && allowedPeer.IsJump {
		// Route all traffic through jump server
		return []string{"0.0.0.0/0", "::/0"}
	}

	// If the allowed peer is a jump server, route the entire network through it
	if allowedPeer.IsJump {
		allowedIPs = []string{network.CIDR}
		allowedIPs = append(allowedIPs, allowedPeer.AdditionalAllowedIPs...)
	} else {
		// Otherwise, use the peer's address
		allowedIPs = []string{allowedPeer.Address + "/32"}
	}

	return allowedIPs
}

// AllocateIP allocates a new IP address from the network CIDR
func AllocateIP(cidr string, usedIPs []string) (string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %w", err)
	}

	// Create a map of used IPs for quick lookup
	used := make(map[string]bool)
	for _, usedIP := range usedIPs {
		// Extract IP without /32 suffix if present
		cleanIP := strings.TrimSuffix(usedIP, "/32")
		used[cleanIP] = true
	}

	// Iterate through the IP range to find an available IP
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		ipStr := ip.String()

		// Skip network address and broadcast address
		if isNetworkOrBroadcast(ip, ipnet) {
			continue
		}

		if !used[ipStr] {
			return ipStr + "/32", nil
		}
	}

	return "", fmt.Errorf("no available IPs in CIDR %s", cidr)
}

// incIP increments an IP address
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// isNetworkOrBroadcast checks if an IP is the network or broadcast address
func isNetworkOrBroadcast(ip net.IP, ipnet *net.IPNet) bool {
	// Network address check
	if ip.Equal(ipnet.IP) {
		return true
	}

	// Broadcast address check
	broadcast := make(net.IP, len(ip))
	for i := range ip {
		broadcast[i] = ipnet.IP[i] | ^ipnet.Mask[i]
	}

	return ip.Equal(broadcast)
}

// extractDNSServer extracts a DNS server IP from the CIDR (typically .1)
func extractDNSServer(cidr string) string {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return ""
	}

	// Use the first IP in the range as DNS server (typically .1)
	ip = ip.Mask(ipnet.Mask)
	incIP(ip)

	return ip.String()
}
