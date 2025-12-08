package wireguard

import (
	"fmt"
	"net"
	"strings"

	domain "wirety/internal/domain/network"
)

// GenerateConfig generates a WireGuard configuration file for a peer
func GenerateConfig(peer *domain.Peer, allowedPeers []*domain.Peer, network *domain.Network, presharedKeys map[string]string, routes []*domain.Route) string {
	var sb strings.Builder

	// [Interface] section
	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("# Name: %s\n", peer.Name))
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", peer.PrivateKey))
	sb.WriteString(fmt.Sprintf("Address = %s\n", peer.Address))
	if peer.ListenPort > 0 {
		sb.WriteString(fmt.Sprintf("ListenPort = %d\n", peer.ListenPort))
	}

	// Add DNS configuration
	// For peers with internal domain support, use jump server DNS only
	// The jump server will forward external queries to upstream DNS servers
	if !peer.IsJump {
		dns := ""

		for _, allowedPeer := range allowedPeers {
			if allowedPeer.IsJump {
				dns = allowedPeer.Address
			}
		}

		if dns != "" {
			sb.WriteString(fmt.Sprintf("DNS = %s\n", dns))
		}
	} else {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", peer.Address))
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

		// Determine AllowedIPs based on peer type and routes
		allowedIPs := determineAllowedIPs(peer, allowedPeer, network, routes)
		sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(allowedIPs, ", ")))

		// Add endpoint if the allowed peer is a jump server or has an endpoint
		if allowedPeer.Endpoint != "" {
			sb.WriteString(fmt.Sprintf("Endpoint = %s:%d\n", allowedPeer.Endpoint, allowedPeer.ListenPort))
			sb.WriteString("PersistentKeepalive = 25\n")
		} else if peer.IsJump && !allowedPeer.IsJump {
			// Jump server connecting to regular peer (no endpoint)
			// Add keepalive so jump server can initiate handshakes and maintain connection
			// This is critical for mobile peers behind NAT
			sb.WriteString("PersistentKeepalive = 25\n")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// determineAllowedIPs determines the AllowedIPs for a peer connection
// Implements policy-based routing with group routes
func determineAllowedIPs(peer, allowedPeer *domain.Peer, network *domain.Network, routes []*domain.Route) []string {
	var allowedIPs []string

	// For jump peers: include network CIDR and all route CIDRs
	if peer.IsJump {
		// Include network CIDR for peer-to-peer communication within the network
		allowedIPs = []string{network.CIDR}

		// Include all route destination CIDRs for external network access
		for _, route := range routes {
			allowedIPs = append(allowedIPs, route.DestinationCIDR)
		}

		// Include any additional allowed IPs configured for this peer
		allowedIPs = append(allowedIPs, peer.AdditionalAllowedIPs...)

		return allowedIPs
	}

	// For regular peers connecting to a jump peer
	if allowedPeer.IsJump {
		// Include network CIDR for peer-to-peer communication within the network
		allowedIPs = []string{network.CIDR}

		// Include route CIDRs that use this jump peer as gateway
		for _, route := range routes {
			if route.JumpPeerID == allowedPeer.ID {
				allowedIPs = append(allowedIPs, route.DestinationCIDR)
			}
		}

		// Include any additional allowed IPs configured for the jump peer
		allowedIPs = append(allowedIPs, allowedPeer.AdditionalAllowedIPs...)
	} else {
		// Regular peer to regular peer: just the peer's address
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
// func extractDNSServer(cidr string) string {
// 	ip, ipnet, err := net.ParseCIDR(cidr)
// 	if err != nil {
// 		return ""
// 	}

// 	// Use the first IP in the range as DNS server (typically .1)
// 	ip = ip.Mask(ipnet.Mask)
// 	incIP(ip)

// 	return ip.String()
// }
