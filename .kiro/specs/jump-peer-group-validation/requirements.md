# Requirements Document

## Introduction

This specification addresses a critical validation gap in the network groups and routing system. Currently, the system allows a jump peer to be added to a group that has routes using that same jump peer as a gateway, creating a circular routing configuration. This results in the jump peer attempting to route traffic through itself, which is logically invalid and will cause routing failures.

This specification defines validation rules to prevent circular routing configurations by ensuring jump peers cannot be members of groups that use them as route gateways.

## Glossary

- **Jump Peer**: A peer configured to act as a gateway for routing traffic to external networks
- **Route**: A network destination (CIDR) that is routed through a specific jump peer
- **Group**: An organizational unit containing peers, with attached policies and routes
- **Circular Routing**: An invalid configuration where a jump peer would route traffic to itself

## Requirements

### Requirement 1: Jump Peer Group Membership Validation

**User Story:** As a network administrator, I want the system to prevent jump peers from being added to groups that use them as route gateways, so that I cannot create invalid circular routing configurations.

#### Acceptance Criteria

1. WHEN an administrator attempts to add a peer to a group THEN the System SHALL check if the peer is a jump peer for any routes attached to that group
2. WHEN an administrator attempts to add a jump peer to a group that has routes using that jump peer THEN the System SHALL reject the operation with HTTP status 400 and error message "Cannot add jump peer to group: peer is the gateway for routes attached to this group"
3. WHEN an administrator attempts to add a regular peer to a group THEN the System SHALL allow the operation regardless of the group's routes
4. WHEN an administrator attempts to add a peer to a group with no routes THEN the System SHALL allow the operation regardless of whether the peer is a jump peer

### Requirement 2: Route Attachment Validation

**User Story:** As a network administrator, I want the system to prevent routes from being attached to groups that contain the route's jump peer, so that I cannot create invalid circular routing configurations.

#### Acceptance Criteria

1. WHEN an administrator attempts to attach a route to a group THEN the System SHALL check if the route's jump peer is a member of that group
2. WHEN an administrator attempts to attach a route to a group that contains the route's jump peer THEN the System SHALL reject the operation with HTTP status 400 and error message "Cannot attach route to group: the route's jump peer is a member of this group"
3. WHEN an administrator attempts to attach a route to a group that does not contain the route's jump peer THEN the System SHALL allow the operation
4. WHEN an administrator attempts to attach a route to an empty group THEN the System SHALL allow the operation

### Requirement 3: Validation Error Reporting

**User Story:** As a network administrator, I want clear error messages when validation fails, so that I understand why my operation was rejected and how to fix it.

#### Acceptance Criteria

1. WHEN a validation error occurs THEN the System SHALL return an HTTP 400 status code
2. WHEN a validation error occurs THEN the System SHALL include a descriptive error message explaining the circular routing issue
3. WHEN a validation error occurs THEN the System SHALL include the peer ID and route IDs involved in the conflict
4. WHEN a validation error occurs in the API THEN the System SHALL log the validation failure with relevant context for debugging
