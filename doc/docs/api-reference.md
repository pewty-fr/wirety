---
id: api-reference
title: API Reference
sidebar_position: 10
---

# API Reference

Complete reference for the Wirety REST API.

## Overview

### Base URL

All endpoints are relative to:

```
/api/v1
```

### Authentication

Most endpoints require authentication. Three mechanisms are accepted and may be combined:

| Mechanism | Header / Cookie | Value |
|-----------|----------------|-------|
| Session hash | `Authorization: Session <hash>` | Obtained from login or OIDC token exchange |
| API token | `Authorization: Bearer wirety_<hex>` | Long-lived personal access token |
| Session cookie | `wirety_session` (HttpOnly cookie) | Set automatically by the server on login |

Session cookies are set automatically when using the login or token exchange endpoints.

### Roles

| Role | Description |
|------|-------------|
| `administrator` | Full access to all resources across all networks |
| `user` | Access limited to authorized networks and own peers |

Endpoints marked **[admin]** require the `administrator` role and return `403 Forbidden` otherwise.

### Error Format

All error responses use:

```json
{ "error": "human-readable error message" }
```

### Pagination

Paginated endpoints accept `page` (default `1`) and `page_size` (default `20`) query parameters and return:

```json
{
  "data": [...],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

---

## Health

### Health Check

Check if the API server is running. No authentication required.

**`GET /health`**

**Response `200`**
```json
{ "status": "ok" }
```

---

## Authentication

### Get Auth Config

Returns the public authentication configuration. No credentials required.

**`GET /auth/config`**

**Response `200`**
```json
{
  "enabled": true,
  "issuer_url": "https://accounts.example.com",
  "client_id": "wirety-app",
  "simple_auth": false
}
```

| Field | Description |
|-------|-------------|
| `enabled` | Whether OIDC authentication is enabled |
| `issuer_url` | OIDC issuer URL (empty when OIDC is disabled) |
| `client_id` | OIDC client ID for the frontend |
| `simple_auth` | `true` when `AUTH_ENABLED=false` — admin/password login is used instead |

---

### Exchange OIDC Token

Exchange an OIDC authorization code for a server-side session. Only available when OIDC is enabled (`enabled: true` in auth config).

**`POST /auth/token`**

**Request Body**
```json
{
  "code": "authorization_code_from_oidc",
  "redirect_uri": "https://app.example.com/callback"
}
```

**Response `200`**
```json
{
  "session_hash": "abc123...",
  "expires_in": 3600
}
```

The server also sets a `wirety_session` HttpOnly cookie. Subsequent requests can use either the `session_hash` value in the `Authorization: Session` header or the cookie.

---

### Login (Simple Auth)

Authenticate with username/password. Only available when OIDC is disabled (`simple_auth: true` in auth config).

**`POST /auth/login`**

**Request Body**
```json
{
  "username": "admin",
  "password": "your-admin-password"
}
```

**Response `200`**
```json
{
  "session_hash": "abc123...",
  "expires_in": 2592000
}
```

The server also sets a `wirety_session` HttpOnly cookie.

---

### Logout

Invalidate the current session.

**`POST /auth/logout`**

**Request Body** (optional — the session is preferably resolved from the cookie or `Authorization` header)
```json
{
  "session_hash": "abc123..."
}
```

**Response `200`**
```json
{ "message": "Logged out successfully" }
```

---

## Users

### Get Current User

Returns the authenticated user's profile.

**`GET /users/me`**

**Response `200`**
```json
{
  "id": "user-sub-from-oidc",
  "email": "alice@example.com",
  "name": "Alice",
  "role": "user",
  "authorized_networks": ["net-id-1", "net-id-2"],
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-10T12:00:00Z",
  "last_login_at": "2024-04-10T08:00:00Z"
}
```

---

### List Users [admin]

Returns all registered users.

**`GET /users`**

**Response `200`** — array of User objects (same shape as above).

---

### Get User [admin]

**`GET /users/:userId`**

**Path Parameters**

| Parameter | Description |
|-----------|-------------|
| `userId` | User ID |

**Response `200`** — User object.

---

### Update User [admin]

Update a user's name, role, or authorized networks.

**`PUT /users/:userId`**

**Request Body**
```json
{
  "name": "Alice Smith",
  "role": "administrator",
  "authorized_networks": ["net-id-1"]
}
```

All fields are optional. **Response `200`** — updated User object.

---

### Delete User [admin]

**`DELETE /users/:userId`**

**Response `204 No Content`**

---

### Get Default Permissions [admin]

Get the default role and network list applied to newly created users.

**`GET /users/defaults`**

**Response `200`**
```json
{
  "default_role": "user",
  "default_authorized_networks": ["net-id-1"]
}
```

---

### Update Default Permissions [admin]

**`PUT /users/defaults`**

**Request Body**
```json
{
  "default_role": "user",
  "default_authorized_networks": ["net-id-1"]
}
```

**Response `200`** — updated DefaultNetworkPermissions object.

---

## API Tokens

Tokens belong to the authenticated user and use the `wirety_` prefix.

### List API Tokens

**`GET /users/me/tokens`**

**Response `200`** — array of token objects (raw token is **not** included in listing).
```json
[
  {
    "id": "token-uuid",
    "name": "ci-bot",
    "created_at": "2024-01-01T00:00:00Z",
    "expires_at": null,
    "last_used_at": "2024-04-10T08:00:00Z"
  }
]
```

---

### Create API Token

**`POST /users/me/tokens`**

**Request Body**
```json
{
  "name": "ci-bot",
  "expires_at": "2025-01-01T00:00:00Z"
}
```

`expires_at` is optional — omit for a non-expiring token.

**Response `201`**
```json
{
  "id": "token-uuid",
  "name": "ci-bot",
  "token": "wirety_deadbeef...",
  "created_at": "2024-04-13T00:00:00Z",
  "expires_at": "2025-01-01T00:00:00Z",
  "last_used_at": null
}
```

**The `token` field is shown exactly once.** Store it securely — it cannot be retrieved again.

---

### Delete API Token

**`DELETE /users/me/tokens/:tokenId`**

**Response `204 No Content`**

---

## Networks

### List Networks

Returns networks accessible to the authenticated user, with pagination and optional filtering.

**`GET /networks`**

**Query Parameters**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page` | `1` | Page number |
| `page_size` | `20` | Items per page (max 200) |
| `filter` | — | Case-insensitive substring filter on name, CIDR, or ID |

**Response `200`**
```json
{
  "data": [
    {
      "id": "net-uuid",
      "name": "office",
      "cidr": "10.10.0.0/16",
      "peer_count": 12,
      "dns": ["1.1.1.1"],
      "domain_suffix": "internal",
      "default_group_ids": [],
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-04-01T00:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

| Field | Description |
|-------|-------------|
| `peer_count` | Number of peers (computed, lightweight) |
| `dns` | Additional DNS servers pushed to peers |
| `domain_suffix` | Internal DNS domain suffix (default: `internal`) |
| `default_group_ids` | Groups automatically assigned to non-admin peers |

---

### Create Network [admin]

**`POST /networks`**

**Request Body**
```json
{
  "name": "office",
  "cidr": "10.10.0.0/16",
  "dns": ["1.1.1.1", "8.8.8.8"],
  "domain_suffix": "internal"
}
```

`dns` and `domain_suffix` are optional. **Response `201`** — Network object.

---

### Get Network

**`GET /networks/:networkId`**

**Response `200`** — Network object.

---

### Update Network [admin]

**`PUT /networks/:networkId`**

**Request Body** (all fields optional)
```json
{
  "name": "office-v2",
  "cidr": "10.20.0.0/16",
  "dns": ["1.1.1.1"],
  "domain_suffix": "corp",
  "default_group_ids": ["group-uuid"]
}
```

**Response `200`** — updated Network object.

---

### Delete Network [admin]

**`DELETE /networks/:networkId`**

**Response `204 No Content`**

---

## Peers

### List Peers

**`GET /networks/:networkId/peers`**

Non-admin users only see peers they own. Supports pagination and filtering.

**Query Parameters**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page` | `1` | Page number |
| `page_size` | `20` | Items per page (max 500) |
| `filter` | — | Substring filter on name, IP address, or ID |

**Response `200`**
```json
{
  "data": [
    {
      "id": "peer-uuid",
      "name": "laptop-alice",
      "public_key": "base64pubkey=",
      "address": "10.10.0.2",
      "endpoint": "203.0.113.5:51820",
      "listen_port": 51820,
      "additional_allowed_ips": ["192.168.1.0/24"],
      "token": "enroll-token-value",
      "is_jump": false,
      "use_agent": true,
      "owner_id": "user-sub-123",
      "group_ids": ["group-uuid"],
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-04-01T00:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

| Field | Description |
|-------|-------------|
| `endpoint` | External IP:port (mainly for jump peers) |
| `listen_port` | WireGuard listen port (mainly for jump peers) |
| `additional_allowed_ips` | Extra CIDRs this peer can route |
| `token` | Agent enrollment token (secret, handle with care) |
| `is_jump` | Whether this peer acts as a hub/jump server |
| `use_agent` | Whether the dynamic agent manages this peer |
| `owner_id` | User ID of the peer owner (empty for admin-created peers) |
| `group_ids` | Groups this peer belongs to |

---

### Create Peer

**`POST /networks/:networkId/peers`**

Non-admin users automatically become the peer owner.

**Request Body**
```json
{
  "name": "laptop-alice",
  "endpoint": "203.0.113.5:51820",
  "listen_port": 51820,
  "is_jump": false,
  "use_agent": true,
  "additional_allowed_ips": ["192.168.1.0/24"]
}
```

All fields except `name` are optional. **Response `201`** — Peer object.

---

### Get Peer

**`GET /networks/:networkId/peers/:peerId`**

Non-admin users can only retrieve their own peers.

**Response `200`** — Peer object.

---

### Update Peer

**`PUT /networks/:networkId/peers/:peerId`**

Non-admin users can only update their own peers. Only admins can change `owner_id`.

**Request Body** (all fields optional)
```json
{
  "name": "laptop-alice-v2",
  "endpoint": "203.0.113.10:51820",
  "listen_port": 51820,
  "additional_allowed_ips": ["192.168.2.0/24"],
  "owner_id": "another-user-id"
}
```

**Response `200`** — updated Peer object.

---

### Delete Peer

**`DELETE /networks/:networkId/peers/:peerId`**

Non-admin users can only delete their own peers.

**Response `204 No Content`**

---

### Get Peer Config

Returns the WireGuard configuration file for a peer.

**`GET /networks/:networkId/peers/:peerId/config`**

Non-admin users can only retrieve their own peer config.

**Response `200`**
```json
{ "config": "[Interface]\nPrivateKey = ...\n..." }
```

---

### Get Peer Session Status

Returns the security session status for a peer (active agent sessions, endpoint changes, suspicious activity).

**`GET /networks/:networkId/peers/:peerId/session`**

**Response `200`**
```json
{
  "peer_id": "peer-uuid",
  "has_active_agent": true,
  "current_session": {
    "peer_id": "peer-uuid",
    "hostname": "laptop-alice",
    "system_uptime": 86400,
    "wireguard_uptime": 3600,
    "reported_endpoint": "203.0.113.5:51820",
    "last_seen": "2024-04-13T10:00:00Z",
    "first_seen": "2024-04-12T09:00:00Z",
    "session_id": "sess-uuid"
  },
  "conflicting_sessions": [],
  "recent_endpoint_changes": [],
  "suspicious_activity": false,
  "last_checked": "2024-04-13T10:05:00Z"
}
```

---

### Get Peer Reachability

Computes which peers, policy rules, and external routes are reachable from a given peer, based on ACL and group/policy configuration.

**`GET /networks/:networkId/peers/:peerId/reachability`**

**Response `200`**
```json
{
  "peer_id": "peer-uuid",
  "peer_name": "laptop-alice",
  "peer_address": "10.10.0.2",
  "is_jump": false,
  "peer_access": [
    {
      "peer_id": "peer-uuid-2",
      "peer_name": "server-bob",
      "address": "10.10.0.3",
      "is_jump": false,
      "allowed": true,
      "reason": "default_allow"
    }
  ],
  "rules": [
    {
      "direction": "output",
      "action": "allow",
      "target_type": "cidr",
      "target": "192.168.1.0/24",
      "addresses": ["192.168.1.0/24"],
      "policy_name": "allow-office",
      "group_name": "engineering",
      "description": "Allow office subnet"
    }
  ],
  "routes": [
    {
      "route_id": "route-uuid",
      "route_name": "office-lan",
      "destination_cidr": "192.168.1.0/24",
      "jump_peer_id": "jump-uuid",
      "jump_peer_name": "jump-server-1",
      "group_name": "engineering"
    }
  ]
}
```

`reason` values for `peer_access`:

| Value | Meaning |
|-------|---------|
| `acl_disabled` | ACL is disabled — all peers can communicate |
| `allow_rule` | Matched an explicit allow rule |
| `deny_rule` | Matched an explicit deny rule |
| `blocked` | Peer is in the blocked list |
| `default_allow` | No rule matched — default is allow |

### Revoke Captive-Portal Authentication

**`POST /networks/:networkId/peers/:peerId/revoke-auth`**

Removes the peer from the captive-portal whitelist across all jump peers in the network. The peer record itself is unchanged — only the authenticated session state is cleared. The next request from the peer will hit the captive portal and SSO is required to regain access.

Authorisation: same as peer management — the peer's owner OR an administrator.

Use cases:
- A peer's WireGuard config is suspected of being shared or stolen.
- Forcing a session to refresh after group/policy changes.
- Rotating credentials.

**Response `204`** — no content.

**Response `403`** — caller is neither the peer's owner nor an administrator.

**Response `404`** — peer not found.

---

## Groups

Groups require `DB_ENABLED=true`. All group endpoints are **[admin]** only.

### List Groups [admin]

**`GET /networks/:networkId/groups`**

**Response `200`** — array of Group objects.

```json
[
  {
    "id": "group-uuid",
    "network_id": "net-uuid",
    "name": "engineering",
    "description": "Engineering team",
    "priority": 100,
    "peer_ids": ["peer-uuid"],
    "policy_ids": ["policy-uuid"],
    "route_ids": ["route-uuid"],
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-04-01T00:00:00Z"
  }
]
```

| Field | Description |
|-------|-------------|
| `priority` | Policy application order — lower value = higher priority (range 1–999) |

---

### Create Group [admin]

**`POST /networks/:networkId/groups`**

**Request Body**
```json
{
  "name": "engineering",
  "description": "Engineering team",
  "priority": 100
}
```

`description` and `priority` are optional (default priority: 100). **Response `201`** — Group object.

---

### Get Group [admin]

**`GET /networks/:networkId/groups/:groupId`**

**Response `200`** — Group object.

---

### Update Group [admin]

**`PUT /networks/:networkId/groups/:groupId`**

**Request Body** (all fields optional)
```json
{
  "name": "engineering-v2",
  "description": "Updated description",
  "priority": 50
}
```

**Response `200`** — updated Group object.

---

### Delete Group [admin]

**`DELETE /networks/:networkId/groups/:groupId`**

**Response `204 No Content`**

---

### Add Peer to Group [admin]

**`POST /networks/:networkId/groups/:groupId/peers/:peerId`**

**Response `200`**

Returns `400 Bad Request` with details if adding the peer would create a circular routing dependency.

---

### Remove Peer from Group [admin]

**`DELETE /networks/:networkId/groups/:groupId/peers/:peerId`**

**Response `204 No Content`**

---

### Get Group Routes [admin]

**`GET /networks/:networkId/groups/:groupId/routes`**

**Response `200`** — array of Route objects attached to the group.

---

## Policies

Policies require `DB_ENABLED=true`. All policy endpoints are **[admin]** only.

### List Policies [admin]

**`GET /networks/:networkId/policies`**

**Response `200`** — array of Policy objects.

```json
[
  {
    "id": "policy-uuid",
    "network_id": "net-uuid",
    "name": "allow-office",
    "description": "Allow access to office subnet",
    "rules": [
      {
        "id": "rule-uuid",
        "direction": "output",
        "action": "allow",
        "target": "192.168.1.0/24",
        "target_type": "cidr",
        "description": "Office LAN"
      }
    ],
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-04-01T00:00:00Z"
  }
]
```

**PolicyRule fields**

| Field | Values |
|-------|--------|
| `direction` | `"input"` or `"output"` |
| `action` | `"allow"` or `"deny"` |
| `target_type` | `"cidr"`, `"peer"`, or `"group"` |
| `target` | CIDR string, peer ID, or group ID depending on `target_type` |

---

### Create Policy [admin]

**`POST /networks/:networkId/policies`**

**Request Body**
```json
{
  "name": "allow-office",
  "description": "Allow access to office subnet",
  "rules": [
    {
      "direction": "output",
      "action": "allow",
      "target": "192.168.1.0/24",
      "target_type": "cidr",
      "description": "Office LAN"
    }
  ]
}
```

`description` and `rules` are optional. **Response `201`** — Policy object.

---

### Get Policy [admin]

**`GET /networks/:networkId/policies/:policyId`**

**Response `200`** — Policy object.

---

### Update Policy [admin]

**`PUT /networks/:networkId/policies/:policyId`**

**Request Body** (all fields optional)
```json
{
  "name": "allow-office-v2",
  "description": "Updated description"
}
```

**Response `200`** — updated Policy object.

---

### Delete Policy [admin]

**`DELETE /networks/:networkId/policies/:policyId`**

**Response `204 No Content`**

---

### Add Rule to Policy [admin]

**`POST /networks/:networkId/policies/:policyId/rules`**

**Request Body**
```json
{
  "direction": "output",
  "action": "allow",
  "target": "192.168.1.0/24",
  "target_type": "cidr",
  "description": "Office LAN"
}
```

**Response `201`** — PolicyRule object.

---

### Remove Rule from Policy [admin]

**`DELETE /networks/:networkId/policies/:policyId/rules/:ruleId`**

**Response `204 No Content`**

---

### Attach Policy to Group [admin]

**`POST /networks/:networkId/groups/:groupId/policies/:policyId`**

**Response `200`**

---

### Detach Policy from Group [admin]

**`DELETE /networks/:networkId/groups/:groupId/policies/:policyId`**

**Response `204 No Content`**

---

### Get Group Policies [admin]

**`GET /networks/:networkId/groups/:groupId/policies`**

**Response `200`** — array of Policy objects attached to the group.

---

### Reorder Group Policies [admin]

Set the evaluation order of policies within a group. The array must contain all policy IDs currently attached to the group.

**`PUT /networks/:networkId/groups/:groupId/policies/order`**

**Request Body**
```json
{
  "policy_ids": ["policy-uuid-1", "policy-uuid-2"]
}
```

**Response `200`**
```json
{ "message": "Policies reordered successfully" }
```

---

## Routes

Routes require `DB_ENABLED=true`. All route endpoints are **[admin]** only.

### List Routes [admin]

**`GET /networks/:networkId/routes`**

**Response `200`** — array of Route objects.

```json
[
  {
    "id": "route-uuid",
    "network_id": "net-uuid",
    "name": "office-lan",
    "description": "Office internal network",
    "destination_cidr": "192.168.1.0/24",
    "jump_peer_id": "jump-uuid",
    "domain_suffix": "office.internal",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-04-01T00:00:00Z"
  }
]
```

---

### Create Route [admin]

**`POST /networks/:networkId/routes`**

**Request Body**
```json
{
  "name": "office-lan",
  "description": "Office internal network",
  "destination_cidr": "192.168.1.0/24",
  "jump_peer_id": "jump-uuid",
  "domain_suffix": "office.internal"
}
```

`description` and `domain_suffix` are optional. **Response `201`** — Route object.

---

### Get Route [admin]

**`GET /networks/:networkId/routes/:routeId`**

**Response `200`** — Route object.

---

### Update Route [admin]

**`PUT /networks/:networkId/routes/:routeId`**

**Request Body** (all fields optional)
```json
{
  "name": "office-lan-v2",
  "description": "Updated description",
  "destination_cidr": "192.168.2.0/24",
  "jump_peer_id": "jump-uuid-2",
  "domain_suffix": "corp.internal"
}
```

**Response `200`** — updated Route object.

---

### Delete Route [admin]

**`DELETE /networks/:networkId/routes/:routeId`**

**Response `204 No Content`**

---

### Attach Route to Group [admin]

**`POST /networks/:networkId/groups/:groupId/routes/:routeId`**

**Response `200`**

Returns `400 Bad Request` with details if attaching would create a circular routing dependency.

---

### Detach Route from Group [admin]

**`DELETE /networks/:networkId/groups/:groupId/routes/:routeId`**

**Response `204 No Content`**

---

## DNS Mappings

DNS mappings require `DB_ENABLED=true`. All DNS endpoints are **[admin]** only.

DNS mappings resolve hostnames within a route's CIDR to specific IP addresses. The FQDN format is `<name>.<route_name>.<domain_suffix>`.

### List DNS Mappings [admin]

**`GET /networks/:networkId/routes/:routeId/dns`**

**Response `200`** — array of DNSMapping objects.

```json
[
  {
    "id": "dns-uuid",
    "route_id": "route-uuid",
    "name": "server1",
    "ip_address": "192.168.1.10",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-04-01T00:00:00Z"
  }
]
```

---

### Create DNS Mapping [admin]

**`POST /networks/:networkId/routes/:routeId/dns`**

**Request Body**
```json
{
  "name": "server1",
  "ip_address": "192.168.1.10"
}
```

The `name` field must follow DNS label rules (alphanumeric + hyphens, max 63 characters). **Response `201`** — DNSMapping object.

---

### Update DNS Mapping [admin]

**`PUT /networks/:networkId/routes/:routeId/dns/:dnsId`**

**Request Body** (all fields optional)
```json
{
  "name": "server1-new",
  "ip_address": "192.168.1.11"
}
```

**Response `200`** — updated DNSMapping object.

---

### Delete DNS Mapping [admin]

**`DELETE /networks/:networkId/routes/:routeId/dns/:dnsId`**

**Response `204 No Content`**

---

### Get Network DNS Records [admin]

Returns all DNS records for a network (both peer-based and route mapping-based).

**`GET /networks/:networkId/dns`**

**Response `200`** — array of DNS record objects.

```json
[
  {
    "name": "laptop-alice",
    "ip_address": "10.10.0.2",
    "fqdn": "laptop-alice.office.internal",
    "type": "peer"
  },
  {
    "name": "server1",
    "ip_address": "192.168.1.10",
    "fqdn": "server1.office-lan.office.internal",
    "type": "route"
  }
]
```

| `type` | Source |
|--------|--------|
| `"peer"` | Peer in the network |
| `"route"` | DNS mapping attached to a route |

---

## ACL

ACL endpoints are **[admin]** only.

The ACL is a lightweight allow/deny layer operating at the peer-to-peer level. It complements group policies. When disabled, all peers can communicate freely.

### Get ACL [admin]

**`GET /networks/:networkId/acl`**

**Response `200`**
```json
{
  "id": "acl-uuid",
  "name": "default",
  "enabled": true,
  "blocked_peers": {
    "peer-uuid-bad": true
  },
  "rules": [
    {
      "id": "rule-uuid",
      "source_peer": "*",
      "target_peer": "peer-uuid-3",
      "action": "deny",
      "description": "Block access to server"
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `enabled` | When `false`, ACL is bypassed and all peers can communicate |
| `blocked_peers` | Map of peer IDs that are unconditionally blocked |
| `rules` | Ordered list of rules; first match wins; default is allow |
| `source_peer` / `target_peer` | Peer ID or `"*"` for all peers |

---

### Update ACL [admin]

Replaces the entire ACL configuration.

**`PUT /networks/:networkId/acl`**

**Request Body** — full ACL object (same structure as GET response).

**Response `200`** — updated ACL object. Notifies connected agents via WebSocket.

---

## IPAM

### Suggest Available CIDRs

Suggests one or more CIDRs of the right size for a target number of peers, carved from a base CIDR. Useful when planning a new network.

**`GET /ipam/available-cidrs`**

**Query Parameters**

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `max_peers` | Yes | — | Minimum number of usable host addresses needed |
| `count` | No | `1` | Number of CIDRs to return (max 20) |
| `base_cidr` | No | `10.0.0.0/8` | Root CIDR to carve from |

**Response `200`**
```json
{
  "base_cidr": "10.0.0.0/8",
  "requested_max_peers": 50,
  "suggested_prefix": 26,
  "usable_hosts": 62,
  "cidrs": ["10.0.0.0/26", "10.0.0.64/26"]
}
```

---

### List IPAM Allocations

Returns all IP allocations across accessible networks, with pagination and filtering.

**`GET /ipam`**

Non-admin users only see peers they own.

**Query Parameters**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `page` | `1` | Page number |
| `page_size` | `20` | Items per page (max 100) |
| `filter` | — | Substring filter on network name, IP, or peer name |

**Response `200`**
```json
{
  "data": [
    {
      "network_id": "net-uuid",
      "network_name": "office",
      "network_cidr": "10.10.0.0/16",
      "ip": "10.10.0.2",
      "peer_id": "peer-uuid",
      "peer_name": "laptop-alice",
      "allocated": true
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

---

### Get Network IPAM Allocations

**`GET /ipam/networks/:networkId`**

**Response `200`** — array of IPAMAllocation objects for the specified network.

---

## Sessions

### List Network Sessions

Returns active agent sessions in a network.

**`GET /networks/:networkId/sessions`**

**Response `200`** — array of AgentSession objects.

```json
[
  {
    "peer_id": "peer-uuid",
    "hostname": "laptop-alice",
    "system_uptime": 86400,
    "wireguard_uptime": 3600,
    "reported_endpoint": "203.0.113.5:51820",
    "last_seen": "2024-04-13T10:00:00Z",
    "first_seen": "2024-04-12T09:00:00Z",
    "session_id": "sess-uuid"
  }
]
```

---

## Agent Enrollment

### Resolve Agent Token

Exchange a peer enrollment token for the peer's identifiers and WireGuard configuration. This endpoint is unauthenticated (uses the token itself as authentication via `Authorization: Bearer`).

**`GET /agent/resolve`**

**Headers**

```
Authorization: Bearer <enrollment-token>
```

**Response `200`**
```json
{
  "network_id": "net-uuid",
  "peer_id": "peer-uuid",
  "peer_name": "laptop-alice",
  "config": "[Interface]\nPrivateKey = ...\n..."
}
```

Returns `404 Not Found` if the token is invalid or expired.

---

## MCP Server

Wirety exposes a Model Context Protocol (MCP) server at `/api/v1/mcp` using the SSE transport. Both `GET` (stream) and `POST` (messages) at the same path are supported. Authentication is required (same session/token mechanisms as other endpoints).

See the [MCP documentation](./mcp.md) for the list of available tools.

---

## WebSocket

### WebSocket (Token-based)

**`GET /ws`**

Connect with a short-lived WebSocket token. Delivers real-time peer configuration updates.

### WebSocket (Legacy)

**`GET /ws/:networkId/:peerId`**

Peer-specific WebSocket connection using network and peer IDs directly.

---

## Captive Portal

### Create Captive Portal Token

Called by a jump peer agent (authenticated via enrollment token) when a new peer connects and needs to authenticate.

**`POST /captive-portal/token`**

**Headers**

```
Authorization: Bearer <jump-peer-enrollment-token>
```

**Request Body**
```json
{ "peer_ip": "10.10.0.5" }
```

**Response `201`** — captive portal token object.

Requires OIDC authentication to be enabled (`AUTH_ENABLED=true`).

---

### Authenticate Captive Portal

Validates the captive portal token against the current user session and whitelists the peer.

**`POST /captive-portal/authenticate`**

Requires the `wirety_session` cookie to be present (set during login).

**Request Body**
```json
{ "captive_token": "captive-token-value" }
```

**Response `200`**
```json
{
  "peer_ip": "10.10.0.5",
  "network_id": "net-uuid",
  "whitelisted": true
}
```
