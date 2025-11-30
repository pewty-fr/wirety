# Requirements Document

## Introduction

This document specifies requirements for enhancing the WireGuard network management system with groups, policies, routing, and DNS capabilities. The current system manages peers individually with basic isolation and encapsulation flags, and uses a separate ACL system for access control. This enhancement introduces a complete architectural redesign where:

- Peers can be organized into groups (admin-only)
- Policies define iptables rules on jump peers for traffic filtering (admin-only, attached to groups)
- Routes define AllowedIPs in WireGuard configurations for regular peers (admin-only, attached to groups, routed through specific jump peers)
- DNS resolves custom domains for routes (admin-only)
- Default groups can be configured for automatic assignment to non-admin created peers

These changes represent breaking changes from the current system. The legacy ACL system, IsIsolated flag, and FullEncapsulation flag are completely removed with no backward compatibility.

## Glossary

- **System**: The WireGuard network management server and agent
- **Peer**: A network participant in the WireGuard mesh (device or server)
- **Jump Peer**: A peer that acts as a hub routing traffic for regular peers
- **Regular Peer**: A peer that connects through jump peers
- **Group**: A collection of peers that share common characteristics or policies
- **Policy**: A set of iptables rules applied on jump peers to filter traffic between peers
- **Policy Rule**: A specific allow or deny iptables rule for IP ranges or peer traffic in input or output direction
- **Route**: A network destination (IP range) that is added to AllowedIPs in regular peer WireGuard configurations when attached to a group, accessible through a specific jump peer
- **Administrator**: A user with administrative privileges who can manage groups, policies, routes, and DNS
- **Default Group**: A group automatically assigned to peers created by non-administrator users
- **DNS Record**: A domain name to IP address mapping in the internal DNS system
- **Internal Domain**: The DNS domain suffix used for internal name resolution (default: .internal)

- **CIDR**: Classless Inter-Domain Routing notation for IP address ranges

## Requirements

### Requirement 1: Group Management

**User Story:** As a network administrator, I want to organize peers into groups, so that I can manage related peers collectively and apply policies efficiently.

#### Acceptance Criteria

1. WHEN an administrator creates a group THEN the System SHALL create a new group with a unique identifier, name, description, and network association
2. WHEN an administrator adds a peer to a group THEN the System SHALL associate the peer with the group while maintaining the peer's individual identity
3. WHEN an administrator removes a peer from a group THEN the System SHALL disassociate the peer from the group without deleting the peer
4. WHEN an administrator deletes a group THEN the System SHALL remove the group and disassociate all member peers without deleting the peers
5. WHEN an administrator lists groups THEN the System SHALL return all groups for the specified network with member peer counts
6. WHEN an administrator retrieves a group THEN the System SHALL return the group details including all member peer identifiers
7. WHEN a non-administrator attempts to create, modify, or delete a group THEN the System SHALL reject the request with HTTP status 403

### Requirement 2: Policy Definition and Management

**User Story:** As a network administrator, I want to define reusable policies with iptables rules, so that I can control traffic filtering on jump peers consistently across groups.

#### Acceptance Criteria

1. WHEN an administrator creates a policy THEN the System SHALL create a new policy with a unique identifier, name, description, and network association
2. WHEN an administrator defines a policy rule THEN the System SHALL validate the rule contains direction (input or output), action (allow or deny), and target (IP range, peer identifier, or group identifier)
3. WHEN an administrator adds a rule to a policy THEN the System SHALL append the rule to the policy's rule list
4. WHEN an administrator removes a rule from a policy THEN the System SHALL delete the rule from the policy's rule list
5. WHEN an administrator updates a policy THEN the System SHALL modify the policy properties and regenerate iptables rules on all affected jump peers
6. WHEN an administrator deletes a policy THEN the System SHALL remove the policy, detach it from all associated groups, and remove corresponding iptables rules from jump peers
7. WHEN a non-administrator attempts to create, modify, or delete a policy THEN the System SHALL reject the request with HTTP status 403

### Requirement 3: Default Policy Templates

**User Story:** As a network administrator, I want to use predefined policy templates, so that I can quickly apply common iptables configurations without defining rules manually.

#### Acceptance Criteria

1. WHEN an administrator requests default policies THEN the System SHALL provide a fully encapsulated policy template that allows all outbound traffic and denies all inbound traffic
2. WHEN an administrator requests default policies THEN the System SHALL provide an isolated policy template that denies all inbound and outbound traffic
3. WHEN an administrator requests default policies THEN the System SHALL provide a default network policy template that allows all traffic within the specified network CIDR
4. WHEN an administrator applies a default policy template THEN the System SHALL create a new policy instance with the template's predefined iptables rules
5. WHEN an administrator modifies a policy created from a template THEN the System SHALL allow modifications without affecting the original template
6. WHEN a non-administrator attempts to access policy templates THEN the System SHALL reject the request with HTTP status 403

### Requirement 4: Policy Attachment to Groups

**User Story:** As a network administrator, I want to attach policies to groups, so that I can control iptables filtering for all group members on jump peers.

#### Acceptance Criteria

1. WHEN an administrator attaches a policy to a group THEN the System SHALL apply the policy's iptables rules on all jump peers for all current and future members of the group
2. WHEN an administrator attaches multiple policies to a group THEN the System SHALL apply all policies' iptables rules in the order they were attached
3. WHEN an administrator detaches a policy from a group THEN the System SHALL remove the policy's iptables rules from all jump peers for all group members
4. WHEN a peer joins a group with attached policies THEN the System SHALL automatically apply the group's policy iptables rules on all jump peers for that peer
5. WHEN a peer leaves a group with attached policies THEN the System SHALL remove the group's policy iptables rules from all jump peers for that peer
6. WHEN a non-administrator attempts to attach or detach policies THEN the System SHALL reject the request with HTTP status 403

### Requirement 5: Policy-Based Access Control

**User Story:** As a network administrator, I want all access control managed exclusively through policies, so that I have a single unified system for managing network permissions.

#### Acceptance Criteria

1. WHEN the System evaluates peer communication THEN the System SHALL use only policy rules for access control decisions
2. WHEN a policy rule denies traffic THEN the System SHALL block the traffic
3. WHEN a policy rule allows traffic THEN the System SHALL permit the traffic
4. WHEN no policy rules match a traffic flow THEN the System SHALL deny the traffic by default
5. WHEN a peer has no attached policies THEN the System SHALL deny all traffic to and from that peer

### Requirement 6: Route Definition and Management

**User Story:** As a network administrator, I want to define routes to external networks through jump peers, so that regular peers in groups can access resources outside the WireGuard network via AllowedIPs configuration.

#### Acceptance Criteria

1. WHEN an administrator creates a route THEN the System SHALL create a new route with a unique identifier, name, description, destination CIDR, and associated jump peer identifier
2. WHEN an administrator creates a route THEN the System SHALL validate the destination CIDR is a valid IP range
3. WHEN an administrator creates a route THEN the System SHALL validate the associated jump peer exists and is configured as a jump peer
4. WHEN an administrator attaches a route to a group THEN the System SHALL add the route's destination CIDR to the AllowedIPs configuration for all group members
5. WHEN an administrator detaches a route from a group THEN the System SHALL remove the route's destination CIDR from the AllowedIPs configuration for all group members
6. WHEN a peer joins a group with attached routes THEN the System SHALL automatically add all route destination CIDRs to the peer's AllowedIPs configuration
7. WHEN a peer leaves a group with attached routes THEN the System SHALL remove the group's route destination CIDRs from the peer's AllowedIPs configuration
8. WHEN an administrator updates a route THEN the System SHALL modify the route properties and regenerate WireGuard configurations for all peers in groups using that route
9. WHEN an administrator deletes a route THEN the System SHALL remove the route, detach it from all groups, and update all affected peer WireGuard configurations
10. WHEN an administrator lists routes THEN the System SHALL return all routes for the specified network with their associated jump peers
11. WHEN a non-administrator attempts to create, modify, or delete a route THEN the System SHALL reject the request with HTTP status 403

### Requirement 7: Default Groups for Non-Administrator Peers

**User Story:** As a network administrator, I want to configure default groups that are automatically assigned to peers created by non-administrators, so that new peers automatically receive appropriate policies and access.

#### Acceptance Criteria

1. WHEN an administrator configures a default group for a network THEN the System SHALL store the default group association with the network
2. WHEN a non-administrator creates a peer THEN the System SHALL automatically add the peer to all configured default groups for that network
3. WHEN an administrator creates a peer THEN the System SHALL not automatically add the peer to default groups
4. WHEN an administrator removes a default group configuration THEN the System SHALL not affect existing peers in that group
5. WHEN an administrator lists default groups for a network THEN the System SHALL return all configured default groups
6. WHEN a non-administrator attempts to configure default groups THEN the System SHALL reject the request with HTTP status 403

### Requirement 8: DNS Domain Management

**User Story:** As a network administrator, I want to define custom DNS domains for routes, so that peers can resolve human-readable names for external network resources.

#### Acceptance Criteria

1. WHEN an administrator creates a route with a domain name THEN the System SHALL associate the domain name with the route
2. WHEN an administrator defines a DNS mapping for a route THEN the System SHALL validate the IP address is within the route's destination CIDR
3. WHEN an administrator creates a DNS mapping THEN the System SHALL create a record mapping the specified name to the IP address with the format name.route_name.internal
4. WHEN an administrator updates a DNS mapping THEN the System SHALL modify the DNS record and propagate changes to all jump peer DNS servers within 60 seconds
5. WHEN an administrator deletes a DNS mapping THEN the System SHALL remove the DNS record from all jump peer DNS servers within 60 seconds
6. WHEN a non-administrator attempts to create, modify, or delete DNS mappings THEN the System SHALL reject the request with HTTP status 403

### Requirement 9: Custom Internal Domain Configuration

**User Story:** As a network administrator, I want to configure custom internal domain suffixes for networks and routes, so that DNS names match my organization's naming conventions.

#### Acceptance Criteria

1. WHEN an administrator creates a network THEN the System SHALL use the default internal domain suffix of .internal
2. WHEN an administrator specifies a custom domain suffix for a network THEN the System SHALL use the custom suffix for all peer DNS records in that network
3. WHEN an administrator specifies a custom domain suffix for a route THEN the System SHALL use the custom suffix for all DNS records associated with that route
4. WHEN an administrator updates a domain suffix THEN the System SHALL regenerate all affected DNS records with the new suffix and propagate to jump peer DNS servers
5. WHEN the System generates a peer DNS record THEN the System SHALL use the format peer_name.network_name.domain_suffix
6. WHEN a non-administrator attempts to modify domain suffixes THEN the System SHALL reject the request with HTTP status 403

### Requirement 10: DNS Server Update for Routes

**User Story:** As a network administrator, I want jump peer DNS servers to resolve route-based domains, so that peers can access external resources using human-readable names.

#### Acceptance Criteria

1. WHEN a jump peer DNS server starts THEN the System SHALL load all peer DNS records and all route DNS records for the network
2. WHEN a DNS query matches a route domain pattern THEN the System SHALL return the IP address associated with the route DNS mapping
3. WHEN a DNS query matches a peer domain pattern THEN the System SHALL return the peer's IP address
4. WHEN a route DNS mapping is added THEN the System SHALL update all jump peer DNS servers in the network within 60 seconds
5. WHEN a route DNS mapping is removed THEN the System SHALL remove the record from all jump peer DNS servers in the network within 60 seconds

### Requirement 11: API Endpoints for Groups

**User Story:** As a frontend developer, I want RESTful API endpoints for group management, so that I can build user interfaces for group operations.

#### Acceptance Criteria

1. WHEN a client sends a POST request to /api/networks/{networkID}/groups THEN the System SHALL create a new group and return the group details with HTTP status 201
2. WHEN a client sends a GET request to /api/networks/{networkID}/groups THEN the System SHALL return a list of all groups in the network with HTTP status 200
3. WHEN a client sends a GET request to /api/networks/{networkID}/groups/{groupID} THEN the System SHALL return the group details with HTTP status 200
4. WHEN a client sends a PUT request to /api/networks/{networkID}/groups/{groupID} THEN the System SHALL update the group and return the updated details with HTTP status 200
5. WHEN a client sends a DELETE request to /api/networks/{networkID}/groups/{groupID} THEN the System SHALL delete the group and return HTTP status 204
6. WHEN a client sends a POST request to /api/networks/{networkID}/groups/{groupID}/peers/{peerID} THEN the System SHALL add the peer to the group and return HTTP status 200
7. WHEN a client sends a DELETE request to /api/networks/{networkID}/groups/{groupID}/peers/{peerID} THEN the System SHALL remove the peer from the group and return HTTP status 204

### Requirement 12: API Endpoints for Policies

**User Story:** As a frontend developer, I want RESTful API endpoints for policy management, so that I can build user interfaces for policy operations.

#### Acceptance Criteria

1. WHEN a client sends a POST request to /api/networks/{networkID}/policies THEN the System SHALL create a new policy and return the policy details with HTTP status 201
2. WHEN a client sends a GET request to /api/networks/{networkID}/policies THEN the System SHALL return a list of all policies in the network with HTTP status 200
3. WHEN a client sends a GET request to /api/networks/{networkID}/policies/{policyID} THEN the System SHALL return the policy details including all rules with HTTP status 200
4. WHEN a client sends a PUT request to /api/networks/{networkID}/policies/{policyID} THEN the System SHALL update the policy and return the updated details with HTTP status 200
5. WHEN a client sends a DELETE request to /api/networks/{networkID}/policies/{policyID} THEN the System SHALL delete the policy and return HTTP status 204
6. WHEN a client sends a POST request to /api/networks/{networkID}/policies/{policyID}/rules THEN the System SHALL add a rule to the policy and return HTTP status 201
7. WHEN a client sends a DELETE request to /api/networks/{networkID}/policies/{policyID}/rules/{ruleID} THEN the System SHALL remove the rule from the policy and return HTTP status 204
8. WHEN a client sends a GET request to /api/networks/{networkID}/policies/templates THEN the System SHALL return available default policy templates with HTTP status 200

### Requirement 13: API Endpoints for Routes

**User Story:** As a frontend developer, I want RESTful API endpoints for route management, so that I can build user interfaces for route operations.

#### Acceptance Criteria

1. WHEN a client sends a POST request to /api/networks/{networkID}/routes THEN the System SHALL create a new route and return the route details with HTTP status 201
2. WHEN a client sends a GET request to /api/networks/{networkID}/routes THEN the System SHALL return a list of all routes in the network with HTTP status 200
3. WHEN a client sends a GET request to /api/networks/{networkID}/routes/{routeID} THEN the System SHALL return the route details with HTTP status 200
4. WHEN a client sends a PUT request to /api/networks/{networkID}/routes/{routeID} THEN the System SHALL update the route and return the updated details with HTTP status 200
5. WHEN a client sends a DELETE request to /api/networks/{networkID}/routes/{routeID} THEN the System SHALL delete the route and return HTTP status 204
6. WHEN a client sends a POST request to /api/networks/{networkID}/groups/{groupID}/routes/{routeID} THEN the System SHALL attach the route to the group and return HTTP status 200
7. WHEN a client sends a DELETE request to /api/networks/{networkID}/groups/{groupID}/routes/{routeID} THEN the System SHALL detach the route from the group and return HTTP status 204
8. WHEN a client sends a GET request to /api/networks/{networkID}/groups/{groupID}/routes THEN the System SHALL return all routes attached to the group with HTTP status 200
9. WHEN a non-administrator client attempts route operations THEN the System SHALL reject the request with HTTP status 403

### Requirement 14: API Endpoints for DNS Mappings

**User Story:** As a frontend developer, I want RESTful API endpoints for DNS mapping management, so that I can build user interfaces for DNS operations.

#### Acceptance Criteria

1. WHEN a client sends a POST request to /api/networks/{networkID}/routes/{routeID}/dns THEN the System SHALL create a new DNS mapping and return the mapping details with HTTP status 201
2. WHEN a client sends a GET request to /api/networks/{networkID}/routes/{routeID}/dns THEN the System SHALL return a list of all DNS mappings for the route with HTTP status 200
3. WHEN a client sends a GET request to /api/networks/{networkID}/dns THEN the System SHALL return all DNS mappings for the network including peer and route mappings with HTTP status 200
4. WHEN a client sends a PUT request to /api/networks/{networkID}/routes/{routeID}/dns/{dnsID} THEN the System SHALL update the DNS mapping and return the updated details with HTTP status 200
5. WHEN a client sends a DELETE request to /api/networks/{networkID}/routes/{routeID}/dns/{dnsID} THEN the System SHALL delete the DNS mapping and return HTTP status 204

### Requirement 15: Policy Attachment API Endpoints

**User Story:** As a frontend developer, I want RESTful API endpoints for attaching and detaching policies to groups, so that I can build user interfaces for policy assignment.

#### Acceptance Criteria

1. WHEN a client sends a POST request to /api/networks/{networkID}/groups/{groupID}/policies/{policyID} THEN the System SHALL attach the policy to the group and return HTTP status 200
2. WHEN a client sends a DELETE request to /api/networks/{networkID}/groups/{groupID}/policies/{policyID} THEN the System SHALL detach the policy from the group and return HTTP status 204
3. WHEN a client sends a GET request to /api/networks/{networkID}/groups/{groupID}/policies THEN the System SHALL return all policies attached to the group with HTTP status 200
4. WHEN a non-administrator client attempts policy attachment operations THEN the System SHALL reject the request with HTTP status 403

### Requirement 16: Removal of Legacy Systems

**User Story:** As a system administrator, I want the legacy ACL and peer flag systems completely removed, so that I work with a clean, modern policy-based architecture.

#### Acceptance Criteria

1. WHEN the System starts THEN the System SHALL not support the legacy IsIsolated, FullEncapsulation peer flags, or ACL system
2. WHEN a network administrator accesses peer configuration THEN the System SHALL not expose IsIsolated, FullEncapsulation, or ACL fields in API responses
3. WHEN a network administrator creates or updates a peer THEN the System SHALL reject requests containing IsIsolated, FullEncapsulation, or ACL fields with HTTP status 400
4. WHEN a network administrator accesses network configuration THEN the System SHALL not expose ACL fields in API responses
5. WHEN the System generates WireGuard configuration THEN the System SHALL use only policy rules for access control and routing decisions

### Requirement 17: WireGuard Configuration Generation

**User Story:** As a system component, I want to generate WireGuard configurations based on routes and apply iptables rules based on policies, so that peers receive correct network access settings.

#### Acceptance Criteria

1. WHEN the System generates a regular peer configuration THEN the System SHALL include AllowedIPs entries for all routes attached to the groups that the peer belongs to
2. WHEN the System generates a regular peer configuration THEN the System SHALL include the network CIDR in AllowedIPs for communication with other peers
3. WHEN the System generates a regular peer configuration THEN the System SHALL configure the route's jump peer as the gateway for the route's destination CIDR
4. WHEN the System generates a jump peer configuration THEN the System SHALL include all peer addresses in AllowedIPs for proper routing
5. WHEN the System generates a jump peer configuration THEN the System SHALL include all route destination CIDRs in AllowedIPs for external network access
6. WHEN the System applies policies to a jump peer THEN the System SHALL generate iptables rules based on all policies attached to groups whose members connect through that jump peer
7. WHEN a policy rule specifies input direction and deny action THEN the System SHALL create an iptables rule to drop incoming traffic matching the rule target
8. WHEN a policy rule specifies output direction and deny action THEN the System SHALL create an iptables rule to drop outgoing traffic matching the rule target
9. WHEN a route changes THEN the System SHALL trigger WireGuard configuration regeneration for all peers in groups using that route
10. WHEN a policy changes THEN the System SHALL trigger iptables rule regeneration on all affected jump peers

