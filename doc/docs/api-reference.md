# API Reference

This document provides detailed information about the WireGuard network management API endpoints.

## Authentication

All API endpoints require authentication via JWT token in the Authorization header:

```
Authorization: Bearer <token>
```

## Admin-Only Endpoints

Endpoints marked with 🔒 require administrator privileges. Non-administrator users will receive HTTP 403 Forbidden.

## Groups API

Groups allow administrators to organize peers into logical collections for applying policies and routes.

### Create Group 🔒

Create a new group in a network.

**Endpoint:** `POST /api/v1/networks/:networkId/groups`

**Request Body:**
```json
{
  "name": "engineering-team",
  "description": "Engineering team members"
}
```

**Response:** `201 Created`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "engineering-team",
  "description": "Engineering team members",
  "peer_ids": [],
  "policy_ids": [],
  "route_ids": [],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**
- `400 Bad Request` - Invalid request body or duplicate group name
- `403 Forbidden` - Non-administrator user
- `404 Not Found` - Network not found

---

### List Groups 🔒

List all groups in a network.

**Endpoint:** `GET /api/v1/networks/:networkId/groups`

**Response:** `200 OK`
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "network_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "engineering-team",
    "description": "Engineering team members",
    "peer_ids": ["peer-1", "peer-2"],
    "policy_ids": ["policy-1"],
    "route_ids": ["route-1"],
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### Get Group 🔒

Get details of a specific group.

**Endpoint:** `GET /api/v1/networks/:networkId/groups/:groupId`

**Response:** `200 OK`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "engineering-team",
  "description": "Engineering team members",
  "peer_ids": ["peer-1", "peer-2"],
  "policy_ids": ["policy-1"],
  "route_ids": ["route-1"],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**
- `403 Forbidden` - Non-administrator user
- `404 Not Found` - Group or network not found

---

### Update Group 🔒

Update a group's name or description.

**Endpoint:** `PUT /api/v1/networks/:networkId/groups/:groupId`

**Request Body:**
```json
{
  "name": "senior-engineers",
  "description": "Senior engineering team members"
}
```

**Response:** `200 OK`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "senior-engineers",
  "description": "Senior engineering team members",
  "peer_ids": ["peer-1", "peer-2"],
  "policy_ids": ["policy-1"],
  "route_ids": ["route-1"],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T11:00:00Z"
}
```

---

### Delete Group 🔒

Delete a group. Peers in the group are not deleted.

**Endpoint:** `DELETE /api/v1/networks/:networkId/groups/:groupId`

**Response:** `204 No Content`

**Error Responses:**
- `403 Forbidden` - Non-administrator user
- `404 Not Found` - Group or network not found

---

### Add Peer to Group 🔒

Add a peer to a group. Automatically applies group policies and routes.

**Endpoint:** `POST /api/v1/networks/:networkId/groups/:groupId/peers/:peerId`

**Response:** `200 OK`

**Error Responses:**
- `403 Forbidden` - Non-administrator user
- `404 Not Found` - Group, peer, or network not found

---

### Remove Peer from Group 🔒

Remove a peer from a group. Removes group policies and routes from the peer.

**Endpoint:** `DELETE /api/v1/networks/:networkId/groups/:groupId/peers/:peerId`

**Response:** `204 No Content`

**Error Responses:**
- `403 Forbidden` - Non-administrator user
- `404 Not Found` - Group, peer, or network not found

---

## Policies API

Policies define iptables rules applied on jump peers to control traffic filtering.

### Create Policy 🔒

Create a new policy with optional rules.

**Endpoint:** `POST /api/v1/networks/:networkId/policies`

**Request Body:**
```json
{
  "name": "allow-web-traffic",
  "description": "Allow HTTP and HTTPS traffic",
  "rules": [
    {
      "direction": "output",
      "action": "allow",
      "target": "0.0.0.0/0",
      "target_type": "cidr",
      "description": "Allow all outbound traffic"
    },
    {
      "direction": "input",
      "action": "deny",
      "target": "0.0.0.0/0",
      "target_type": "cidr",
      "description": "Deny all inbound traffic"
    }
  ]
}
```

**Response:** `201 Created`
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "allow-web-traffic",
  "description": "Allow HTTP and HTTPS traffic",
  "rules": [
    {
      "id": "rule-1",
      "direction": "output",
      "action": "allow",
      "target": "0.0.0.0/0",
      "target_type": "cidr",
      "description": "Allow all outbound traffic"
    },
    {
      "id": "rule-2",
      "direction": "input",
      "action": "deny",
      "target": "0.0.0.0/0",
      "target_type": "cidr",
      "description": "Deny all inbound traffic"
    }
  ],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Policy Rule Fields:**
- `direction`: "input" or "output"
- `action`: "allow" or "deny"
- `target`: IP/CIDR, peer ID, or group ID
- `target_type`: "cidr", "peer", or "group"

---

### List Policies 🔒

List all policies in a network.

**Endpoint:** `GET /api/v1/networks/:networkId/policies`

**Response:** `200 OK`
```json
[
  {
    "id": "770e8400-e29b-41d4-a716-446655440000",
    "network_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "allow-web-traffic",
    "description": "Allow H and HTTPS traffic",
    "rules": [...],
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### Get Policy 🔒

Get details of a specific policy including all rules.

**Endpoint:** `GET /api/v1/networks/:networkId/policies/:policyId`

**Response:** `200 OK`
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "allow-web-traffic",
  "description": "Allow HTTP and HTTPS traffic",
  "rules": [...],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

---

### Update Policy 🔒

Update a policy's name or description. Triggers iptables regeneration on affected jump peers.

**Endpoint:** `PUT /api/v1/networks/:networkId/policies/:policyId`

**Request Body:**
```json
{
  "name": "updated-policy-name",
  "description": "Updated description"
}
```

**Response:** `200 OK`

---

### Delete Policy 🔒

Delete a policy. Removes from all groups and updates affected jump peers.

**Endpoint:** `DELETE /api/v1/networks/:networkId/policies/:policyId`

**Response:** `204 No Content`

---

### Add Rule to Policy 🔒

Add a new rule to a policy.

**Endpoint:** `POST /api/v1/networks/:networkId/policies/:policyId/rules`

**Request Body:**
```json
{
  "direction": "input",
  "action": "allow",
  "target": "10.0.0.0/24",
  "target_type": "cidr",
  "description": "Allow traffic from internal network"
}
```

**Response:** `201 Created`

---

### Remove Rule from Policy 🔒

Remove a rule from a policy.

**Endpoint:** `DELETE /api/v1/networks/:networkId/policies/:policyId/rules/:ruleId`

**Response:** `204 No Content`

---

### Get Policy Templates 🔒

Get predefined policy templates.

**Endpoint:** `GET /api/v1/networks/:networkId/policies/templates`

**Response:** `200 OK`
```json
[
  {
    "name": "fully-encapsulated",
    "description": "Allow all outbound traffic, deny all inbound traffic",
    "rules": [
      {
        "direction": "output",
        "action": "allow",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Allow all outbound"
      },
      {
        "direction": "input",
        "action": "deny",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Deny all inbound"
      }
    ]
  },
  {
    "name": "isolated",
    "description": "Deny all inbound and outbound traffic",
    "rules": [
      {
        "direction": "input",
        "action": "deny",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Deny all inbound"
      },
      {
        "direction": "output",
        "action": "deny",
        "target": "0.0.0.0/0",
        "target_type": "cidr",
        "description": "Deny all outbound"
      }
    ]
  },
  {
    "name": "default-network",
    "description": "Allow all traffic within network CIDR",
    "rules": [
      {
        "direction": "input",
        "action": "allow",
        "target": "{network_cidr}",
        "target_type": "cidr",
        "description": "Allow inbound from network"
      },
      {
        "direction": "output",
        "action": "allow",
        "target": "{network_cidr}",
        "target_type": "cidr",
        "description": "Allow outbound to network"
      }
    ]
  }
]
```

---

### Attach Policy to Group 🔒

Attach a policy to a group. Applies iptables rules on jump peers for all group members.

**Endpoint:** `POST /api/v1/networks/:networkId/groups/:groupId/policies/:policyId`

**Response:** `200 OK`

---

### Detach Policy from Group 🔒

Detach a policy from a group. Removes iptables rules from jump peers for group members.

**Endpoint:** `DELETE /api/v1/networks/:networkId/groups/:groupId/policies/:policyId`

**Response:** `204 No Content`

---

### Get Group Policies 🔒

Get all policies attached to a group.

**Endpoint:** `GET /api/v1/networks/:networkId/groups/:groupId/policies`

**Response:** `200 OK`
```json
[
  {
    "id": "770e8400-e29b-41d4-a716-446655440000",
    "network_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "allow-web-traffic",
    "description": "Allow HTTP and HTTPS traffic",
    "rules": [...],
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

## Routes API

Routes define network destinations accessible through jump peers, added to WireGuard AllowedIPs.

### Create Route 🔒

Create a new route to an external network.

**Endpoint:** `POST /api/v1/networks/:networkId/routes`

**Request Body:**
```json
{
  "name": "aws-vpc",
  "description": "AWS VPC network",
  "destination_cidr": "172.31.0.0/16",
  "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
  "domain_suffix": "aws.internal"
}
```

**Response:** `201 Created`
```json
{
  "id": "990e8400-e29b-41d4-a716-446655440000",
  "network_id": "660e8400-e29b-41d4-a716-446655440000",
  "name": "aws-vpc",
  "description": "AWS VPC network",
  "destination_cidr": "172.31.0.0/16",
  "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
  "domain_suffix": "aws.internal",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Error Responses:**
- `400 Bad Request` - Invalid CIDR format or jump peer not found
- `403 Forbidden` - Non-administrator user
- `404 Not Found` - Network not found

---

### List Routes 🔒

List all routes in a network.

**Endpoint:** `GET /api/v1/networks/:networkId/routes`

**Response:** `200 OK`
```json
[
  {
    "id": "990e8400-e29b-41d4-a716-446655440000",
    "network_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "aws-vpc",
    "description": "AWS VPC network",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
    "domain_suffix": "aws.internal",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### Get Route 🔒

Get details of a specific route.

**Endpoint:** `GET /api/v1/networks/:networkId/routes/:routeId`

**Response:** `200 OK`

---

### Update Route 🔒

Update a route. Triggers WireGuard config regeneration for affected peers.

**Endpoint:** `PUT /api/v1/networks/:networkId/routes/:routeId`

**Request Body:**
```json
{
  "name": "aws-vpc-updated",
  "description": "Updated AWS VPC network",
  "destination_cidr": "172.31.0.0/16",
  "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
  "domain_suffix": "aws.internal"
}
```

**Response:** `200 OK`

---

### Delete Route 🔒

Delete a route. Removes from all groups and updates affected peer configurations.

**Endpoint:** `DELETE /api/v1/networks/:networkId/routes/:routeId`

**Response:** `204 No Content`

---

### Attach Route to Group 🔒

Attach a route to a group. Adds route CIDR to AllowedIPs for all group members.

**Endpoint:** `POST /api/v1/networks/:networkId/groups/:groupId/routes/:routeId`

**Response:** `200 OK`

---

### Detach Route from Group 🔒

Detach a route from a group. Removes route CIDR from AllowedIPs for group members.

**Endpoint:** `DELETE /api/v1/networks/:networkId/groups/:groupId/routes/:routeId`

**Response:** `204 No Content`

---

### Get Group Routes 🔒

Get all routes attached to a group.

**Endpoint:** `GET /api/v1/networks/:networkId/groups/:groupId/routes`

**Response:** `200 OK`
```json
[
  {
    "id": "990e8400-e29b-41d4-a716-446655440000",
    "network_id": "660e8400-e29b-41d4-a716-446655440000",
    "name": "aws-vpc",
    "description": "AWS VPC network",
    "destination_cidr": "172.31.0.0/16",
    "jump_peer_id": "880e8400-e29b-41d4-a716-446655440000",
    "domain_suffix": "aws.internal",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

## DNS Mappings API

DNS mappings provide name resolution for IP addresses within route networks.

### Create DNS Mapping 🔒

Create a DNS mapping for a route.

**Endpoint:** `POST /api/v1/networks/:networkId/routes/:routeId/dns`

**Request Body:**
```json
{
  "name": "database-server",
  "ip_address": "172.31.10.50"
}
```

**Response:** `201 Created`
```json
{
  "id": "aa0e8400-e29b-41d4-a716-446655440000",
  "route_id": "990e8400-e29b-41d4-a716-446655440000",
  "name": "database-server",
  "ip_address": "172.31.10.50",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**FQDN Format:** `database-server.aws-vpc.aws.internal`

**Error Responses:**
- `400 Bad Request` - IP address not within route CIDR
- `403 Forbidden` - Non-administrator user
- `404 Not Found` - Route not found

---

### List DNS Mappings 🔒

List all DNS mappings for a route.

**Endpoint:** `GET /api/v1/networks/:networkId/routes/:routeId/dns`

**Response:** `200 OK`
```json
[
  {
    "id": "aa0e8400-e29b-41d4-a716-446655440000",
    "route_id": "990e8400-e29b-41d4-a716-446655440000",
    "name": "database-server",
    "ip_address": "172.31.10.50",
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
]
```

---

### Update DNS Mapping 🔒

Update a DNS mapping. Propagates to jump peer DNS servers within 60 seconds.

**Endpoint:** `PUT /api/v1/networks/:networkId/routes/:routeId/dns/:dnsId`

**Request Body:**
```json
{
  "name": "db-primary",
  "ip_address": "172.31.10.51"
}
```

**Response:** `200 OK`

---

### Delete DNS Mapping 🔒

Delete a DNS mapping. Removes from jump peer DNS servers within 60 seconds.

**Endpoint:** `DELETE /api/v1/networks/:networkId/routes/:routeId/dns/:dnsId`

**Response:** `204 No Content`

---

### Get Network DNS Records 🔒

Get all DNS records for a network (peer records + route records).

**Endpoint:** `GET /api/v1/networks/:networkId/dns`

**Response:** `200 OK`
```json
{
  "peer_records": [
    {
      "name": "peer1.mynetwork.internal",
      "ip_address": "10.0.0.2"
    }
  ],
  "route_records": [
    {
      "name": "database-server.aws-vpc.aws.internal",
      "ip_address": "172.31.10.50"
    }
  ]
}
```

---

## Networks API Updates

### Create Network

Networks now support custom domain suffixes and default groups.

**Additional Fields:**
```json
{
  "domain_suffix": "mycompany.internal",
  "default_group_ids": ["group-id-1", "group-id-2"]
}
```

**Default Values:**
- `domain_suffix`: "internal"
- `default_group_ids`: []

---

## Peers API Updates

### Peer Model Changes

**Removed Fields:**
- `is_isolated` - Replaced by policies
- `full_encapsulation` - Replaced by policies

**New Fields:**
- `group_ids`: Array of group IDs the peer belongs to

**Example Peer Response:**
```json
{
  "id": "peer-1",
  "name": "laptop-1",
  "public_key": "...",
  "address": "10.0.0.2",
  "is_jump": false,
  "group_ids": ["group-1", "group-2"],
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

---

## Error Response Format

All error responses follow this format:

```json
{
  "error": "Descriptive error message"
}
```

**Common HTTP Status Codes:**
- `200 OK` - Successful GET/PUT/POST operation
- `201 Created` - Successful resource creation
- `204 No Content` - Successful DELETE operation
- `400 Bad Request` - Invalid request data
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource conflict (e.g., duplicate name)
- `500 Internal Server Error` - Server error

---

## Rate Limiting

API endpoints are rate-limited to prevent abuse:
- 100 requests per minute per user
- 1000 requests per hour per user

Rate limit headers are included in responses:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1642252800
```

---

## Pagination

List endpoints support pagination via query parameters:

**Query Parameters:**
- `page`: Page number (default: 1)
- `page_size`: Items per page (default: 50, max: 100)

**Example:**
```
GET /api/v1/networks/:networkId/groups?page=2&page_size=25
```

**Response includes pagination metadata:**
```json
{
  "data": [...],
  "page": 2,
  "page_size": 25,
  "total": 150
}
```

