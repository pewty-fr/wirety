package group

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

// Service implements the business logic for group management
type Service struct {
	groupRepo  network.GroupRepository
	peerRepo   network.Repository
	wsNotifier WebSocketNotifier
}

// NewService creates a new group service
func NewService(groupRepo network.GroupRepository, peerRepo network.Repository) *Service {
	return &Service{
		groupRepo: groupRepo,
		peerRepo:  peerRepo,
	}
}

// SetWebSocketNotifier sets the WebSocket notifier for the service
func (s *Service) SetWebSocketNotifier(notifier WebSocketNotifier) {
	s.wsNotifier = notifier
}

// CreateGroup creates a new group with name validation
func (s *Service) CreateGroup(ctx context.Context, networkID string, req *network.GroupCreateRequest) (*network.Group, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Verify network exists
	_, err := s.peerRepo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	now := time.Now()
	group := &network.Group{
		ID:          uuid.New().String(),
		NetworkID:   networkID,
		Name:        req.Name,
		Description: req.Description,
		PeerIDs:     []string{},
		PolicyIDs:   []string{},
		RouteIDs:    []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.groupRepo.CreateGroup(ctx, networkID, group); err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	return group, nil
}

// GetGroup retrieves a group by ID
func (s *Service) GetGroup(ctx context.Context, networkID, groupID string) (*network.Group, error) {
	group, err := s.groupRepo.GetGroup(ctx, networkID, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}
	return group, nil
}

// UpdateGroup updates an existing group
func (s *Service) UpdateGroup(ctx context.Context, networkID, groupID string, req *network.GroupUpdateRequest) (*network.Group, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get existing group
	group, err := s.groupRepo.GetGroup(ctx, networkID, groupID)
	if err != nil {
		return nil, fmt.Errorf("group not found: %w", err)
	}

	// Update fields
	if req.Name != "" {
		group.Name = req.Name
	}
	if req.Description != "" {
		group.Description = req.Description
	}
	group.UpdatedAt = time.Now()

	if err := s.groupRepo.UpdateGroup(ctx, networkID, group); err != nil {
		return nil, fmt.Errorf("failed to update group: %w", err)
	}

	return group, nil
}

// DeleteGroup deletes a group
func (s *Service) DeleteGroup(ctx context.Context, networkID, groupID string) error {
	// Verify group exists
	_, err := s.groupRepo.GetGroup(ctx, networkID, groupID)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	if err := s.groupRepo.DeleteGroup(ctx, networkID, groupID); err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	return nil
}

// ListGroups lists all groups in a network
func (s *Service) ListGroups(ctx context.Context, networkID string) ([]*network.Group, error) {
	// Verify network exists
	_, err := s.peerRepo.GetNetwork(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	groups, err := s.groupRepo.ListGroups(ctx, networkID)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	return groups, nil
}

// AddPeerToGroup adds a peer to a group with validation
func (s *Service) AddPeerToGroup(ctx context.Context, networkID, groupID, peerID string) error {
	// Verify peer exists and belongs to the network
	peer, err := s.peerRepo.GetPeer(ctx, networkID, peerID)
	if err != nil {
		return fmt.Errorf("peer not found: %w", err)
	}

	// Verify group exists
	_, err = s.groupRepo.GetGroup(ctx, networkID, groupID)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	// Add peer to group
	if err := s.groupRepo.AddPeerToGroup(ctx, networkID, groupID, peerID); err != nil {
		return fmt.Errorf("failed to add peer to group: %w", err)
	}

	// Trigger config regeneration for the peer
	if s.wsNotifier != nil && peer.UseAgent {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// RemovePeerFromGroup removes a peer from a group with validation
func (s *Service) RemovePeerFromGroup(ctx context.Context, networkID, groupID, peerID string) error {
	// Verify peer exists
	peer, err := s.peerRepo.GetPeer(ctx, networkID, peerID)
	if err != nil {
		return fmt.Errorf("peer not found: %w", err)
	}

	// Verify group exists
	_, err = s.groupRepo.GetGroup(ctx, networkID, groupID)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	// Remove peer from group
	if err := s.groupRepo.RemovePeerFromGroup(ctx, networkID, groupID, peerID); err != nil {
		return fmt.Errorf("failed to remove peer from group: %w", err)
	}

	// Trigger config regeneration for the peer
	if s.wsNotifier != nil && peer.UseAgent {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// AttachPolicyToGroup attaches a policy to a group with WebSocket notification
func (s *Service) AttachPolicyToGroup(ctx context.Context, networkID, groupID, policyID string) error {
	// Verify group exists
	_, err := s.groupRepo.GetGroup(ctx, networkID, groupID)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	// Attach policy to group
	if err := s.groupRepo.AttachPolicyToGroup(ctx, networkID, groupID, policyID); err != nil {
		return fmt.Errorf("failed to attach policy to group: %w", err)
	}

	// Notify all peers in the network about the policy change
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// DetachPolicyFromGroup detaches a policy from a group with WebSocket notification
func (s *Service) DetachPolicyFromGroup(ctx context.Context, networkID, groupID, policyID string) error {
	// Verify group exists
	_, err := s.groupRepo.GetGroup(ctx, networkID, groupID)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	// Detach policy from group
	if err := s.groupRepo.DetachPolicyFromGroup(ctx, networkID, groupID, policyID); err != nil {
		return fmt.Errorf("failed to detach policy from group: %w", err)
	}

	// Notify all peers in the network about the policy change
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// AttachRouteToGroup attaches a route to a group with config regeneration
func (s *Service) AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error {
	// Verify group exists
	_, err := s.groupRepo.GetGroup(ctx, networkID, groupID)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	// Attach route to group
	if err := s.groupRepo.AttachRouteToGroup(ctx, networkID, groupID, routeID); err != nil {
		return fmt.Errorf("failed to attach route to group: %w", err)
	}

	// Trigger WireGuard config regeneration for all peers in the group
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// DetachRouteFromGroup detaches a route from a group with config regeneration
func (s *Service) DetachRouteFromGroup(ctx context.Context, networkID, groupID, routeID string) error {
	// Verify group exists
	_, err := s.groupRepo.GetGroup(ctx, networkID, groupID)
	if err != nil {
		return fmt.Errorf("group not found: %w", err)
	}

	// Detach route from group
	if err := s.groupRepo.DetachRouteFromGroup(ctx, networkID, groupID, routeID); err != nil {
		return fmt.Errorf("failed to detach route from group: %w", err)
	}

	// Trigger WireGuard config regeneration for all peers in the group
	if s.wsNotifier != nil {
		s.wsNotifier.NotifyNetworkPeers(networkID)
	}

	return nil
}

// GetGroupPolicies retrieves all policies attached to a group
func (s *Service) GetGroupPolicies(ctx context.Context, networkID, groupID string) ([]*network.Policy, error) {
	// Verify group exists
	_, err := s.groupRepo.GetGroup(ctx, networkID, groupID)
	if err != nil {
		return nil, fmt.Errorf("group not found: %w", err)
	}

	// Get policies for the group
	policies, err := s.groupRepo.GetGroupPolicies(ctx, networkID, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group policies: %w", err)
	}

	return policies, nil
}
