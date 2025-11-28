package group

import "fmt"

// CircularRoutingError represents a validation error when a circular routing configuration is detected
type CircularRoutingError struct {
	PeerID   string   // The jump peer involved in the conflict
	GroupID  string   // The group involved in the conflict
	RouteIDs []string // The routes involved in the conflict
	Message  string   // Human-readable error message
}

func (e *CircularRoutingError) Error() string {
	return fmt.Sprintf("%s (peer: %s, group: %s, routes: %v)",
		e.Message, e.PeerID, e.GroupID, e.RouteIDs)
}

// NewCircularRoutingErrorForPeer creates an error for when a jump peer cannot be added to a group
func NewCircularRoutingErrorForPeer(peerID, groupID string, routeIDs []string) *CircularRoutingError {
	return &CircularRoutingError{
		PeerID:   peerID,
		GroupID:  groupID,
		RouteIDs: routeIDs,
		Message:  "cannot add jump peer to group: peer is the gateway for routes attached to this group",
	}
}

// NewCircularRoutingErrorForRoute creates an error for when a route cannot be attached to a group
func NewCircularRoutingErrorForRoute(peerID, groupID, routeID string) *CircularRoutingError {
	return &CircularRoutingError{
		PeerID:   peerID,
		GroupID:  groupID,
		RouteIDs: []string{routeID},
		Message:  "cannot attach route to group: the route's jump peer is a member of this group",
	}
}
