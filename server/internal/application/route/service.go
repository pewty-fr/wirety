package route

import (
	"context"
	"fmt"
	"time"

	"wirety/internal/domain/network"

	"github.com/google/uuid"
)

// WebSocketNotifier is an interface for notifying peers about config updates
type WebSocketNotifier interface {
	NotifyNetworkPeers(networkID string)
}

// Service implements the business logic for route management
type Service struct {
	routeRepo  network.RouteRepository
	groupRepo  network.GroupRepository
	peerRepo   network.Repository
	wsNotifier WebSocketNotifier
}

// NewService creates a new route service
func NewService(routeRepo network.RouteRepository, groupRepo network.GroupRepository, peerRepo network.Repository) *Service {
	return &Service{
		routeRepo: routeRepo,
		groupRepo: groupRepo,
		peerRepo:  peerRepo,
	}
}

// SetWebSocketNotifier sets the WebSocket notifier for the service
func (s *Service) SetWebSocketNotifier(notifier WebSocketNotifier) {
	s.wsNotifier = notifier
}

// CreateRoute creates a new route with CIDR and jump peer validation
func (s *Service) CreateRoute(ctx context.Context, networkID string, req *network.RouteCreateRequest) (*network.Route, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Verify network exists
	_, err := s.peerRepo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	// Verify jump peer exists and is a jump peer
	jumpPeer, err := s.peerRepo.GetPeer(ctx, networkID, req.JumpPeerID)
	if err != nil {
		return nil, fmt.Errorf("jump peer not found: %w", err)
	}
	if !jumpPeer.IsJump {
		return nil, fmt.Errorf("peer is not a jump peer")
	}

	now := time.Now()
	domainSuffix := req.DomainSuffix
	if domainSuffix == "" {
		domainSuffix = "internal"
	}

	route := &network.Route{
		ID:              uuid.New().String(),
		NetworkID:       networkID,
		Name:            req.Name,
		Description:     req.Description,
		DestinationCIDR: req.DestinationCIDR,
		JumpPeerID:      req.JumpPeerID,
		DomainSuffix:    domainSuffix,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.routeRepo.CreateRoute(ctx, networkID, route); err != nil {
		return nil, fmt.Errorf("failed to create route: %w", err)
	}

	return route, nil
}

// GetRoute retrieves a route by ID
func (s *Service) GetRoute(ctx context.Context, networkID, routeID string) (*network.Route, error) {
	route, err := s.routeRepo.GetRoute(ctx, networkID, routeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get route: %w", err)
	}
	return route, nil
}

// UpdateRoute updates an existing route
func (s *Service) UpdateRoute(ctx context.Context, networkID, routeID string, req *network.RouteUpdateRequest) (*network.Route, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing route
	route, err := s.routeRepo.GetRoute(ctx, networkID, routeID)
	if err != nil {
		return nil, fmt.Errorf("route not found: %w", err)
	}

	// Update fields
	if req.Name != "" {
		route.Name = req.Name
	}
	if req.Description != "" {
		route.Description = req.Description
	}
	if req.DestinationCIDR != "" {
		route.DestinationCIDR = req.DestinationCIDR
	}
	if req.JumpPeerID != "" {
		// Verify new jump peer exists and is a jump peer
		jumpPeer, err := s.peerRepo.GetPeer(ctx, networkID, req.JumpPeerID)
		if err != nil {
			return nil, fmt.Errorf("jump peer not found: %w", err)
		}
		if !jumpPeer.IsJump {
			return nil, fmt.Errorf("peer is not a jump peer")
		}
		route.JumpPeerID = req.JumpPeerID
	}
	if req.DomainSuffix != "" {
		route.DomainSuffix = req.DomainSuffix
	}
	route.UpdatedAt = time.Now()

	if err := s.routeRepo.UpdateRoute(ctx, networkID, route); err != nil {
		return nil, fmt.Errorf("failed to update route: %w", err)
	}

	// Trigger WireGuard config regeneration for all peers in groups using this route
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return route, nil
}

// DeleteRoute deletes a route
func (s *Service) DeleteRoute(ctx context.Context, networkID, routeID string) error {
	// Verify route exists
	_, err := s.routeRepo.GetRoute(ctx, networkID, routeID)
	if err != nil {
		return fmt.Errorf("route not found: %w", err)
	}

	if err := s.routeRepo.DeleteRoute(ctx, networkID, routeID); err != nil {
		return fmt.Errorf("failed to delete route: %w", err)
	}

	// Trigger WireGuard config regeneration for all affected peers
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// ListRoutes lists all routes in a network
func (s *Service) ListRoutes(ctx context.Context, networkID string) ([]*network.Route, error) {
	// Verify network exists
	_, err := s.peerRepo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	routes, err := s.routeRepo.ListRoutes(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %w", err)
	}

	return routes, nil
}

// GetPeerRoutes calculates routes for a peer based on group membership
func (s *Service) GetPeerRoutes(ctx context.Context, networkID, peerID string) ([]*network.Route, error) {
	// Verify peer exists
	_, err := s.peerRepo.GetPeer(ctx, networkID, peerID)
	if err != nil {
		return nil, fmt.Errorf("peer not found: %w", err)
	}

	// Get all groups the peer belongs to
	groups, err := s.groupRepo.GetPeerGroups(ctx, networkID, peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer groups: %w", err)
	}

	// Collect all routes from all groups (deduplicate by route ID)
	routeMap := make(map[string]*network.Route)
	for _, group := range groups {
		routes, err := s.routeRepo.GetRoutesForGroup(ctx, networkID, group.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get routes for group %s: %w", group.ID, err)
		}
		for _, route := range routes {
			routeMap[route.ID] = route
		}
	}

	// Convert map to slice
	var routes []*network.Route
	for _, route := range routeMap {
		routes = append(routes, route)
	}

	return routes, nil
}
