package group

import (
	"context"
	"testing"

	"wirety/internal/domain/network"

	"github.com/google/uuid"
)

// TestCircularRoutingValidation_AddJumpPeerToGroupWithRoute tests that adding a jump peer
// to a group that has routes using that jump peer is rejected
func TestCircularRoutingValidation_AddJumpPeerToGroupWithRoute(t *testing.T) {
	ctx := context.Background()
	networkID := uuid.New().String()
	groupID := uuid.New().String()
	jumpPeerID := uuid.New().String()
	routeID := uuid.New().String()

	// Setup mocks
	groupRepo := newMockGroupRepository()
	netGetter := newMockNetworkGetter()
	routeRepo := newMockRouteRepository()

	// Create network
	netGetter.networks[networkID] = &network.Network{
		ID:   networkID,
		Name: "test-network",
		CIDR: "10.0.0.0/24",
	}

	// Create jump peer
	jumpPeer := &network.Peer{
		ID:        jumpPeerID,
		Name:      "jump-peer",
		IsJump:    true,
		Address:   "10.0.0.1",
		PublicKey: "test-key",
	}
	netGetter.peers[jumpPeerID] = jumpPeer

	// Create group
	group := &network.Group{
		ID:        groupID,
		NetworkID: networkID,
		Name:      "test-group",
		PeerIDs:   []string{},
		RouteIDs:  []string{routeID},
	}
	groupRepo.groups[groupID] = group
	groupRepo.groupRoutes[groupID] = []string{routeID}

	// Create route that uses the jump peer
	route := &network.Route{
		ID:              routeID,
		NetworkID:       networkID,
		Name:            "test-route",
		DestinationCIDR: "192.168.0.0/24",
		JumpPeerID:      jumpPeerID,
	}
	routeRepo.routes[routeID] = route

	// Create service
	service := NewService(groupRepo, &networkGetterAdapter{getter: netGetter}, routeRepo)

	// Try to add jump peer to group - should fail
	err := service.AddPeerToGroup(ctx, networkID, groupID, jumpPeerID)

	// Verify error is CircularRoutingError
	if err == nil {
		t.Fatal("Expected error when adding jump peer to group with route using that peer, got nil")
	}

	var circularErr *CircularRoutingError
	if !isCircularRoutingError(err, &circularErr) {
		t.Fatalf("Expected CircularRoutingError, got: %v", err)
	}

	if circularErr.PeerID != jumpPeerID {
		t.Errorf("Expected peer ID %s, got %s", jumpPeerID, circularErr.PeerID)
	}

	if circularErr.GroupID != groupID {
		t.Errorf("Expected group ID %s, got %s", groupID, circularErr.GroupID)
	}

	if len(circularErr.RouteIDs) != 1 || circularErr.RouteIDs[0] != routeID {
		t.Errorf("Expected route IDs [%s], got %v", routeID, circularErr.RouteIDs)
	}
}

// TestCircularRoutingValidation_AttachRouteWithJumpPeerInGroup tests that attaching a route
// to a group that contains the route's jump peer is rejected
func TestCircularRoutingValidation_AttachRouteWithJumpPeerInGroup(t *testing.T) {
	ctx := context.Background()
	networkID := uuid.New().String()
	groupID := uuid.New().String()
	jumpPeerID := uuid.New().String()
	routeID := uuid.New().String()

	// Setup mocks
	groupRepo := newMockGroupRepository()
	netGetter := newMockNetworkGetter()
	routeRepo := newMockRouteRepository()

	// Create network
	netGetter.networks[networkID] = &network.Network{
		ID:   networkID,
		Name: "test-network",
		CIDR: "10.0.0.0/24",
	}

	// Create jump peer
	jumpPeer := &network.Peer{
		ID:        jumpPeerID,
		Name:      "jump-peer",
		IsJump:    true,
		Address:   "10.0.0.1",
		PublicKey: "test-key",
	}
	netGetter.peers[jumpPeerID] = jumpPeer

	// Create group with jump peer as member
	group := &network.Group{
		ID:        groupID,
		NetworkID: networkID,
		Name:      "test-group",
		PeerIDs:   []string{jumpPeerID},
		RouteIDs:  []string{},
	}
	groupRepo.groups[groupID] = group
	groupRepo.groupPeers[groupID] = []string{jumpPeerID}

	// Create route that uses the jump peer
	route := &network.Route{
		ID:              routeID,
		NetworkID:       networkID,
		Name:            "test-route",
		DestinationCIDR: "192.168.0.0/24",
		JumpPeerID:      jumpPeerID,
	}
	routeRepo.routes[routeID] = route

	// Create service
	service := NewService(groupRepo, &networkGetterAdapter{getter: netGetter}, routeRepo)

	// Try to attach route to group - should fail
	err := service.AttachRouteToGroup(ctx, networkID, groupID, routeID)

	// Verify error is CircularRoutingError
	if err == nil {
		t.Fatal("Expected error when attaching route to group containing the route's jump peer, got nil")
	}

	var circularErr *CircularRoutingError
	if !isCircularRoutingError(err, &circularErr) {
		t.Fatalf("Expected CircularRoutingError, got: %v", err)
	}

	if circularErr.PeerID != jumpPeerID {
		t.Errorf("Expected peer ID %s, got %s", jumpPeerID, circularErr.PeerID)
	}

	if circularErr.GroupID != groupID {
		t.Errorf("Expected group ID %s, got %s", groupID, circularErr.GroupID)
	}

	if len(circularErr.RouteIDs) != 1 || circularErr.RouteIDs[0] != routeID {
		t.Errorf("Expected route IDs [%s], got %v", routeID, circularErr.RouteIDs)
	}
}

// TestCircularRoutingValidation_AllowRegularPeer tests that adding a regular (non-jump) peer
// to a group with routes is allowed
func TestCircularRoutingValidation_AllowRegularPeer(t *testing.T) {
	ctx := context.Background()
	networkID := uuid.New().String()
	groupID := uuid.New().String()
	regularPeerID := uuid.New().String()
	jumpPeerID := uuid.New().String()
	routeID := uuid.New().String()

	// Setup mocks
	groupRepo := newMockGroupRepository()
	netGetter := newMockNetworkGetter()
	routeRepo := newMockRouteRepository()

	// Create network
	netGetter.networks[networkID] = &network.Network{
		ID:   networkID,
		Name: "test-network",
		CIDR: "10.0.0.0/24",
	}

	// Create regular peer (not a jump peer)
	regularPeer := &network.Peer{
		ID:        regularPeerID,
		Name:      "regular-peer",
		IsJump:    false,
		Address:   "10.0.0.2",
		PublicKey: "test-key",
	}
	netGetter.peers[regularPeerID] = regularPeer

	// Create jump peer
	jumpPeer := &network.Peer{
		ID:        jumpPeerID,
		Name:      "jump-peer",
		IsJump:    true,
		Address:   "10.0.0.1",
		PublicKey: "test-key-2",
	}
	netGetter.peers[jumpPeerID] = jumpPeer

	// Create group with route
	group := &network.Group{
		ID:        groupID,
		NetworkID: networkID,
		Name:      "test-group",
		PeerIDs:   []string{},
		RouteIDs:  []string{routeID},
	}
	groupRepo.groups[groupID] = group
	groupRepo.groupRoutes[groupID] = []string{routeID}

	// Create route that uses a different jump peer
	route := &network.Route{
		ID:              routeID,
		NetworkID:       networkID,
		Name:            "test-route",
		DestinationCIDR: "192.168.0.0/24",
		JumpPeerID:      jumpPeerID,
	}
	routeRepo.routes[routeID] = route

	// Create service
	service := NewService(groupRepo, &networkGetterAdapter{getter: netGetter}, routeRepo)

	// Try to add regular peer to group - should succeed
	err := service.AddPeerToGroup(ctx, networkID, groupID, regularPeerID)

	if err != nil {
		t.Fatalf("Expected no error when adding regular peer to group with routes, got: %v", err)
	}
}

// Helper function to check if error is CircularRoutingError
func isCircularRoutingError(err error, target **CircularRoutingError) bool {
	if err == nil {
		return false
	}
	if circErr, ok := err.(*CircularRoutingError); ok {
		*target = circErr
		return true
	}
	return false
}
