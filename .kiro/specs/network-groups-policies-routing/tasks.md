# Implementation Plan

- [x] 1. Database schema and migrations
- [x] 1.1 Create migration file for new tables
  - Create groups, policies, policy_rules, routes, dns_mappings tables
  - Create junction tables: group_peers, group_policies, group_routes, network_default_groups
  - Add domain_suffix column to networks table
  - Add indexes for foreign keys and frequently queried columns
  - _Requirements: 1.1, 2.1, 6.1, 8.1, 9.1_

- [x] 1.2 Create migration file to remove legacy systems
  - Drop acls and acl_rules tables
  - Remove is_isolated and full_encapsulation columns from peers table
  - _Requirements: 16.1, 16.2, 16.3, 16.4_

- [x] 2. Domain models and types
- [x] 2.1 Create Group domain model
  - Define Group struct with ID, NetworkID, Name, Description, PeerIDs, PolicyIDs, RouteIDs, timestamps
  - Define GroupCreateRequest and GroupUpdateRequest structs
  - Add validation methods for group names
  - _Requirements: 1.1_

- [x] 2.2 Create Policy domain model
  - Define Policy struct with ID, NetworkID, Name, Description, Rules, timestamps
  - Define PolicyRule struct with ID, Direction, Action, Target, TargetType, Description
  - Define PolicyCreateRequest and PolicyUpdateRequest structs
  - Add validation methods for policy rules
  - _Requirements: 2.1, 2.2_

- [x] 2.3 Create Route domain model
  - Define Route struct with ID, NetworkID, Name, Description, DestinationCIDR, JumpPeerID, DomainSuffix, timestamps
  - Define RouteCreateRequest and RouteUpdateRequest structs
  - Add CIDR validation methods
  - _Requirements: 6.1, 6.2_

- [x] 2.4 Create DNSMapping domain model
  - Define DNSMapping struct with ID, RouteID, Name, IPAddress, timestamps
  - Define DNSMappingCreateRequest and DNSMappingUpdateRequest structs
  - Add GetFQDN method to generate fully qualified domain names
  - Add IP validation methods
  - _Requirements: 8.1, 8.2, 8.3_

- [x] 2.5 Update Network domain model
  - Add DomainSuffix field (default: "internal")
  - Add DefaultGroupIDs field
  - Remove ACL field
  - Update NetworkCreateRequest and NetworkUpdateRequest
  - _Requirements: 9.1, 7.1, 16.4_

- [x] 2.6 Update Peer domain model
  - Add GroupIDs field
  - Remove IsIsolated and FullEncapsulation fields
  - Update PeerCreateRequest and PeerUpdateRequest
  - _Requirements: 16.1, 16.2_

- [x] 3. Repository interfaces
- [x] 3.1 Define GroupRepository interface
  - CreateGroup, GetGroup, UpdateGroup, DeleteGroup, ListGroups methods
  - AddPeerToGroup, RemovePeerFromGroup, GetPeerGroups methods
  - AttachPolicyToGroup, DetachPolicyFromGroup, GetGroupPolicies methods
  - AttachRouteToGroup, DetachRouteFromGroup, GetGroupRoutes methods
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

- [x] 3.2 Define PolicyRepository interface
  - CreatePolicy, GetPolicy, UpdatePolicy, DeletePolicy, ListPolicies methods
  - AddRuleToPolicy, RemoveRuleFromPolicy, UpdateRule methods
  - GetPoliciesForGroup method
  - _Requirements: 2.1, 2.3, 2.4, 2.5, 2.6_

- [x] 3.3 Define RouteRepository interface
  - CreateRoute, GetRoute, UpdateRoute, DeleteRoute, ListRoutes methods
  - GetRoutesForGroup, GetRoutesByJumpPeer methods
  - _Requirements: 6.1, 6.8, 6.9, 6.10_

- [x] 3.4 Define DNSRepository interface
  - CreateDNSMapping, GetDNSMapping, UpdateDNSMapping, DeleteDNSMapping, ListDNSMappings methods
  - GetNetworkDNSMappings method
  - _Requirements: 8.1, 8.3, 8.4, 8.5_

- [x] 4. PostgreSQL repository implementations
- [x] 4.1 Implement GroupRepository for PostgreSQL
  - Implement all CRUD operations with proper error handling
  - Implement peer membership operations with transaction support
  - Implement policy and route attachment operations
  - Handle foreign key constraints and cascading deletes
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

- [x] 4.2 IlicyRepository for PostgreSQL
  - Implement all CRUD operations with proper error handling
  - Implement rule management with ordering support
  - Handle policy-group relationships
  - _Requirements: 2.1, 2.3, 2.4, 2.5, 2.6_

- [x] 4.3 Implement RouteRepository for PostgreSQL
  - Implement all CRUD operations with proper error handling
  - Implement group-route relationship queries
  - Validate jump peer references
  - _Requirements: 6.1, 6.8, 6.9, 6.10_

- [x] 4.4 Implement DNSRepository for PostgreSQL
  - Implement all CRUD operations with proper error handling
  - Implement network-wide DNS mapping queries
  - Validate IP addresses within route CIDRs
  - _Requirements: 8.1, 8.3, 8.4, 8.5_

- [x] 5. Application services
- [x] 5.1 Implement GroupService
  - Implement CreateGroup with name validation and admin check
  - Implement GetGroup, UpdateGroup, DeleteGroup with authorization
  - Implement ListGroups with network filtering
  - Implement AddPeerToGroup and RemovePeerFromGroup with validation
  - Implement AttachPolicyToGroup and DetachPolicyFromGroup with WebSocket notification
  - Implement AttachRouteToGroup and DetachRouteFromGroup with config regeneration
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7_

- [x] 5.2 Write property test for group creation
  - **Property 1: Group creation completeness**
  - **Validates: Requirements 1.1**

- [x] 5.3 Write property test for peer-group association
  - **Property 2: Peer-group association preservation**
  - **Validates: Requirements 1.2**

- [x] 5.4 Write property test for peer removal
  - **Property 3: Peer removal non-destructiveness**
  - **Validates: Requirements 1.3**

- [x] 5.5 Write property test for group deletion
  - **Property 4: Group deletion peer preservation**
  - **Validates: Requirements 1.4**

- [x] 5.6 Write property test for group listing
  - **Property 5: Group listing completeness**
  - **Validates: Requirements 1.5**

- [x] 5.7 Write property test for group authorization
  - **Property 7: Group operation authorization**
  - **Validates: Requirements 1.7**

- [x] 5.8 Implement PolicyService
  - Implement CreatePolicy with name validation and admin check
  - Implement GetPolicy, UpdatePolicy, DeletePolicy with authorization
  - Implement ListPolicies with network filtering
  - Implement AddRuleToPolicy and RemoveRuleFromPolicy with validation
  - Implement GetDefaultTemplates for policy templates
  - Implement GenerateIPTablesRules for jump peer configuration
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

- [x] 5.9 Write property test for policy creation
  - **Property 8: Policy creation completeness**
  - **Validates: Requirements 2.1**

- [x] 5.10 Write property test for policy rule validation
  - **Property 9: Policy rule validation**
  - **Validates: Requirements 2.2**

- [x] 5.11 Write property test for policy rule addition
  - **Property 10: Policy rule addition**
  - **Validates: Requirements 2.3**

- [x] 5.12 Write property test for policy rule removal
  - **Property 11: Policy rule removal**
  - **Validates: Requirements 2.4**

- [x] 5.13 Write property test for policy update propagation
  - **Property 12: Policy update propagation**
  - **Validates: Requirements 2.5**

- [x] 5.14 Write property test for policy deletion cleanup
  - **Property 13: Policy deletion cleanup**
  - **Validates: Requirements 2.6**

- [x] 5.15 Write property test for template independence
  - **Property 15: Template policy independence**
  - **Validates: Requirements 3.5**

- [x] 5.16 Write property test for policy attachment
  - **Property 17: Policy attachment application**
  - **Validates: Requirements 4.1**

- [x] 5.17 Write property test for multiple policy ordering
  - **Property 18: Multiple policy ordering**
  - **Validates: Requirements 4.2**

- [x] 5.18 Write property test for policy detachment
  - **Property 19: Policy detachment cleanup**
  - **Validates: Requirements 4.3**

- [x] 5.19 Write property test for automatic policy application
  - **Property 20: Automatic policy application on join**
  - **Validates: Requirements 4.4**

- [x] 5.20 Write property test for automatic policy removal
  - **Property 21: Automatic policy removal on leave**
  - **Validates: Requirements 4.5**

- [x] 5.21 Write property test for policy-only access control
  - **Property 23: Policy-only access control**
  - **Validates: Requirements 5.1**

- [x] 5.22 Write property test for deny rule enforcement
  - **Property 24: Deny rule enforcement**
  - **Validates: Requirements 5.2**

- [x] 5.23 Write property test for allow rule enforcement
  - **Property 25: Allow rule enforcement**
  - **Validates: Requirements 5.3**

- [x] 5.24 Write property test for default deny behavior
  - **Property 26: Default deny behavior**
  - **Validates: Requirements 5.4**

- [x] 5.25 Implement RouteService
  - Implement CreateRoute with CIDR and jump peer validation
  - Implement GetRoute, UpdateRoute, DeleteRoute with authorization
  - Implement ListRoutes with network filtering
  - Implement GetPeerRoutes to calculate routes for a peer based on group membership
  - Trigger WireGuard config regeneration on route changes
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8, 6.9, 6.10, 6.11_

- [x] 5.26 Write property test for route creation
  - **Property 27: Route creation completeness**
  - **Validates: Requirements 6.1**

- [x] 5.27 Write property test for route CIDR validation
  - **Property 28: Route CIDR validation**
  - **Validates: Requirements 6.2**

- [x] 5.28 Write property test for route jump peer validation
  - **Property 29: Route jump peer validation**
  - **Validates: Requirements 6.3**

- [x] 5.29 Write property test for route attachment
  - **Property 30: Route attachment to group**
  - **Validates: Requirements 6.4**

- [x] 5.30 Write property test for route detachment
  - **Property 31: Route detachment from group**
  - **Validates: Requirements 6.5**

- [x] 5.31 Write property test for automatic route addition
  - **Property 32: Automatic route addition on join**
  - **Validates: Requirements 6.6**

- [x] 5.32 Write property test for automatic route removal
  - **Property 33: Automatic route removal on leave**
  - **Validates: Requirements 6.7**

- [x] 5.33 Write property test for route update propagation
  - **Property 34: Route update propagation**
  - **Validates: Requirements 6.8**

- [x] 5.34 Write property test for route deletion cleanup
  - **Property 35: Route deletion cleanup**
  - **Validates: Requirements 6.9**

- [x] 5.35 Implement DNSService
  - Implement CreateDNSMapping with IP validation within route CIDR
  - Implement GetDNSMapping, UpdateDNSMapping, DeleteDNSMapping with authorization
  - Implement ListDNSMappings for a route
  - Implement GetNetworkDNSRecords to combine peer and route DNS records
  - Trigger DNS server updates via WebSocket on changes
  - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6_

- [x] 5.36 Write property test for DNS mapping IP validation
  - **Property 45: DNS mapping IP validation**
  - **Validates: Requirements 8.2**

- [x] 5.37 Write property test for DNS mapping FQDN format
  - **Property 46: DNS mapping FQDN format**
  - **Validates: Requirements 8.3**

- [x] 6. Update NetworkService for new features
- [x] 6.1 Update CreateNetwork to support domain suffix
  - Add domain_suffix field with default value "internal"
  - Validate domain suffix format
  - _Requirements: 9.1_

- [x] 6.2 Update AddPeer to support default groups
  - Check if user is admin or non-admin
  - For non-admin users, automatically add peer to network's default groups
  - Apply group policies and routes to new peer
  - _Requirements: 7.2, 7.3_

- [x] 6.3 Write property test for non-admin peer auto-assignment
  - **Property 39: Non-admin peer auto-assignment**
  - **Validates: Requirements 7.2**

- [x] 6.4 Write property test for admin peer no auto-assignment
  - **Property 40: Admin peer no auto-assignment**
  - **Validates: Requirements 7.3**

- [x] 6.5 Update GeneratePeerConfigWithDNS to include route DNS records
  - Combine peer DNS records with route DNS records
  - Use custom domain suffixes for networks and routes
  - Send updated DNS configuration to jump peer agents
  - _Requirements: 10.1, 10.2, 10.3_

- [x] 6.6 Write property test for DNS server initialization
  - **Property 53: DNS server initialization completeness**
  - **Validates: Requirements 10.1**

- [x] 6.7 Write property test for route DNS query resolution
  - **Property 54: Route DNS query resolution**
  - **Validates: Requirements 10.2**

- [x] 6.8 Write property test for peer DNS query resolution
  - **Property 55: Peer DNS query resolution**
  - **Validates: Requirements 10.3**

- [x] 6.9 Update GeneratePeerConfig for policy-based routing
  - Remove legacy IsIsolated and FullEncapsulation logic
  - Calculate AllowedIPs based on group routes
  - Include network CIDR for peer-to-peer communication
  - Configure jump peer as gateway for route CIDRs
  - _Requirements: 17.1, 17.2, 17.3, 17.4, 17.5_

- [x] 6.10 Write property test for WireGuard config route inclusion
  - **Property 56: WireGuard config route inclusion**
  - **Validates: Requirements 17.1**

- [x] 6.11 Write property test for WireGuard config network CIDR
  - **Property 57: WireGuard config network CIDR inclusion**
  - **Validates: Requirements 17.2**

- [x] 6.12 Write property test for WireGuard config route gateway
  - **Property 58: WireGuard config route gateway**
  - **Validates: Requirements 17.3**

- [x] 6.13 Write property test for jump peer config completeness
  - **Property 59: Jump peer config completeness**
  - **Validates: Requirements 17.4**

- [x] 6.14 Write property test for jump peer config route CIDRs
  - **Property 60: Jump peer config route CIDRs**
  - **Validates: Requirements 17.5**

- [x] 6.15 Implement iptables rule generation for jump peers
  - Generate iptables rules from policies attached to groups
  - Apply rules in order of policy attachment
  - Include default deny rule at end
  - Send iptables rules to jump peer agents via WebSocket
  - _Requirements: 17.6, 17.7, 17.8_

- [x] 6.16 Write property test for jump peer iptables generation
  - **Property 61: Jump peer iptables generation**
  - **Validates: Requirements 17.6**

- [x] 6.17 Write property test for iptables input deny rule
  - **Property 62: IPtables input deny rule**
  - **Validates: Requirements 17.7**

- [x] 6.18 Write property test for iptables output deny rule
  - **Property 63: IPtables output deny rule**
  - **Validates: Requirements 17.8**

- [x] 7. API handlers for groups
- [x] 7.1 Implement group CRUD handlers
  - POST /api/v1/networks/:networkId/groups - CreateGroup
  - GET /api/v1/networks/:networkId/groups - ListGroups
  - GET /api/v1/networks/:networkId/groups/:groupId - GetGroup
  - PUT /api/v1/networks/:networkId/groups/:groupId - UpdateGroup
  - DELETE /api/v1/networks/:networkId/groups/:groupId - DeleteGroup
  - Add requireAdmin middleware to all group endpoints
  - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5_

- [x] 7.2 Implement group membership handlers
  - POST /api/v1/networks/:networkId/groups/:groupId/peers/:peerId - AddPeerToGroup
  - DELETE /api/v1/networks/:networkId/groups/:groupId/peers/:peerId - RemovePeerFromGroup
  - Add requireAdmin middleware
  - _Requirements: 11.6, 11.7_

- [x] 8. API handlers for policies
- [x] 8.1 Implement policy CRUD handlers
  - POST /api/v1/networks/:networkId/policies - CreatePolicy
  - GET /api/v1/networks/:networkId/policies - ListPolicies
  - GET /api/v1/networks/:networkId/policies/:policyId - GetPolicy
  - PUT /api/v1/networks/:networkId/policies/:policyId - UpdatePolicy
  - DELETE /api/v1/networks/:networkId/policies/:policyId - DeletePolicy
  - Add requireAdmin middleware to all policy endpoints
  - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5_

- [x] 8.2 Implement policy rule handlers
  - POST /api/v1/networks/:networkId/policies/:policyId/rules - AddRuleToPolicy
  - DELETE /api/v1/networks/:networkId/policies/:policyId/rules/:ruleId - RemoveRuleFromPolicy
  - Add requireAdmin middleware
  - _Requirements: 12.6, 12.7_

- [x] 8.3 Implement policy template handler
  - GET /api/v1/networks/:networkId/policies/templates - GetDefaultTemplates
  - Return fully encapsulated, isolated, and default network templates
  - Add requireAdmin middleware
  - _Requirements: 12.8_

- [x] 8.4 Implement policy attachment handlers
  - POST /api/v1/networks/:networkId/groups/:groupId/policies/:policyId - AttachPolicyToGroup
  - DELETE /api/v1/networks/:networkId/groups/:groupId/policies/:policyId - DetachPolicyFromGroup
  - GET /api/v1/networks/:networkId/groups/:groupId/policies - GetGroupPolicies
  - Add requireAdmin middleware
  - _Requirements: 15.1, 15.2, 15.3, 15.4_

- [x] 9. API handlers for routes
- [x] 9.1 Implement route CRUD handlers
  - POST /api/v1/networks/:networkId/routes - CreateRoute
  - GET /api/v1/networks/:networkId/routes - ListRoutes
  - GET /api/v1/networks/:networkId/routes/:routeId - GetRoute
  - PUT /api/v1/networks/:networkId/routes/:routeId - UpdateRoute
  - DELETE /api/v1/networks/:networkId/routes/:routeId - DeleteRoute
  - Add requireAdmin middleware to all route endpoints
  - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5_

- [x] 9.2 Implement route attachment handlers
  - POST /api/v1/networks/:networkId/groups/:groupId/routes/:routeId - AttachRouteToGroup
  - DELETE /api/v1/networks/:networkId/groups/:groupId/routes/:routeId - DetachRouteFromGroup
  - GET /api/v1/networks/:networkId/groups/:groupId/routes - GetGroupRoutes
  - Add requireAdmin middleware
  - _Requirements: 13.6, 13.7, 13.8, 13.9_

- [x] 10. API handlers for DNS mappings
- [x] 10.1 Implement DNS mapping CRUD handlers
  - POST /api/v1/networks/:networkId/routes/:routeId/dns - CreateDNSMapping
  - GET /api/v1/networks/:networkId/routes/:routeId/dns - ListDNSMappings
  - PUT /api/v1/networks/:networkId/routes/:routeId/dns/:dnsId - UpdateDNSMapping
  - DELETE /api/v1/networks/:networkId/routes/:routeId/dns/:dnsId - DeleteDNSMapping
  - Add requireAdmin middleware to all DNS endpoints
  - _Requirements: 14.1, 14.2, 14.4, 14.5_

- [x] 10.2 Implement network DNS listing handler
  - GET /api/v1/networks/:networkId/dns - GetNetworkDNSRecords
  - Return combined peer and route DNS records
  - Add requireAdmin middleware
  - _Requirements: 14.3_

- [x] 11. Update agent DNS server
- [x] 11.1 Update DNS server to support route domains
  - Extend DNSPeer type to include route DNS mappings
  - Update lookupPeerIP to check both peer and route domains
  - Support custom domain suffixes
  - _Requirements: 10.1, 10.2, 10.3_

- [x] 11.2 Update agent runner to handle DNS updates
  - Process DNS configuration updates from WebSocket
  - Update running DNS server with new records
  - _Requirements: 10.4, 10.5_

- [x] 12. Update agent firewall for policy-based rules
- [x] 12.1 Update firewall adapter to accept policy-based rules
  - Replace isolation-based logic with policy rule processing
  - Generate iptables rules from policy rules
  - Apply rules in order with default deny at end
  - _Requirements: 5.1, 5.2, 5.3, 5.4, 17.6, 17.7, 17.8_

- [x] 12.2 Update agent runner to handle policy updates
  - Process policy configuration updates from WebSocket
  - Apply new iptables rules atomically
  - _Requirements: 17.9, 17.10_

- [x] 13. Frontend components (admin-only)
- [x] 13.1 Create Groups management page
  - List groups with member counts
  - Create/edit/delete group forms
  - Add/remove peers from groups
  - Display attached policies and routes
  - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7_

- [x] 13.2 Create Policies management page
  - List policies with rule counts
  - Create/edit/delete policy forms
  - Add/remove rules from policies
  - Display policy templates
  - Attach/detach policies to/from groups
  - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5, 12.6, 12.7, 12.8, 15.1, 15.2, 15.3_

- [x] 13.3 Create Routes management page
  - List routes with jump peer and group associations
  - Create/edit/delete route forms
  - Attach/detach routes to/from groups
  - Manage DNS mappings for routes
  - _Requirements: 13.1, 13.2, 13.3, 13.4, 13.5, 13.6, 13.7, 13.8, 14.1, 14.2, 14.4, 14.5_

- [x] 13.4 Update Network settings page
  - Add domain suffix configuration
  - Add default groups configuration
  - _Requirements: 9.1, 9.2, 7.1, 7.5_

- [x] 13.5 Update Peer detail page
  - Display group memberships
  - Show effective policies and routes from groups
  - Remove legacy isolation and encapsulation controls
  - _Requirements: 16.2_

- [x] 14. Documentation and migration guide
- [x] 14.1 Write API documentation
  - Document all new endpoints with request/response examples
  - Update Swagger/OpenAPI specifications
  - _Requirements: All API requirements_

- [x] 14.2 Write migration guide
  - Document breaking changes (ACL removal, peer flag removal)
  - Provide migration steps for existing deployments
  - Explain new concepts (groups, policies, routes)
  - _Requirements: 16.1, 16.2, 16.3, 16.4, 16.5_

- [x] 14.3 Update user documentation
  - Add guides for creating groups and assigning peers
  - Add guides for creating policies and attaching to groups
  - Add guides for creating routes and DNS mappings
  - Add guide for configuring default groups
  - _Requirements: All requirements_

- [x] 15. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

