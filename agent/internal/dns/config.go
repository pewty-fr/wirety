package dns

import "strings"

// ParseWireGuardConfig parses a WireGuard config and extracts peer names and IPs.
// Assumes each peer has a # Name: comment and AllowedIPs line.
func ParseWireGuardConfig(cfg string, domain string) []Peer {
	var peers []Peer
	var name string
	for _, line := range strings.Split(cfg, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# Name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "# Name:"))
		}
		if strings.HasPrefix(line, "AllowedIPs =") && name != "" {
			ip := strings.TrimSpace(strings.Split(strings.TrimPrefix(line, "AllowedIPs ="), ",")[0])
			peers = append(peers, Peer{Name: sanitizeLabel(name), IP: ip})
			name = ""
		}
	}
	return peers
}

func sanitizeLabel(label string) string {
	label = strings.ToLower(label)
	label = strings.ReplaceAll(label, "_", "-")
	label = strings.ReplaceAll(label, ".", "-")
	return label
}
