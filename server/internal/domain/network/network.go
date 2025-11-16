package network

import "time"

// Network represents a WireGuard mesh network
type Network struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	CIDR      string           `json:"cidr"`   // Network CIDR (e.g., "10.0.0.0/16")
	Domain    string           `json:"domain"` // DNS domain for the network
	Peers     map[string]*Peer `json:"peers"`  // Peer ID -> Peer
	ACL       *ACL             `json:"acl"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// NetworkCreateRequest represents the data needed to create a new network
type NetworkCreateRequest struct {
	Name   string   `json:"name" binding:"required"`
	CIDR   string   `json:"cidr" binding:"required"`
	Domain string   `json:"domain" binding:"required"`
	DNS    []string `json:"dns,omitempty"`
}

// NetworkUpdateRequest represents the data that can be updated for a network
type NetworkUpdateRequest struct {
	Name   string `json:"name,omitempty"`
	Domain string `json:"domain,omitempty"`
	CIDR   string `json:"cidr,omitempty"`
}

// AddPeer adds a peer to the network
func (n *Network) AddPeer(peer *Peer) {
	if n.Peers == nil {
		n.Peers = make(map[string]*Peer)
	}
	n.Peers[peer.ID] = peer
	n.UpdatedAt = time.Now()
}

// RemovePeer removes a peer from the network
func (n *Network) RemovePeer(peerID string) {
	delete(n.Peers, peerID)
	n.UpdatedAt = time.Now()
}

// GetPeer retrieves a peer by ID
func (n *Network) GetPeer(peerID string) (*Peer, bool) {
	peer, exists := n.Peers[peerID]
	return peer, exists
}

// GetAllPeers returns all peers in the network
func (n *Network) GetAllPeers() []*Peer {
	peers := make([]*Peer, 0, len(n.Peers))
	for _, peer := range n.Peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetAllowedPeersFor returns peers to include in WireGuard config for peerID.
// Regular peers: only jump peers are listed (tunnel hub pattern). All peer-to-peer
// communication goes through jump servers.
// Jump peers: all other peers are listed, with ACL filtering (isolation enforced via jump iptables).
func (n *Network) GetAllowedPeersFor(peerID string) []*Peer {
	result := make([]*Peer, 0)

	peer, exists := n.Peers[peerID]
	if !exists {
		return result
	}

	// If this is a jump peer, include all other peers (respect ACL if present)
	if peer.IsJump {
		for _, other := range n.Peers {
			if other.ID == peerID {
				continue
			}
			if n.ACL != nil && !n.ACL.CanCommunicate(peerID, other.ID) {
				continue
			}
			result = append(result, other)
		}
		return result
	}

	// Regular peer: only jump peers (respect ACL if present)
	for _, other := range n.Peers {
		if !other.IsJump || other.ID == peerID {
			continue
		}
		if n.ACL != nil && !n.ACL.CanCommunicate(peerID, other.ID) {
			continue
		}
		result = append(result, other)
	}
	return result
}

// HasJumpServer checks if the network has at least one jump server
func (n *Network) HasJumpServer() bool {
	for _, peer := range n.Peers {
		if peer.IsJump {
			return true
		}
	}
	return false
}

// GetJumpServers returns all jump servers in the network
func (n *Network) GetJumpServers() []*Peer {
	jumps := make([]*Peer, 0)
	for _, peer := range n.Peers {
		if peer.IsJump {
			jumps = append(jumps, peer)
		}
	}
	return jumps
}
