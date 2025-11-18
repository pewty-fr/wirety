package network

import "time"

// Peer represents a network participant in the WireGuard mesh
// Two types of peers exist:
// - Jump peers: Act as hubs with NatInterface, route all traffic
// - Regular peers: Connect through jump peers, can be isolated or fully encapsulated
type Peer struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	PublicKey            string    `json:"public_key"`
	PrivateKey           string    `json:"-"`                                // Never expose private key in API responses (only used for config generation)
	Address              string    `json:"address"`                          // IP address in the network CIDR
	Endpoint             string    `json:"endpoint"`                         // External endpoint (IP:port)
	ListenPort           int       `json:"listen_port,omitempty"`            // WireGuard listen port (mainly for jump peers)
	AdditionalAllowedIPs []string  `json:"additional_allowed_ips,omitempty"` // Additional IPs this peer can route to
	Token                string    `json:"token,omitempty"`                  // Agent enrollment token (secret)
	IsJump               bool      `json:"is_jump"`                          // Whether this peer acts as a jump server (hub)
	JumpNatInterface     string    `json:"jump_nat_interface,omitempty"`     // NAT interface for jump server
	IsIsolated           bool      `json:"is_isolated"`                      // Regular peers only: isolated from other peers
	FullEncapsulation    bool      `json:"full_encapsulation"`               // Regular peers only: route all traffic (0.0.0.0/0) through jump
	UseAgent             bool      `json:"use_agent"`                        // Whether this peer uses the agent (dynamic) or static config
	OwnerID              string    `json:"owner_id,omitempty"`               // User ID who owns this peer (empty for admin-created peers)
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
	JumpNatInterface     string   `json:"jump_nat_interface,omitempty"`
	IsIsolated           bool     `json:"is_isolated"`
	FullEncapsulation    bool     `json:"full_encapsulation"`
	UseAgent             bool     `json:"use_agent"`
	AdditionalAllowedIPs []string `json:"additional_allowed_ips,omitempty"`
}

// PeerUpdateRequest represents the data that can be updated for a peer
type PeerUpdateRequest struct {
	Name                 string   `json:"name,omitempty"`
	Endpoint             string   `json:"endpoint,omitempty"`
	ListenPort           int      `json:"listen_port,omitempty"`
	IsIsolated           bool     `json:"is_isolated"`
	FullEncapsulation    bool     `json:"full_encapsulation"`
	AdditionalAllowedIPs []string `json:"additional_allowed_ips,omitempty"`
}
