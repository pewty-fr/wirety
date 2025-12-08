package network

import "time"

// Peer represents a network participant in the WireGuard mesh
// Two types of peers exist:
// - Jump peers: Act as hubs routing traffic for regular peers
// - Regular peers: Connect through jump peers
type Peer struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	PublicKey            string    `json:"public_key"`
	PrivateKey           string    `json:"-"`                                // Never expose private key in API responses (only used for config generation)
	Address              string    `json:"address"`                          // IP address in the network CIDR
	Endpoint             string    `json:"endpoint,omitempty"`               // External endpoint (IP:port)
	ListenPort           int       `json:"listen_port,omitempty"`            // WireGuard listen port (mainly for jump peers)
	AdditionalAllowedIPs []string  `json:"additional_allowed_ips,omitempty"` // Additional IPs this peer can route to
	Token                string    `json:"token,omitempty"`                  // Agent enrollment token (secret)
	IsJump               bool      `json:"is_jump"`                          // Whether this peer acts as a jump server (hub)
	UseAgent             bool      `json:"use_agent"`                        // Whether this peer uses the agent (dynamic) or static config
	OwnerID              string    `json:"owner_id,omitempty"`               // User ID who owns this peer (empty for admin-created peers)
	GroupIDs             []string  `json:"group_ids"`                        // Groups this peer belongs to
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// PeerConnection represents a preshared key between two peers
type PeerConnection struct {
	Peer1ID      string    `json:"peer1_id"`
	Peer2ID      string    `json:"peer2_id"`
	PresharedKey string    `json:"preshared_key"`
	CreatedAt    time.Time `json:"created_at"`
}

// PeerCreateRequest represents the data needed to create a new peer
type PeerCreateRequest struct {
	Name                 string   `json:"name" binding:"required"`
	Endpoint             string   `json:"endpoint,omitempty"`
	ListenPort           int      `json:"listen_port,omitempty"`
	IsJump               bool     `json:"is_jump"`
	UseAgent             bool     `json:"use_agent"`
	AdditionalAllowedIPs []string `json:"additional_allowed_ips,omitempty"`
}

// PeerUpdateRequest represents the data that can be updated for a peer
type PeerUpdateRequest struct {
	Name                 string   `json:"name,omitempty"`
	Endpoint             string   `json:"endpoint,omitempty"`
	ListenPort           int      `json:"listen_port,omitempty"`
	AdditionalAllowedIPs []string `json:"additional_allowed_ips,omitempty"`
	OwnerID              string   `json:"owner_id,omitempty"` // Admin can change owner
}
