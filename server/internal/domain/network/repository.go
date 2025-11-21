package network

import (
	"context"
	"time"
)

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
	GetPeerByToken(ctx context.Context, token string) (networkID string, peer *Peer, err error)
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

	// Agent session operations
	CreateOrUpdateSession(ctx context.Context, networkID string, session *AgentSession) error
	GetSession(ctx context.Context, networkID, peerID string) (*AgentSession, error)
	GetActiveSessionsForPeer(ctx context.Context, networkID, peerID string) ([]*AgentSession, error)
	DeleteSession(ctx context.Context, networkID, sessionID string) error
	ListSessions(ctx context.Context, networkID string) ([]*AgentSession, error)

	// Endpoint change tracking
	RecordEndpointChange(ctx context.Context, networkID string, change *EndpointChange) error
	GetEndpointChanges(ctx context.Context, networkID, peerID string, since time.Time) ([]*EndpointChange, error)

	// Security incident operations
	CreateSecurityIncident(ctx context.Context, incident *SecurityIncident) error
	GetSecurityIncident(ctx context.Context, incidentID string) (*SecurityIncident, error)
	ListSecurityIncidents(ctx context.Context, resolved *bool) ([]*SecurityIncident, error)
	ListSecurityIncidentsByNetwork(ctx context.Context, networkID string, resolved *bool) ([]*SecurityIncident, error)
	ResolveSecurityIncident(ctx context.Context, incidentID, resolvedBy string) error
}
