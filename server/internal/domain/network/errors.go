package network

import "errors"

// Group errors
var (
	ErrGroupNotFound      = errors.New("group not found")
	ErrDuplicateGroupName = errors.New("group name already exists in network")
	ErrPeerNotInGroup     = errors.New("peer not in group")
)

// Policy errors
var (
	ErrPolicyNotFound      = errors.New("policy not found")
	ErrDuplicatePolicyName = errors.New("policy name already exists in network")
	ErrPolicyNotAttached   = errors.New("policy not attached to group")
)

// Route errors
var (
	ErrRouteNotFound        = errors.New("route not found")
	ErrDuplicateRouteName   = errors.New("route name already exists in network")
	ErrRouteNotAttached     = errors.New("route not attached to group")
	ErrInvalidCIDR          = errors.New("invalid CIDR format")
	ErrJumpPeerNotFound     = errors.New("jump peer not found")
	ErrNotJumpPeer          = errors.New("peer is not a jump peer")
	ErrCannotDeleteLastJump = errors.New("cannot delete route: jump peer is last in network")
)

// DNS errors
var (
	ErrDNSMappingNotFound = errors.New("DNS mapping not found")
	ErrDuplicateDNSName   = errors.New("DNS name already exists for route")
	ErrIPNotInRouteCIDR   = errors.New("IP address not in route CIDR")
)

// Network errors
var (
	ErrNetworkNotFound = errors.New("network not found")
)

// Peer errors
var (
	ErrPeerNotFound = errors.New("peer not found")
)

// Authorization errors
var (
	ErrUnauthorized = errors.New("unauthorized: admin privileges required")
)
