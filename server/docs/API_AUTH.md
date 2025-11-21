# API Authentication Quick Reference

## HTTP Headers

### For Web/Mobile Clients (OIDC)

All protected API endpoints require the Authorization header:

```
Authorization: Bearer <jwt-token-from-keycloak>
```

### For Agents (Token-based)

Agent endpoints use query parameters, not HTTP headers:

```
GET /api/v1/agent/resolve?token=<enrollment-token>
GET /api/v1/ws?token=<enrollment-token>
```

## Endpoint Authentication Requirements

### Public Endpoints (No Auth Required)

```
GET  /api/v1/health
GET  /api/v1/agent/resolve?token=xxx
GET  /api/v1/ws?token=xxx
GET  /api/v1/ws/:networkId/:peerId  (legacy)
```

### Protected Endpoints (Requires OIDC JWT)

#### User Management
```
GET  /api/v1/users/me                    # Any authenticated user
GET  /api/v1/users                       # Admin only
GET  /api/v1/users/:userId               # Admin only
PUT  /api/v1/users/:userId               # Admin only
DELETE /api/v1/users/:userId             # Admin only
GET  /api/v1/users/defaults              # Admin only
PUT  /api/v1/users/defaults              # Admin only
```

#### Networks
```
GET  /api/v1/networks                    # Authorized users (see their networks)
POST /api/v1/networks                    # Admin only
GET  /api/v1/networks/:networkId         # Requires network access
PUT  /api/v1/networks/:networkId         # Admin only
DELETE /api/v1/networks/:networkId       # Admin only
```

#### Peers
```
POST   /api/v1/networks/:networkId/peers              # Requires network access
GET    /api/v1/networks/:networkId/peers              # Requires network access
GET    /api/v1/networks/:networkId/peers/:peerId      # Requires network access
PUT    /api/v1/networks/:networkId/peers/:peerId      # Owner or admin only
DELETE /api/v1/networks/:networkId/peers/:peerId      # Owner or admin only
GET    /api/v1/networks/:networkId/peers/:peerId/config  # Requires network access
```

#### ACL
```
GET  /api/v1/networks/:networkId/acl     # Admin only
PUT  /api/v1/networks/:networkId/acl     # Admin only
```

#### IPAM
```
GET  /api/v1/ipam/available-cidrs                  # Any authenticated user
GET  /api/v1/ipam                                  # Any authenticated user
GET  /api/v1/ipam/networks/:networkId              # Requires network access
```

## Example cURL Commands

### No-Auth Mode

```bash
# All requests work without authentication
curl http://localhost:8080/api/v1/networks
```

### OIDC Mode

```bash
# First, get token from Keycloak
# (This is typically done by your frontend)
TOKEN=$(curl -X POST "https://keycloak.example.com/realms/wirety/protocol/openid-connect/token" \
  -d "client_id=wirety-server" \
  -d "username=user@example.com" \
  -d "password=password" \
  -d "grant_type=password" | jq -r '.access_token')

# Use token in API requests
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/networks

# Create a peer (auto-owned by user)
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-laptop","endpoint":"1.2.3.4:51820"}' \
  http://localhost:8080/api/v1/networks/net-123/peers
```

### Agent Endpoints

```bash
# Agent resolves its identity
curl "http://localhost:8080/api/v1/agent/resolve?token=enrollment-token-here"

# Agent connects to WebSocket
wscat -c "ws://localhost:8080/api/v1/ws?token=enrollment-token-here"
```

## Response Codes

- `200 OK` - Request successful
- `201 Created` - Resource created successfully
- `204 No Content` - Deletion successful
- `400 Bad Request` - Invalid request data
- `401 Unauthorized` - Missing or invalid authentication token
- `403 Forbidden` - Insufficient permissions for the operation
- `404 Not Found` - Resource not found
- `500 Internal Server Error` - Server error

## Error Responses

All errors return JSON:

```json
{
  "error": "description of the error"
}
```

### Common Authentication Errors

```json
{"error": "missing authorization header"}
{"error": "invalid authorization header format"}
{"error": "invalid token: <reason>"}
{"error": "administrator role required"}
{"error": "access to this network is not authorized"}
{"error": "you can only manage your own peers"}
```
