# Design Document

## Overview

This design document specifies the architecture for enhancing the WireGuard network management system with groups, policies, routing, and DNS capabilities. The system currently manages peers individually with basic isolation flags and a separate ACL system. This redesign introduces:

- **Groups**: Organizational units for peers with admin-only management
- **Policies**: iptables-based traffic filtering rules applied on jump peers, attached to groups
- **Routes**: Network destinations added to AllowedIPs in WireGuard configurations, attached to groups and routed through specific jump peers
- **DNS**: Enhanced internal DNS with route-based domain resolution
- **Default Groups**: Automatic group assignment for non-admin created peers

The system follows hexagonal architecture principles with clear separation between domain logic, application services, and infrastructure adapters.

## Architecture

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Frontend (React)                        │
│                   API Client / UI Components                 │
└──────────────────────────┬──────────────────────────────────┘
                           │ HTTP/REST
┌──────────────────────────┴──────────────────────────────────┐
│                    API Layer (Gin Handlers)                  │
│  Groups │ Policies │ Routes │ DNS │ Networks │ Peers │ Auth │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────┴──────────────────────────────────┐
│                  Application Services                        │
│  NetworkService │ GroupService │ PolicyService │ RouteService│
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────┴──────────────────────────────────┐
│                     Domain Layer                             │
│  Group │ Policy │ Route │ DNSMapping │ Network │ Peer       │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────┴──────────────────────────────────┐
│                  Repository Layer                            │
│              PostgreSQL Adapters                             │
└──────────────────────────┬──────────────────────────────────┘
                           │
┌──────────────────────────┴──────────────────────────────────┐
│                    Infrastructure                            │
│  Database │ WebSocket │ WireGuard Config Generation         │
└─────────────────────────────────────────────────────────────┘
```

### Agent Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    WireGuard Agent                           │
│  WebSocket Client │ Config Writer │ DNS Server │ Firewall   │
└──────────────────────────┬──────────────────────────────────┘
                           │ WebSocket
┌──────────────────────────┴──────────────────────────────────┐
│                      Server                                  │
│              WebSocket Manager                               │
└─────────────────────────────────────────────────────────────┘
```

## Components and Interfaces

### Domain Models

#### Group
```go
type Group struct {
    ID          string    `json:"id"`
    NetworkID   string    `json:"network_id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    PeerIDs     []string  `json:"peer_ids"`      // Member peer identifiers
    PolicyIDs   []string  `json:"policy_ids"`    // Attached policy identifiers
    RouteIDs    []string  `json:"route_ids"`     // Attached route identifiers
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type GroupCreateRequest struct {
    Name        string `json:"name" binding:"required"`
    Description string `json:"description"`
}

type GroupUpdateRequest struct {
    Name        string `json:"name,omitempty"`
    Description string `json:"description,omitempty"`
}
```

#### Policy
```go
type Policy struct {
    ID          string       `json:"id"`
    NetworkID   string       `json:"network_id"`
    Name        string       `json:"name"`
    Description string       `json:"description"`
    Rules       []PolicyRule `json:"rules"`
    CreatedAtme.Time    `json:"created_at"`
    UpdatedAt   time.Time    `json:"updated_at"`
}

type PolicyRule struct {
    ID          string `json:"id"`
    Direction   string `json:"direction"`   // "input" or "output"
    Action      string `json:"action"`      // "allow" or "deny"
    Target      string `json:"target"`      // IP/CIDR, peer ID, or group ID
    TargetType  string `json:"target_type"` // "cidr", "peer", "group"
    Description string `json:"description"`
}

type PolicyCreateRequest struct {
    Name        string       `json:"name" binding:"required"`
    Description string       `json:"description"`
    Rules       []PolicyRule `json:"rules"`
}

type PolicyUpdateRequest struct {
    Name        string       `json:"name,omitempty"`
    Description string       `json:"description,omitempty"`
}
```

#### Route
```go
type Route struct {
    ID              string    `json:"id"`
    NetworkID       string    `json:"network_id"`
    Name            string    `json:"name"`
    Description     string    `json:"description"`
    DestinationCIDR string    `json:"destination_cidr"` // External network range
    JumpPeerID      string    `json:"jump_peer_id"`     // Gateway jump peer
    DomainSuffix    string    `json:"domain_suffix"`    // Custom domain (default: .internal)
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

type RouteCreateRequest struct {
    Name            string `json:"name" binding:"required"`
    Description     string `json:"description"`
    DestinationCIDR string `json:"destination_cidr" binding:"required"`
    JumpPeerID      string `json:"jump_peer_id" binding:"required"`
    DomainSuffix    string `json:"domain_suffix"`
}

type RouteUpdateRequest struct {
    Name            string `json:"name,omitempty"`
    Description     string `json:"description,omitempty"`
    DestinationCIDR string `json:"destination_cidr,omitempty"`
    JumpPeerID      string `json:"jump_peer_id,omitempty"`
    DomainSuffix    string `json:"domain_suffix,omitempty"`
}
```

#### DNSMapping
```go
type DNSMapping struct {
    ID        string    `json:"id"`
    RouteID   string    `json:"route_id"`
    Name      string    `json:"name"`        // DNS name (e.g., "server1")
    IPAddress string    `json:"ip_address"`  // IP within route's CIDR
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// GetFQDN returns the fully qualified domain name
// Format: name.route_name.domain_suffix
func (d *DNSMapping) GetFQDN(route *Route) string {
    suffix := route.DomainSuffix
    if suffix == "" {
        suffix = "internal"
    }
    return fmt.Sprintf("%s.%s.%s", d.Name, route.Name, suffix)
}

type DNSMappingCreateRequest struct {
    Name      string `json:"name" binding:"required"`
    IPAddress string `json:"ip_address" binding:"required"`
}

type DNSMappingUpdateRequest struct {
    Name      string `json:"name,omitempty"`
    IPAddress string `json:"ip_address,omitempty"`
}
```

#### Network (Updated)
```go
type Network struct {
    ID                string           `json:"id"`
    Name              string           `json:"name"`
    CIDR              string           `json:"cidr"`
    Peers             map[string]*Peer `json:"-"`
    PeerCount         int              `json:"peer_count"`
    DNS               []string         `json:"dns"`
    DomainSuffix      string           `json:"domain_suffix"`      // Custom domain (default: .internal)
    DefaultGroupIDs   []string         `json:"default_group_ids"`  // Groups for non-admin peers
    CreatedAt         time.Time        `json:"created_at"`
    UpdatedAt         time.Time        `json:"updated_at"`
}
```

#### Peer (Updated - Remove Legacy Fields)
```go
type Peer struct {
    ID                   string    `json:"id"`
    Name                 string    `json:"name"`
    PublicKey            string    `json:"public_key"`
    PrivateKey           string    `json:"-"`
    Address              string    `json:"address"`
    Endpoint             string    `json:"endpoint,omitempty"`
    ListenPort           int       `json:"listen_port,omitempty"`
    AdditionalAllowedIPs []string  `json:"additional_allowed_ips,omitempty"`
    Token                string    `json:"token,omitempty"`
    IsJump               bool      `json:"is_jump"`
    UseAgent             bool      `json:"use_agent"`
    OwnerID              string    `json:"owner_id,omitempty"`
    GroupIDs             []string  `json:"group_ids"`  // Groups this peer belongs to
    CreatedAt            time.Time `json:"created_at"`
    UpdatedAt            time.Time `json:"updated_at"`
    
    // REMOVED: IsIsolated, FullEncapsulation (replaced by policies)
}
```

### Repository Interfaces

#### GroupRepository
```go
type GroupRepository interface {
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
    
    // Route attachment operations
    AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error
    DetachRouteFromGroup(ctx context.Context, networkID, groupID, routeID string) error
    GetGroupRoutes(ctx context.Context, networkID, groupID string) ([]*Route, error)
}
```

#### PolicyRepository
```go
type PolicyRepository interface {
    CreatePolicy(ctx context.Context, networkID string, policy *Policy) error
    GetPolicy(ctx context.Context, networkID, policyID string) (*Policy, error)
    UpdatePolicy(ctx context.Context, networkID string, policy *Policy) error
    DeletePolicy(ctx context.Context, networkID, policyID string) error
    ListPolicies(ctx context.Context, networkID string) ([]*Policy, error)
    
    // Rule operations
    AddRuleToPolicy(ctx context.Context, networkID, policyID string, rule *PolicyRule) error
    RemoveRuleFromPolicy(ctx context.Context, networkID, policyID, ruleID string) error
    UpdateRule(ctx context.Context, networkID, policyID string, rule *PolicyRule) error
    
    // Get policies for a specific group
    GetPoliciesForGroup(ctx context.Context, networkID, groupID string) ([]*Policy, error)
}
```

#### RouteRepository
```go
type RouteRepository interface {
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
```

#### DNSRepository
```go
type DNSRepository interface {
    CreateDNSMapping(ctx context.Context, routeID string, mapping *DNSMapping) error
    GetDNSMapping(ctx context.Context, routeID, mappingID string) (*DNSMapping, error)
    UpdateDNSMapping(ctx context.Context, routeID string, mapping *DNSMapping) error
    DeleteDNSMapping(ctx context.Context, routeID, mappingID string) error
    ListDNSMappings(ctx context.Context, routeID string) ([]*DNSMapping, error)
    
    // Get all DNS mappings for a network (for DNS server configuration)
    GetNetworkDNSMappings(ctx context.Context, networkID string) ([]*DNSMapping, error)
}
```

### Application Services

#### GroupService
```go
type GroupService struct {
    groupRepo  GroupRepository
    peerRepo   network.Repository
    wsNotifier WebSocketNotifier
}

func (s *GroupService) CreateGroup(ctx context.Context, networkID string, req *GroupCreateRequest) (*Group, error)
func (s *GroupService) GetGroup(ctx context.Context, networkID, groupID string) (*Group, error)
func (s *GroupService) UpdateGroup(ctx context.Context, networkID, groupID string, req *GroupUpdateRequest) (*Group, error)
func (s *GroupService) DeleteGroup(ctx context.Context, networkID, groupID string) error
func (s *GroupService) ListGroups(ctx context.Context, networkID string) ([]*Group, error)

func (s *GroupService) AddPeerToGroup(ctx context.Context, networkID, groupID, peerID string) error
func (s *GroupService) RemovePeerFromGroup(ctx context.Context, networkID, groupID, peerID string) error

func (s *GroupService) AttachPolicyToGroup(ctx context.Context, networkID, groupID, policyID string) error
func (s *GroupService) DetachPolicyFromGroup(ctx context.Context, networkID, groupID, policyID string) error

func (s *GroupService) AttachRouteToGroup(ctx context.Context, networkID, groupID, routeID string) error
func (s *GroupService) DetachRouteFromGroup(ctx context.Context, networkID, groupID, routeID string) error
```

#### PolicyService
```go
type PolicyService struct {
    policyRepo PolicyRepository
    groupRepo  GroupRepository
    wsNotifier WebSocketNotifier
}

func (s *PolicyService) CreatePolicy(ctx context.Context, networkID string, req *PolicyCreateRequest) (*Policy, error)
func (s *PolicyService) GetPolicy(ctx context.Context, networkID, policyID string) (*Policy, error)
func (s *PolicyService) UpdatePolicy(ctx context.Context, networkID, policyID string, req *PolicyUpdateRequest) (*Policy, error)
func (s *PolicyService) DeletePolicy(ctx context.Context, networkID, policyID string) error
func (s *PolicyService) ListPolicies(ctx context.Context, networkID string) ([]*Policy, error)

func (s *PolicyService) AddRuleToPolicy(ctx context.Context, networkID, policyID string, rule *PolicyRule) error
func (s *PolicyService) RemoveRuleFromPolicy(ctx context.Context, networkID, policyID, ruleID string) error

// Get default policy templates
func (s *PolicyService) GetDefaultTemplates() []PolicyTemplate

// Generate iptables rules for a jump peer based on all policies affecting it
func (s *PolicyService) GenerateIPTablesRules(ctx context.Context, networkID, jumpPeerID string) ([]string, error)
```

#### RouteService
```go
type RouteService struct {
    routeRepo  RouteRepository
    groupRepo  GroupRepository
    peerRepo   network.Repository
    wsNotifier WebSocketNotifier
}

func (s *RouteService) CreateRoute(ctx context.Context, networkID string, req *RouteCreateRequest) (*Route, error)
func (s *RouteService) GetRoute(ctx context.Context, networkID, routeID string) (*Route, error)
func (s *RouteService) UpdateRoute(ctx context.Context, networkID, routeID string, req *RouteUpdateRequest) (*Route, error)
func (s *RouteService) DeleteRoute(ctx context.Context, networkID, routeID string) error
func (s *RouteService) ListRoutes(ctx context.Context, networkID string) ([]*Route, error)

// Get routes that should be in a peer's AllowedIPs based on group membership
func (s *RouteService) GetPeerRoutes(ctx context.Context, networkID, peerID string) ([]*Route, error)
```

#### DNSService
```go
type DNSService struct {
    dnsRepo    DNSRepository
    routeRepo  RouteRepository
    wsNotifier WebSocketNotifier
}

func (s *DNSService) CreateDNSMapping(ctx context.Context, routeID string, req *DNSMappingCreateRequest) (*DNSMapping, error)
func (s *DNSService) GetDNSMapping(ctx context.Context, routeID, mappingID string) (*DNSMapping, error)
func (s *DNSService) UpdateDNSMapping(ctx context.Context, routeID, mappingID string, req *DNSMappingUpdateRequest) (*DNSMapping, error)
func (s *DNSService) DeleteDNSMapping(ctx context.Context, routeID, mappingID string) error
func (s *DNSService) ListDNSMappings(ctx context.Context, routeID string) ([]*DNSMapping, error)

// Get all DNS records for a network (peer records + route records)
func (s *DNSService) GetNetworkDNSRecords(ctx context.Context, networkID string) ([]DNSRecord, error)
```

## Data Models

### Database Schema

#### groups table
```sql
CREATE TABLE groups (
    id UUID PRIMARY KEY,
    network_id UUID NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(network_id, name)
);

CREATE INDEX idx_groups_network_id ON groups(network_id);
```

#### group_peers table (many-to-many)
```sql
CREATE TABLE group_peers (
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    peer_id UUID NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    added_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, peer_id)
);

CREATE INDEX idx_group_peers_peer_id ON group_peers(peer_id);
```

#### policies table
```sql
CREATE TABLE policies (
    id UUID PRIMARY KEY,
    network_id UUID NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(network_id, name)
);

CREATE INDEX idx_policies_network_id ON policies(network_id);
```

#### policy_rules table
```sql
CREATE TABLE policy_rules (
    id UUID PRIMARY KEY,
    policy_id UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    direction VARCHAR(10) NOT NULL CHECK (direction IN ('input', 'output')),
    action VARCHAR(10) NOT NULL CHECK (action IN ('allow', 'deny')),
    target TEXT NOT NULL,
    target_type VARCHAR(10) NOT NULL CHECK (target_type IN ('cidr', 'peer', 'group')),
    description TEXT,
    rule_order INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_policy_rules_policy_id ON policy_rules(policy_id);
```

#### group_policies table (many-to-many)
```sql
CREATE TABLE group_policies (
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    policy_id UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    attached_at TIMESTAMP NOT NULL DEFAULT NOW(),
    policy_order INT NOT NULL,
    PRIMARY KEY (group_id, policy_id)
);

CREATE INDEX idx_group_policies_policy_id ON group_policies(policy_id);
```

#### routes table
```sql
CREATE TABLE routes (
    id UUID PRIMARY KEY,
    network_id UUID NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    destination_cidr VARCHAR(50) NOT NULL,
    jump_peer_id UUID NOT NULL REFERENCES peers(id) ON DELETE RESTRICT,
    domain_suffix VARCHAR(255) NOT NULL DEFAULT 'internal',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(network_id, name)
);

CREATE INDEX idx_routes_network_id ON routes(network_id);
CREATE INDEX idx_routes_jump_peer_id ON routes(jump_peer_id);
```

#### group_routes table (many-to-many)
```sql
CREATE TABLE group_routes (
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    attached_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, route_id)
);

CREATE INDEX idx_group_routes_route_id ON group_routes(route_id);
```

#### dns_mappings table
```sql
CREATE TABLE dns_mappings (
    id UUID PRIMARY KEY,
    route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    ip_address VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(route_id, name)
);

CREATE INDEX idx_dns_mappings_route_id ON dns_mappings(route_id);
```

#### networks table (updated)
```sql
ALTER TABLE networks 
    ADD COLUMN domain_suffix VARCHAR(255) NOT NULL DEFAULT 'internal';

CREATE TABLE network_default_groups (
    network_id UUID NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    added_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (network_id, group_id)
);
```

#### peers table (updated - remove legacy columns)
```sql
ALTER TABLE peers 
    DROP COLUMN is_isolated,
    DROP COLUMN full_encapsulation;

-- ACL table will be dropped entirely
DROP TABLE IF EXISTS acl_rules;
DROP TABLE IF EXISTS acls;
```

## Error Handling

### Error Types

```go
var (
    ErrGroupNotFound        = errors.New("group not found")
    ErrPolicyNotFound       = errors.New("policy not found")
    ErrRouteNotFound        = errors.New("route not found")
    ErrDNSMappingNotFound   = errors.New("DNS mapping not found")
    ErrPeerNotInGroup       = errors.New("peer not in group")
    ErrPolicyNotAttached    = errors.New("policy not attached to group")
    ErrRouteNotAttached     = errors.New("route not attached to group")
    ErrInvalidCIDR          = errors.New("invalid CIDR format")
    ErrIPNotInRouteCIDR     = errors.New("IP address not in route CIDR")
    ErrJumpPeerNotFound     = errors.New("jump peer not found")
    ErrNotJumpPeer          = errors.New("peer is not a jump peer")
    ErrDuplicateGroupName   = errors.New("group name already exists in network")
    ErrDuplicatePolicyName  = errors.New("policy name already exists in network")
    ErrDuplicateRouteName   = errors.New("route name already exists in network")
    ErrDuplicateDNSName     = errors.New("DNS name already exists for route")
    ErrCannotDeleteLastJump = errors.New("cannot delete route: jump peer is last in network")
    ErrUnauthorized         = errors.New("unauthorized: admin privileges required")
)
```

### Error Handling Strategy

1. **Validation Errors**: Return HTTP 400 with descriptive error message
2. **Not Found Errors**: Return HTTP 404 with entity type and ID
3. **Authorization Errors**: Return HTTP 403 with permission requirement
4. **Conflict Errors**: Return HTTP 409 with conflict details
5. **Internal Errors**: Return HTTP 500 with generic message, log details

## Testing Strategy

### Unit Testing

Unit tests will verify specific behaviors and edge cases:

- Group CRUD operations
- Policy rule validation
- Route CIDR validation
- DNS mapping IP validation
- Default group assignment logic
- Authorization checks for admin-only operations

### Property-Based Testing

Property-based tests will verify universal correctness properties using a Go PBT library (e.g., `gopter` or `rapid`). Each test will run a minimum of 100 iterations.



## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Group creation completeness
*For any* valid group creation request with name and description, creating the group should result in a group with a unique ID, the provided name and description, and the correct network association.
**Validates: Requirements 1.1**

### Property 2: Peer-group association preservation
*For any* peer and group in the same network, adding the peer to the group should result in the peer appearing in the group's member list and the peer continuing to exist as an independent entity.
**Validates: Requirements 1.2**

### Property 3: Peer removal non-destructiveness
*For any* peer that is a member of a group, removing the peer from the group should remove the association but the peer should still exist in the network.
**Validates: Requirements 1.3**

### Property 4: Group deletion peer preservation
*For any* group with member peers, deleting the group should remove all peer-group associations but all peers should continue to exist in the network.
**Validates: Requirements 1.4**

### Property 5: Group listing completeness
*For any* network, listing groups should return all groups belonging to that network with accurate member peer counts matching actual membership.
**Validates: Requirements 1.5**

### Property 6: Group retrieval accuracy
*For any* existing group, retrieving the group should return complete data including all member peer identifiers.
**Validates: Requirements 1.6**

### Property 7: Group operation authorization
*For any* non-administrator user and any group create, update, or delete operation, the system should reject the request with HTTP 403.
**Validates: Requirements 1.7**

### Property 8: Policy creation completeness
*For any* valid policy creation request with name and description, creating the policy should result in a policy with a unique ID, the provided name and description, and the correct network association.
**Validates: Requirements 2.1**

### Property 9: Policy rule validation
*For any* policy rule, the system should validate that it contains a valid direction (input or output), action (allow or deny), and target (valid CIDR, peer ID, or group ID).
**Validates: Requirements 2.2**

### Property 10: Policy rule addition
*For any* policy and valid rule, adding the rule should increase the policy's rule count by one and the rule should appear in the policy's rule list.
**Validates: Requirements 2.3**

### Property 11: Policy rule removal
*For any* policy with at least one rule, removing a rule should decrease the rule count by one and the rule should no longer appear in the policy's rule list.
**Validates: Requirements 2.4**

### Property 12: Policy update propagation
*For any* policy attached to one or more groups, updating the policy should trigger configuration regeneration for all affected jump peers.
**Validates: Requirements 2.5**

### Property 13: Policy deletion cleanup
*For any* policy attached to groups, deleting the policy should remove all group-policy associations and trigger configuration updates for affected jump peers.
**Validates: Requirements 2.6**

### Property 14: Policy operation authorization
*For any* non-administrator user and any policy create, update, or delete operation, the system should reject the request with HTTP 403.
**Validates: Requirements 2.7**

### Property 15: Template policy independence
*For any* policy created from a template, modifyingcy should not affect the original template.
**Validates: Requirements 3.5**

### Property 16: Template access authorization
*For any* non-administrator user attempting to access policy templates, the system should reject the request with HTTP 403.
**Validates: Requirements 3.6**

### Property 17: Policy attachment application
*For any* policy and group, attaching the policy to the group should result in iptables rules being generated for all jump peers that route traffic for group members.
**Validates: Requirements 4.1**

### Property 18: Multiple policy ordering
*For any* group with multiple attached policies, the iptables rules should be applied in the order the policies were attached.
**Validates: Requirements 4.2**

### Property 19: Policy detachment cleanup
*For any* policy attached to a group, detaching the policy should remove the policy's iptables rules from all affected jump peers.
**Validates: Requirements 4.3**

### Property 20: Automatic policy application on join
*For any* peer joining a group that has attached policies, the group's policy iptables rules should be automatically applied on all jump peers for that peer.
**Validates: Requirements 4.4**

### Property 21: Automatic policy removal on leave
*For any* peer leaving a group that has attached policies, the group's policy iptables rules should be automatically removed from all jump peers for that peer.
**Validates: Requirements 4.5**

### Property 22: Policy attachment authorization
*For any* non-administrator user attempting to attach or detach policies, the system should reject the request with HTTP 403.
**Validates: Requirements 4.6**

### Property 23: Policy-only access control
*For any* peer communication attempt, the system should use only policy rules (not legacy ACL or peer flags) for access control decisions.
**Validates: Requirements 5.1**

### Property 24: Deny rule enforcement
*For any* traffic flow matching a deny policy rule, the system should block the traffic.
**Validates: Requirements 5.2**

### Property 25: Allow rule enforcement
*For any* traffic flow matching an allow policy rule, the system should permit the traffic.
**Validates: Requirements 5.3**

### Property 26: Default deny behavior
*For any* traffic flow that does not match any policy rules, the system should deny the traffic by default.
**Validates: Requirements 5.4**

### Property 27: Route creation completeness
*For any* valid route creation request with name, destination CIDR, and jump peer ID, creating the route should result in a route with a unique ID and all provided properties.
**Validates: Requirements 6.1**

### Property 28: Route CIDR validation
*For any* route creation request with an invalid CIDR format, the system should reject the request with a validation error.
**Validates: Requirements 6.2**

### Property 29: Route jump peer validation
*For any* route creation request, the system should validate that the specified jump peer exists and has IsJump set to true.
**Validates: Requirements 6.3**

### Property 30: Route attachment to group
*For any* route and group, attaching the route to the group should add the route's destination CIDR to the AllowedIPs configuration for all group members.
**Validates: Requirements 6.4**

### Property 31: Route detachment from group
*For any* route attached to a group, detaching the route should remove the route's destination CIDR from the AllowedIPs configuration for all group members.
**Validates: Requirements 6.5**

### Property 32: Automatic route addition on join
*For any* peer joining a group with attached routes, all route destination CIDRs should be automatically added to the peer's AllowedIPs configuration.
**Validates: Requirements 6.6**

### Property 33: Automatic route removal on leave
*For any* peer leaving a group with attached routes, the group's route destination CIDRs should be removed from the peer's AllowedIPs configuration.
**Validates: Requirements 6.7**

### Property 34: Route update propagation
*For any* route attached to one or more groups, updating the route should trigger WireGuard configuration regeneration for all peers in those groups.
**Validates: Requirements 6.8**

### Property 35: Route deletion cleanup
*For any* route attached to groups, deleting the route should remove all group-route associations and update all affected peer WireGuard configurations.
**Validates: Requirements 6.9**

### Property 36: Route listing completeness
*For any* network, listing routes should return all routes for that network with accurate jump peer associations.
**Validates: Requirements 6.10**

### Property 37: Route operation authorization
*For any* non-administrator user and any route create, update, or delete operation, the system should reject the request with HTTP 403.
**Validates: Requirements 6.11**

### Property 38: Default group persistence
*For any* network and group, configuring the group as a default group should persist the association in the database.
**Validates: Requirements 7.1**

### Property 39: Non-admin peer auto-assignment
*For any* peer created by a non-administrator user, the peer should be automatically added to all configured default groups for that network.
**Validates: Requirements 7.2**

### Property 40: Admin peer no auto-assignment
*For any* peer created by an administrator user, the peer should not be automatically added to default groups.
**Validates: Requirements 7.3**

### Property 41: Default group removal non-destructive
*For any* group configured as a default group, removing the default group configuration should not affect existing peer memberships in that group.
**Validates: Requirements 7.4**

### Property 42: Default group listing accuracy
*For any* network, listing default groups should return all groups configured as default for that network.
**Validates: Requirements 7.5**

### Property 43: Default group configuration authorization
*For any* non-administrator user attempting to configure default groups, the system should reject the request with HTTP 403.
**Validates: Requirements 7.6**

### Property 44: Route domain association
*For any* route created with a domain name, the domain name should be stored and associated with the route.
**Validates: Requirements 8.1**

### Property 45: DNS mapping IP validation
*For any* DNS mapping creation request, the system should validate that the IP address is within the route's destination CIDR.
**Validates: Requirements 8.2**

### Property 46: DNS mapping FQDN format
*For any* DNS mapping with name N for route R with domain suffix S, the FQDN should be formatted as N.R.S.
**Validates: Requirements 8.3**

### Property 47: DNS mapping authorization
*For any* non-administrator user attempting to create, update, or delete DNS mappings, the system should reject the request with HTTP 403.
**Validates: Requirements 8.6**

### Property 48: Custom network domain application
*For any* network with a custom domain suffix, all peer DNS records in that network should use the custom suffix instead of the default .internal.
**Validates: Requirements 9.2**

### Property 49: Custom route domain application
*For any* route with a custom domain suffix, all DNS mappings for that route should use the custom suffix instead of the default .internal.
**Validates: Requirements 9.3**

### Property 50: Domain suffix update propagation
*For any* network or route with a domain suffix change, all affected DNS records should be regenerated with the new suffix.
**Validates: Requirements 9.4**

### Property 51: Peer DNS record format
*For any* peer with name P in network N with domain suffix S, the DNS record should be formatted as P.N.S.
**Validates: Requirements 9.5**

### Property 52: Domain suffix modification authorization
*For any* non-administrator user attempting to modify domain suffixes, the system should reject the request with HTTP 403.
**Validates: Requirements 9.6**

### Property 53: DNS server initialization completeness
*For any* jump peer DNS server starting, the server should load all peer DNS records and all route DNS records for the network.
**Validates: Requirements 10.1**

### Property 54: Route DNS query resolution
*For any* DNS query matching a route domain pattern, the DNS server should return the IP address associated with the route DNS mapping.
**Validates: Requirements 10.2**

### Property 55: Peer DNS query resolution
*For any* DNS query matching a peer domain pattern, the DNS server should return the peer's IP address.
**Validates: Requirements 10.3**

### Property 56: WireGuard config route inclusion
*For any* regular peer that is a member of groups with attached routes, the peer's WireGuard configuration should include AllowedIPs entries for all route destination CIDRs.
**Validates: Requirements 17.1**

### Property 57: WireGuard config network CIDR inclusion
*For any* regular peer, the peer's WireGuard configuration should include the network CIDR in AllowedIPs for communication with other peers.
**Validates: Requirements 17.2**

### Property 58: WireGuard config route gateway
*For any* regular peer with routes in AllowedIPs, the WireGuard configuration should specify the route's jump peer as the gateway for the route's destination CIDR.
**Validates: Requirements 17.3**

### Property 59: Jump peer config completeness
*For any* jump peer, the WireGuard configuration should include all peer addresses in AllowedIPs for proper routing.
**Validates: Requirements 17.4**

### Property 60: Jump peer config route CIDRs
*For any* jump peer, the WireGuard configuration should include all route destination CIDRs in AllowedIPs for external network access.
**Validates: Requirements 17.5**

### Property 61: Jump peer iptables generation
*For any* jump peer, the system should generate iptables rules based on all policies attached to groups whose members connect through that jump peer.
**Validates: Requirements 17.6**

### Property 62: IPtables input deny rule
*For any* policy rule with input direction and deny action, the system should create an iptables rule to drop incoming traffic matching the rule target.
**Validates: Requirements 17.7**

### Property 63: IPtables output deny rule
*For any* policy rule with output direction and deny action, the system should create an iptables rule to drop outgoing traffic matching the rule target.
**Validates: Requirements 17.8**

### Property 64: Route change triggers config regeneration
*For any* route that is attached to groups, changing the route should trigger WireGuard configuration regeneration for all peers in those groups.
**Validates: Requirements 17.9**

### Property 65: Policy change triggers iptables regeneration
*For any* policy attached to groups, changing the policy should trigger iptables rule regeneration on all affected jump peers.
**Validates: Requirements 17.10**

## Testing Strategy

### Unit Testing Framework

Unit tests will be written using Go's standard `testing` package with table-driven tests for comprehensive coverage. Tests will focus on:

- **Service Layer Logic**: Group, policy, route, and DNS service operations
- **Domain Model Validation**: CIDR validation, DNS name validation, policy rule validation
- **Repository Operations**: CRUD operations with mock database
- **Authorization Checks**: Admin-only operation enforcement
- **Configuration Generation**: WireGuard config and iptables rule generation

### Property-Based Testing Framework

Property-based tests will use the `gopter` library for Go, configured to run a minimum of 100 iterations per property. Each property test will:

1. Generate random valid inputs using custom generators
2. Execute the operation under test
3. Verify the property holds for the generated inputs
4. Report any counterexamples that violate the property

**Example Property Test Structure:**
```go
func TestProperty_GroupCreationCompleteness(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("Feature: network-groups-policies-routing, Property 1: Group creation completeness",
        prop.ForAll(
            func(name string, description string, networkID string) bool {
                // Create group with generated inputs
                group, err := service.CreateGroup(ctx, networkID, &GroupCreateRequest{
                    Name:        name,
                    Description: description,
                })
                
                // Verify property: group has unique ID, provided name/description, correct network
                return err == nil &&
                    group.ID != "" &&
                    group.Name == name &&
                    group.Description == description &&
                    group.NetworkID == networkID
            },
            genValidGroupName(),
            genDescription(),
            genNetworkID(),
        ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}
```

### Integration Testing

Integration tests will verify end-to-end workflows:

- Creating a group, adding peers, attaching policies, and verifying iptables rules
- Creating routes, attaching to groups, and verifying WireGuard AllowedIPs
- Creating DNS mappings and verifying DNS server responses
- Default group assignment for non-admin peer creation
- Configuration propagation via WebSocket to agents

### API Testing

API tests will verify HTTP endpoints:

- Request/response formats
- Status codes for success and error cases
- Authorization enforcement (admin vs non-admin)
- Pagination and filtering
- Concurrent request handling

### Migration Testing

Migration tests will verify the database migration from legacy system:

- ACL table removal
- Peer table column removal (IsIsolated, FullEncapsulation)
- Data integrity after migration
- Rollback procedures

## Implementation Notes

### Configuration Generation

**WireGuard Configuration:**
- Regular peers: Include network CIDR + route CIDRs from group memberships
- Jump peers: Include all peer addresses + all route CIDRs
- Use existing `wireguard.GenerateConfig` function with updated logic

**IPtables Rules:**
- Generate rules based on policy direction and action
- Apply rules in order of policy attachment
- Include default deny rule at end of chain
- Use iptables-save/iptables-restore for atomic updates

### WebSocket Notification

When groups, policies, or routes change:
1. Identify affected peers (group members)
2. Regenerate configurations for affected peers
3. Send WebSocket notification to connected agents
4. Agents apply new configurations automatically

### DNS Server Updates

When DNS mappings change:
1. Identify all jump peers in the network
2. Generate updated DNS record list (peers + routes)
3. Send WebSocket notification with DNS update
4. Jump peer agents update DNS server configuration

### Default Group Assignment

When a non-admin user creates a peer:
1. Check if network has default groups configured
2. Automatically add peer to all default groups
3. Apply group policies and routes to the new peer
4. Generate initial configuration with group settings

### Authorization Middleware

Extend existing authorization middleware:
```go
func RequireAdminForResource(resourceType string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user := middleware.GetUserFromContext(c)
        if user == nil || !user.IsAdministrator() {
            c.JSON(http.StatusForbidden, gin.H{
                "error": fmt.Sprintf("administrator privileges required for %s operations", resourceType),
            })
            c.Abort()
            return
        }
        c.Next()
    }
}
```

### Database Migration Strategy

1. Create new tables (groups, policies, routes, dns_mappings, junction tables)
2. Add new columns to networks table (domain_suffix, default_group_ids)
3. Remove columns from peers table (is_isolated, full_encapsulation)
4. Drop ACL tables (acls, acl_rules)
5. Create indexes for performance
6. No data migration needed (breaking change, fresh start)

### Performance Considerations

- **Batch Operations**: When adding multiple peers to a group, batch database operations
- **Caching**: Cache policy rules and route CIDRs for configuration generation
- **Async Notifications**: Send WebSocket notifications asynchronously to avoid blocking API requests
- **Database Indexes**: Index foreign keys and frequently queried columns
- **Connection Pooling**: Use database connection pooling for concurrent requests

### Security Considerations

- **Input Validation**: Validate all CIDR formats, DNS names, and identifiers
- **SQL Injection**: Use parameterized queries for all database operations
- **Authorization**: Enforce admin-only access at both API and service layers
- **Audit Logging**: Log all administrative operations (group/policy/route changes)
- **Rate Limiting**: Apply rate limits to prevent abuse of API endpoints

