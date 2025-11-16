package network

import "context"

// IPAMPrefix holds minimal information about an allocated prefix
type IPAMPrefix struct {
	CIDR        string `json:"cidr"`
	ParentCIDR  string `json:"parent_cidr,omitempty"`
	UsableHosts int    `json:"usable_hosts"`
}

// Repository defines the interface for network data persistence
type Repository interface {
	// Network operations
	CreateNetwork(ctx context.Context, network *Network) error
	GetNetwork(ctx context.Context, networkID string) (*Network, error)
	UpdateNetwork(ctx context.Context, network *Network) error
	DeleteNetwork(ctx context.Context, networkID string) error
	ListNetworks(ctx context.Context) ([]*Network, error)

	// Peer operations
	CreatePeer(ctx context.Context, networkID string, peer *Peer) error
	GetPeer(ctx context.Context, networkID, peerID string) (*Peer, error)
	UpdatePeer(ctx context.Context, networkID string, peer *Peer) error
	DeletePeer(ctx context.Context, networkID, peerID string) error
	ListPeers(ctx context.Context, networkID string) ([]*Peer, error)

	// ACL operations
	CreateACL(ctx context.Context, networkID string, acl *ACL) error
	GetACL(ctx context.Context, networkID string) (*ACL, error)
	UpdateACL(ctx context.Context, networkID string, acl *ACL) error

	// PeerConnection operations (preshared keys between peer pairs)
	CreateConnection(ctx context.Context, networkID string, conn *PeerConnection) error
	GetConnection(ctx context.Context, networkID, peer1ID, peer2ID string) (*PeerConnection, error)
	ListConnections(ctx context.Context, networkID string) ([]*PeerConnection, error)
	DeleteConnection(ctx context.Context, networkID, peer1ID, peer2ID string) error

	// IPAM operations (to allow persistence of address management)
	EnsureRootPrefix(ctx context.Context, cidr string) (*IPAMPrefix, error)
	AcquireChildPrefix(ctx context.Context, parentCIDR string, prefixLen uint8) (*IPAMPrefix, error)
	ListChildPrefixes(ctx context.Context, parentCIDR string) ([]*IPAMPrefix, error)
	AcquireIP(ctx context.Context, cidr string) (string, error)
	ReleaseIP(ctx context.Context, cidr string, ip string) error
}
