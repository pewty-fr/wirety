package network

import "context"

// GroupRepository defines the interface for group data persistence
type GroupRepository interface {
	// Group CRUD operations
	CreateGroup(ctx context.Context, networkID string, group *Group) error
	GetGroup(ctx context.Context, networkID, groupID string) (*Group, error)
	UpdateGroup(ctx context.Context, networkID string, group *Group) error
	DeleteGroup(ctx context.Context, networkID, groupID string) error
	ListGroups(ctx context.Context, networkID string) ([]*Group, error)

	// Peer membership operations
	AddPeerToGroup(ctx context.Context, networkID, groupID, peerID string) error
	RemovePeerFromGroup(ctx context.Context, networkID, groupID, peerID string) error
	GetPeerGroups(ctx context.Context, networkID, peerID string) ([]*Group, error)

	// Policy attachment operations
	AttachPolicyToGroup(ctx context.Context, networkID, groupID, policyID string) error
	DetachPolicyFromGroup(ctx context.Context, networkID, groupID, policyID string) error
	GetGroupPolicies(ctx context.Context, networkID, groupID string) ([]*Policy, error)
	ReorderGroupPolicies(ctx context.Context, networkID, groupID string, policyIDs []string) error

	// Route attachment operations
	AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error
	DetachRouteFromGroup(ctx context.Context, networkID, groupID, routeID string) error
	GetGroupRoutes(ctx context.Context, networkID, groupID string) ([]*Route, error)
}
