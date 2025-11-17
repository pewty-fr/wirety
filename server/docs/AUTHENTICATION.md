# Wirety Authentication & Authorization

## Overview

Wirety supports two operational modes:
1. **No Authentication Mode**: Open access with administrator permissions for all users
2. **OIDC Mode**: OpenID Connect authentication with role-based access control

## Configuration

Authentication is configured via environment variables:

```bash
# Enable/disable authentication
AUTH_ENABLED=true  # Set to "false" for no-auth mode

# OIDC Provider Configuration (Keycloak)
AUTH_ISSUER_URL=https://keycloak.example.com/realms/wirety
AUTH_CLIENT_ID=wirety-server

# Optional: JWKS cache duration in seconds (default: 3600)
AUTH_JWKS_CACHE_TTL=3600
```

## User Roles

### Administrator
- Full access to all resources
- Can create, view, update, and delete networks
- Can manage all peers in all networks
- Can manage users and their permissions
- Can set default permissions for new users

### User
- Limited access based on authorized networks
- Can view authorized networks
- Can create, view, update, and delete **only their own peers** in authorized networks
- Cannot create or delete networks
- Cannot manage other users' peers

## First User Setup

When OIDC authentication is enabled, the **first user to log in** automatically receives **Administrator** role.

This first admin can then:
1. Set default permissions for new users
2. Authorize users to specific networks
3. Promote other users to administrators if needed

## Default Permissions

Administrators can configure default settings for newly registered users:

```json
{
  "default_role": "user",
  "default_authorized_networks": ["network-id-1", "network-id-2"]
}
```

These defaults are applied when a new user logs in for the first time via OIDC.

## API Endpoints

### Public Endpoints (No Authentication Required)
- `GET /api/v1/health` - Health check
- `GET /api/v1/agent/resolve` - Agent enrollment token resolution

### Authenticated Endpoints

All other endpoints require authentication when `AUTH_ENABLED=true`.

#### User Management (Admin Only)
- `GET /api/v1/users` - List all users
- `GET /api/v1/users/me` - Get current user info
- `GET /api/v1/users/:userId` - Get user details
- `PUT /api/v1/users/:userId` - Update user role and permissions
- `DELETE /api/v1/users/:userId` - Delete user
- `GET /api/v1/users/defaults` - Get default permissions
- `PUT /api/v1/users/defaults` - Update default permissions

#### Network Management
- `GET /api/v1/networks` - List networks (shows only authorized networks for regular users)
- `POST /api/v1/networks` - Create network (**Admin only**)
- `GET /api/v1/networks/:networkId` - Get network details (requires network access)
- `PUT /api/v1/networks/:networkId` - Update network (**Admin only**)
- `DELETE /api/v1/networks/:networkId` - Delete network (**Admin only**)

#### Peer Management
- `POST /api/v1/networks/:networkId/peers` - Create peer (requires network access)
  - Admins can create peers for any user
  - Regular users create peers owned by themselves
- `GET /api/v1/networks/:networkId/peers` - List peers (requires network access)
- `GET /api/v1/networks/:networkId/peers/:peerId` - Get peer details (requires network access)
- `PUT /api/v1/networks/:networkId/peers/:peerId` - Update peer (owner or admin only)
- `DELETE /api/v1/networks/:networkId/peers/:peerId` - Delete peer (owner or admin only)
- `GET /api/v1/networks/:networkId/peers/:peerId/config` - Get peer config (owner or admin only)

#### ACL Management (Admin Only)
- `GET /api/v1/networks/:networkId/acl` - Get ACL configuration
- `PUT /api/v1/networks/:networkId/acl` - Update ACL configuration

## Authentication Flow

### Frontend Authentication
1. User accesses the frontend application
2. Frontend redirects to Keycloak login page
3. User authenticates with Keycloak
4. Keycloak returns JWT token to frontend
5. Frontend stores JWT token
6. Frontend includes JWT token in all API requests: `Authorization: Bearer <token>`

### Backend Token Validation
1. Backend receives request with JWT token
2. Extracts token from `Authorization` header
3. Validates token signature using Keycloak's public keys (JWKS)
4. Verifies token claims (issuer, audience, expiration)
5. Extracts user information from token claims
6. Creates or updates user record in database
7. Enforces authorization rules based on user role and permissions

## Keycloak Setup

### 1. Create Realm
Create a new realm in Keycloak (e.g., `wirety`)

### 2. Create Client
1. Create a new client with ID: `wirety-server`
2. Set **Access Type**: `public` (for frontend) or `confidential` (if using client secret)
3. Enable **Standard Flow** and **Implicit Flow** for frontend authentication
4. Set **Valid Redirect URIs** to your frontend URL(s)
5. Set **Web Origins** to allow CORS from frontend

### 3. Client Scopes
Ensure the client includes these scopes:
- `openid` - Required for OIDC
- `profile` - For user profile information
- `email` - For user email

### 4. Token Settings
Configure token lifespans as needed:
- Access Token Lifespan: 5-60 minutes
- Refresh Token Lifespan: 30 days (optional)

### 5. User Attributes
Keycloak should provide these claims in the JWT:
- `sub` - User ID (required)
- `email` - User email (required)
- `name` - Display name
- `preferred_username` - Username
- `email_verified` - Email verification status

## Security Considerations

### Token Validation
- Tokens are validated against Keycloak's public keys (JWKS endpoint)
- JWKS is cached for performance (configurable TTL)
- Token signature, issuer, audience, and expiration are verified

### Peer Ownership
- Each peer has an `owner_id` field
- Regular users can only manage peers they own
- Administrators can manage all peers
- Peer ownership is enforced at the API layer

### Network Authorization
- Users can only access networks they are authorized for
- Authorization is checked on every network-related operation
- Administrators bypass network authorization checks

## No-Auth Mode

When `AUTH_ENABLED=false`:
- All requests are treated as administrator
- No token validation occurs
- A virtual admin user is created for each request
- Useful for development and testing
- **NOT RECOMMENDED FOR PRODUCTION**

## Example Usage

### Get Current User Info
```bash
curl -H "Authorization: Bearer <jwt-token>" \
  https://api.wirety.example.com/api/v1/users/me
```

### Create a Peer (Regular User)
```bash
curl -X POST \
  -H "Authorization: Bearer <jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-device",
    "endpoint": "1.2.3.4:51820",
    "is_jump": false,
    "is_isolated": false,
    "full_encapsulation": false
  }' \
  https://api.wirety.example.com/api/v1/networks/<network-id>/peers
```

### Update User Permissions (Admin Only)
```bash
curl -X PUT \
  -H "Authorization: Bearer <admin-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "role": "user",
    "authorized_networks": ["network-1", "network-2"]
  }' \
  https://api.wirety.example.com/api/v1/users/<user-id>
```

## Troubleshooting

### "invalid token" Error
- Verify `AUTH_ISSUER_URL` matches your Keycloak realm URL
- Ensure token is not expired
- Check that token was issued by the correct Keycloak instance

### "access to this network is not authorized" Error
- Verify user has been granted access to the network
- Check user permissions with `GET /api/v1/users/me`
- Contact administrator to grant network access

### "you can only manage your own peers" Error
- You are trying to modify a peer owned by another user
- Only administrators can manage others' peers
- Verify peer ownership with `GET /api/v1/networks/:networkId/peers/:peerId`

## Migration from No-Auth to OIDC

1. Set up Keycloak with realm and client
2. Update environment variables:
   ```bash
   AUTH_ENABLED=true
   AUTH_ISSUER_URL=https://keycloak.example.com/realms/wirety
   AUTH_CLIENT_ID=wirety-server
   ```
3. Restart the server
4. Have the first administrator log in via frontend
5. Configure default permissions for new users
6. Existing peers created in no-auth mode will have no owner (can be managed by admins only)

## Future Enhancements

- Persistent user storage (currently in-memory)
- Group-based permissions
- Fine-grained peer-level permissions
- Audit logging for authorization events
- Support for multiple OIDC providers
