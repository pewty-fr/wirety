package network

// ACL represents access control rules for the network
type ACL struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Enabled      bool            `json:"enabled"`
	BlockedPeers map[string]bool `json:"blocked_peers"` // Peer IDs that are blocked
	Rules        []ACLRule       `json:"rules"`
}

// ACLRule represents a single ACL rule
type ACLRule struct {
	ID          string `json:"id"`
	SourcePeer  string `json:"source_peer"` // Source peer ID or "*" for all
	TargetPeer  string `json:"target_peer"` // Target peer ID or "*" for all
	Action      string `json:"action"`      // "allow" or "deny"
	Description string `json:"description"`
}

// CanCommunicate checks if two peers can communicate based on ACL rules
func (a *ACL) CanCommunicate(sourcePeerID, targetPeerID string) bool {
	if !a.Enabled {
		return true
	}

	// Check if either peer is explicitly blocked
	if a.BlockedPeers[sourcePeerID] || a.BlockedPeers[targetPeerID] {
		return false
	}

	// Check specific rules (deny takes precedence)
	for _, rule := range a.Rules {
		if matchesPeer(rule.SourcePeer, sourcePeerID) && matchesPeer(rule.TargetPeer, targetPeerID) {
			return rule.Action == "allow"
		}
	}

	// Default: allow communication
	return true
}

func matchesPeer(pattern, peerID string) bool {
	return pattern == "*" || pattern == peerID
}
