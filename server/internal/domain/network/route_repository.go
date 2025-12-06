package network

import "context"

// RouteRepository defines the interface for route data persistence
type RouteRepository interface {
	// Route CRUD operations
	CreateRoute(ctx context.Context, networkID string, route *Route) error
	GetRoute(ctx context.Context, networkID, routeID string) (*Route, error)
	UpdateRoute(ctx context.Context, networkID string, route *Route) error
	DeleteRoute(ctx context.Context, networkID, routeID string) error
	ListRoutes(ctx context.Context, networkID string) ([]*Route, error)

	// Get routes for a specific group
	GetRoutesForGroup(ctx context.Context, networkID, groupID string) ([]*Route, error)

	// Get routes by jump peer
	GetRoutesByJumpPeer(ctx context.Context, networkID, jumpPeerID string) ([]*Route, error)
}
