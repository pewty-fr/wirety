# Design Document

## Overview

This design document specifies the implementation of validation logic to prevent circular routing configurations in the network groups and routing system. The validation ensures that jump peers cannot be members of groups that use them as route gateways, preventing invalid configurations where a peer would route traffic through itself.

The solution adds validation checks in two critical operations:
1. Adding a peer to a group (GroupService.AddPeerToGroup)
2. Attaching a route to a group (GroupService.AttachRouteToGroup)

## Architecture

The validation logic will be implemented in the application service layer (GroupService) where business rules are enforced. This follows the existing pattern of validation in the service layer before delegating to repositories.

```
┌─────────────────────────────────────────────────────────────┐
│                    API Layer (Handlers)                      │
│              POST /groups/{id}/peers/{peerId}                │
│              POST /groups/{id}/routes/{routeId}              │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────┐
│                  GroupService (NEW VALIDATION)               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ AddPeerToGroup:                                        │ │
│  │  1. Verify peer exists                                 │ │
│  │  2. Verify group exists                                │ │
│  │  3. Get group's routes                                 │ │
│  │  4. Check if peer is jump peer for any route ← NEW    │ │
│  │  5. Add peer to group                                  │ │
│  └────────────────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ AttachRouteToGroup:                                    │ │
│  │  1. Verify route exists                                │ │
│  │  2. Verify group exists                                │ │
│  │  3. Get group's members                                │ │
│  │  4. Check if route's jump peer is in group ← NEW      │ │
│  │  5. Attach route to group                              │ │
│  └────────────────────────────────────────────────────────┘ │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ↓
┌─────────────────────────────────────────────────────────────┐
│              Repository Layer (No Changes)                   │
│  GroupRepository │ RouteRepository │ PeerRepository          │
└─────────────────────────────────────────────────────────────┘
```

## Components and Interfaces

### Modified Service Methods

#### GroupService.AddPeerToGroup

**Current Implementation:**
```go
func (s *Service) AddPeerToGroup(ctx context.Context, networkID, groupID, peerID string) error {
    // Verify peer exists
    // Verify group exists
    // Add peer to group
    // Trigger config regeneration
}
```

**Enhanced Implementation:**
```go
func (s *Service) AddPeerToGroup(ctx context.Context, networkID, groupID, peerID string) error {
    // Verify peer exists
    peer, err := s.peerRepo.GetPeer(ctx, networkID, peerID)
    if err != nil {
        return fmt.Errorf("peer not found: %w", err)
    }

    // Verify group exists
    group, err := s.groupRepo.GetGroup(ctx, networkID, groupID)
    if err != nil {
        return fmt.Errorf("group not found: %w", err)
    }

    // NEW: Check for circular routing
    if peer.IsJump && len(group.RouteIDs) > 0 {
        // Get all routes attached to the group
        routes, err := s.routeRepo.GetRoutesForGroup(ctx, networkID, groupID)
        if err != nil {
            return fmt.Errorf("failed to get group routes: %w", err)
        }

        // Check if this peer is the jump peer for any of these routes
        conflictingRoutes := []string{}
        for _, route := range routes {
            if route.JumpPeerID == peerID {
                conflictingRoutes = append(conflictingRoutes, route.ID)
            }
        }

        if len(conflictingRoutes) > 0 {
            return &CircularRoutingError{
                PeerID:   peerID,
                GroupID:  groupID,
                RouteIDs: conflictingRoutes,
                Message:  "Cannot add jump peer to group: peer is the gateway for routes attached to this group",
            }
        }
    }

    // Add peer to group
    // Trigger config regeneration
}
```

#### GroupService.AttachRouteToGroup

**Current Implementation:**
```go
func (s *Service) AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error {
    // Verify route exists
    // Verify group exists
    // Attach route to group
    // Trigger config regeneration
}
```

**Enhanced Implementation:**
```go
func (s *Service) AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error {
    // Verify route exists
    route, err := s.routeRepo.GetRoute(ctx, networkID, routeID)
    if err != nil {
        return fmt.Errorf("route not found: %w", err)
    }

    // Verify group exists
    group, err := s.groupRepo.GetGroup(ctx, networkID, groupID)
    if err != nil {
        return fmt.Errorf("group not found: %w", err)
    }

    // NEW: Check for circular routing
    if len(group.PeerIDs) > 0 {
        // Check if the route's jump peer is a member of this group
        for _, peerID := range group.PeerIDs {
            if peerID == route.JumpPeerID {
                return &CircularRoutingError{
                    PeerID:   route.JumpPeerID,
                    GroupID:  groupID,
                    RouteIDs: []string{routeID},
                    Message:  "Cannot attach route to group: the route's jump peer is a member of this group",
                }
            }
        }
    }

    // Attach route to group
    // Trigger config regeneration
}
```

### Error Types

```go
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
```

### API Handler Updates

The API handlers need to detect CircularRoutingError and return HTTP 400 with appropriate error details:

```go
// In group handlers
func (h *Handler) AddPeerToGroup(c *gin.Context) {
    // ... existing code ...
    
    err := h.groupService.AddPeerToGroup(ctx, networkID, groupID, peerID)
    if err != nil {
        var circularErr *CircularRoutingError
        if errors.As(err, &circularErr) {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": circularErr.Message,
                "details": gin.H{
                    "peer_id":   circularErr.PeerID,
                    "group_id":  circularErr.GroupID,
                    "route_ids": circularErr.RouteIDs,
                },
            })
            return
        }
        // ... handle other errors ...
    }
    
    // ... success response ...
}
```

## Data Models

No changes to existing data models are required. The validation uses existing fields:
- `Peer.IsJump` - identifies jump peers
- `Group.PeerIDs` - list of peer members
- `Group.RouteIDs` - list of attached routes
- `Route.JumpPeerID` - the jump peer for a route

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*


Property 1: Jump peer rejection when used by group routes
*For any* group with attached routes and any jump peer that is the gateway for one of those routes, attempting to add that jump peer to the group should fail with a CircularRoutingError
**Validates: Requirements 1.2**

Property 2: Regular peer acceptance regardless of routes
*For any* group with attached routes and any non-jump peer, attempting to add that peer to the group should succeed
**Validates: Requirements 1.3**

Property 3: Peer acceptance for groups without routes
*For any* peer (jump or non-jump) and any group with no attached routes, attempting to add that peer to the group should succeed
**Validates: Requirements 1.4**

Property 4: Route rejection when jump peer is group member
*For any* group with members and any route whose jump peer is a member of that group, attempting to attach that route to the group should fail with a CircularRoutingError
**Validates: Requirements 2.2**

Property 5: Route acceptance when jump peer is not group member
*For any* group with members and any route whose jump peer is not a member of that group, attempting to attach that route to the group should succeed
**Validates: Requirements 2.3**

Property 6: Route acceptance for empty groups
*For any* route and any group with no members, attempting to attach that route to the group should succeed
**Validates: Requirements 2.4**

Property 7: Error structure completeness
*For any* CircularRoutingError, the error should contain non-empty peer_id, group_id, and route_ids fields
**Validates: Requirements 3.3**

## Error Handling

### Validation Errors

The system introduces a new error type `CircularRoutingError` to represent circular routing validation failures. This error:
- Implements the standard Go `error` interface
- Contains structured data about the conflict (peer ID, group ID, route IDs)
- Provides a human-readable message
- Can be detected using `errors.As()` for proper HTTP status code mapping

### Error Propagation

1. **Service Layer**: Returns `CircularRoutingError` when validation fails
2. **API Layer**: Detects `CircularRoutingError` and returns HTTP 400 with structured JSON response
3. **Logging**: Logs validation failures with full context for debugging

### Error Response Format

```json
{
  "error": "Cannot add jump peer to group: peer is the gateway for routes attached to this group",
  "details": {
    "peer_id": "peer-123",
    "group_id": "group-456",
    "route_ids": ["route-789", "route-012"]
  }
}
```

## Testing Strategy

### Unit Testing

Unit tests will verify:
- CircularRoutingError is returned when validation fails
- Validation is skipped when not applicable (non-jump peers, empty groups, etc.)
- Error messages contain correct information
- HTTP handlers map errors to correct status codes

### Property-Based Testing

Property-based tests will use the `pgregory/rapid` library to:
- Generate random groups, peers, and routes
- Test all validation scenarios across many random inputs
- Verify that circular routing is always prevented
- Verify that valid operations always succeed
- Run a minimum of 100 iterations per property

Each property-based test will be tagged with a comment referencing the design document:
```go
// Feature: jump-peer-group-validation, Property 1: Jump peer rejection when used by group routes
```

### Integration Testing

Integration tests will verify:
- End-to-end API behavior with real database
- Error responses match expected format
- Validation works across service boundaries

## Implementation Notes

### Performance Considerations

The validation requires additional database queries:
- `AddPeerToGroup`: Fetches group's routes (only if peer is a jump peer)
- `AttachRouteToGroup`: Checks if jump peer is in group's member list (in-memory check)

These queries are lightweight and only executed when necessary. The performance impact is minimal compared to the config regeneration that follows these operations.

### Backward Compatibility

This change adds new validation that may reject operations that were previously allowed. However:
- Existing valid configurations are unaffected
- Only invalid (circular routing) configurations are rejected
- This is a bug fix, not a breaking change to correct behavior

### Database Considerations

No database schema changes are required. The validation uses existing relationships and fields.
