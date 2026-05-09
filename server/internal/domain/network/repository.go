package network

import (
	"context"
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

	// Captive portal whitelist operations
	AddCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP, peerEndpoint string) error
	RemoveCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID, peerIP string) error
	RemoveCaptivePortalWhitelistByPeerIP(ctx context.Context, networkID, peerIP string) error
	GetCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) ([]string, error)
	ClearCaptivePortalWhitelist(ctx context.Context, networkID, jumpPeerID string) error
	CleanupExpiredCaptivePortalWhitelist(ctx context.Context) error

	// Captive portal token operations
	CreateCaptivePortalToken(ctx context.Context, token *CaptivePortalToken) error
	GetCaptivePortalToken(ctx context.Context, token string) (*CaptivePortalToken, error)
	DeleteCaptivePortalToken(ctx context.Context, token string) error
	CleanupExpiredCaptivePortalTokens(ctx context.Context) error

	// ListActiveCaptivePortalTokens returns all unexpired tokens for a jump peer.
	// Used to populate the "pending auth" tier of the firewall: peers that have
	// been issued a token but have not yet completed SSO get a temporary HTTPS
	// allow rule for the OIDC redirect to work.
	ListActiveCaptivePortalTokens(ctx context.Context, networkID, jumpPeerID string) ([]*CaptivePortalToken, error)

	// MarkCaptivePortalTokenConsumed records the successful conversion of a token
	// into a whitelist entry.  Used by the strike-tracking cleanup loop to
	// distinguish "token expired with success" from "token expired without auth".
	MarkCaptivePortalTokenConsumed(ctx context.Context, token string) error

	// ListExpiredUnconsumedCaptivePortalTokens returns tokens that have expired
	// without ever being consumed (no successful auth).  The cleanup loop reads
	// this, records a strike per peer, and then deletes them.  Order is undefined.
	ListExpiredUnconsumedCaptivePortalTokens(ctx context.Context) ([]*CaptivePortalToken, error)

	// Captive portal endpoint denylist (per-peer rogue source blocking).
	AddEndpointDenylist(ctx context.Context, entry *EndpointDenylistEntry) error
	GetEndpointDenylist(ctx context.Context, networkID, jumpPeerID string) ([]*EndpointDenylistEntry, error)
	ClearEndpointDenylistForPeer(ctx context.Context, networkID, wgIP string) error
	CleanupExpiredEndpointDenylist(ctx context.Context) error

	// Captive portal quarantine (per-peer auth-failure tracking).
	GetQuarantine(ctx context.Context, networkID, peerID string) (*CaptivePortalQuarantine, error)
	UpsertQuarantine(ctx context.Context, q *CaptivePortalQuarantine) error
	ListQuarantinedPeers(ctx context.Context, networkID string) ([]*CaptivePortalQuarantine, error)
	ClearQuarantine(ctx context.Context, networkID, peerID string) error

	// Per-peer local routes (the peer's own AllowedIPs, reported via heartbeat).
	// Used by the jump peer's DNS server to decide whether to redirect external
	// queries for unauthenticated peers (full-tunnel) or leave them alone
	// (split-tunnel).
	UpsertPeerLocalRoutes(ctx context.Context, networkID, peerID string, allowedIPs []string) error
	GetPeerLocalRoutes(ctx context.Context, networkID, peerID string) ([]string, error)
	ListPeerLocalRoutes(ctx context.Context, networkID string) (map[string][]string, error) // peerID -> CIDRs
}
