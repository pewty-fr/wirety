# OpenID Connect Authentication Implementation Summary

## Overview

Successfully implemented a dual-mode authentication system for Wirety Server supporting both no-authentication mode (for development) and OpenID Connect authentication (for production with Keycloak).

## What Was Implemented

### 1. Domain Layer (`internal/domain/auth/`)

**Files Created:**
- `user.go` - User domain model with roles (Administrator, User) and permission checking methods
- `claims.go` - OIDC JWT claims structure
- `repository.go` - Repository interface for user persistence

**Key Features:**
- Two user roles: Administrator and User
- Permission methods: `IsAdministrator()`, `HasNetworkAccess()`, `CanManagePeer()`
- Support for per-user authorized networks
- Default permissions configuration for new users

### 2. Application Layer (`internal/application/auth/`)

**Files Created:**
- `service.go` - OIDC token validation and user management service
- `jwk.go` - JWK to RSA public key conversion utilities

**Key Features:**
- JWT token validation against Keycloak's JWKS endpoint
- Token issuer and audience verification
- JWKS caching to reduce load on Keycloak
- Automatic user creation on first login
- First user automatically gets Administrator role
- Default permissions applied to new users

### 3. Infrastructure Layer

**Configuration (`internal/config/config.go`):**
- `AUTH_ENABLED` - Enable/disable OIDC authentication
- `AUTH_ISSUER_URL` - Keycloak realm URL
- `AUTH_CLIENT_ID` - OIDC client identifier
- `AUTH_JWKS_CACHE_TTL` - JWKS cache duration
- `AUTH_SCOPES` - OAuth scopes requested at login (default: `openid profile email offline_access`; `offline_access` is omitted automatically for Google and Slack, which reject it)
- `AUTH_AUTHORIZATION_EXTRA_PARAMS` - Extra query params appended to the authorization URL (Google needs `access_type=offline&prompt=consent` for refresh tokens)

**Repository (`internal/adapters/db/memory/user_repository.go`):**
- In-memory user storage
- User lookup by ID and email
- Default permissions management

**Middleware (`internal/adapters/api/middleware/auth.go`):**
- `AuthMiddleware` - Main authentication middleware with dual mode support
- `RequireAdmin` - Requires administrator role
- `RequireNetworkAccess` - Requires access to specific network
- `GetUserFromContext` - Helper to extract user from request context

### 4. API Handlers

**User Management (`internal/adapters/api/user_handlers.go`):**
- `GET /api/v1/users/me` - Get current user
- `GET /api/v1/users` - List all users (admin only)
- `GET /api/v1/users/:userId` - Get user details (admin only)
- `PUT /api/v1/users/:userId` - Update user (admin only)
- `DELETE /api/v1/users/:userId` - Delete user (admin only)
- `GET /api/v1/users/defaults` - Get default permissions (admin only)
- `PUT /api/v1/users/defaults` - Update default permissions (admin only)

**Updated Handlers (`internal/adapters/api/handler.go`):**
- Added peer ownership tracking (`OwnerID` field)
- CreatePeer - Sets owner for non-admin users
- UpdatePeer - Checks ownership before allowing updates
- DeletePeer - Checks ownership before allowing deletion
- Reorganized routes with proper authentication requirements

### 5. Route Protection

**Public Endpoints (No Auth):**
- `/api/v1/health` - Health check
- `/api/v1/agent/resolve` - Agent enrollment token resolution
- `/api/v1/ws` - WebSocket for agents (token-based)
- `/api/v1/ws/:networkId/:peerId` - Legacy WebSocket (token-based)

**Protected Endpoints (OIDC Required):**
- User management routes
- Network CRUD operations
- Peer management
- ACL configuration
- IPAM operations

**Permission Levels:**
- Some operations require Administrator role
- Network operations require network access
- Peer operations check ownership

### 6. Updated Domain Model

**Peer (`internal/domain/network/peer.go`):**
- Added `OwnerID` field to track peer ownership
- Empty owner ID means admin-created peer
- Non-empty owner ID restricts management to owner or admins

### 7. Main Application (`cmd/main.go`)

**Updates:**
- Load configuration from environment variables
- Initialize user repository
- Initialize authentication service (when enabled)
- Setup authentication middleware
- Pass middleware to route registration
- Conditional behavior based on AUTH_ENABLED

## Authentication Modes

### No-Auth Mode (`AUTH_ENABLED=false`)
- All requests treated as administrator
- No token validation
- Suitable for development and testing
- Frontend can skip authentication flow

### OIDC Mode (`AUTH_ENABLED=true`)
- Frontend authenticates with Keycloak
- Frontend sends JWT token in `Authorization: Bearer <token>` header
- Server validates token against Keycloak
- User created/updated automatically on login
- First user becomes administrator
- Subsequent users get default permissions

## Agent Authentication

Agents (WireGuard peers) use a separate authentication mechanism:
- Enrollment tokens (not OIDC JWT tokens)
- Token embedded in peer configuration
- Agent endpoints are public (not protected by OIDC middleware)
- WebSocket connections use token parameter for authentication

## User Workflow

### First Admin Setup
1. Enable OIDC mode
2. First user logs in via Keycloak
3. User automatically gets Administrator role
4. Admin configures default permissions for new users
5. Admin authorizes users to specific networks

### Regular User
1. User logs in via Keycloak
2. Frontend gets JWT token
3. User is created with default permissions
4. User can access authorized networks
5. User can create and manage own peers

### Administrator
1. Has access to all networks
2. Can manage all peers
3. Can create/delete networks
4. Can manage users and permissions
5. Can configure default settings

## Session Refresh and the offline_access Scope

Sessions are kept alive by refreshing the id_token with the refresh token stored
in the server-side session (`middleware/auth.go`). Per OIDC Core §11, most
providers (Azure Entra ID, Dex, ...) only issue a refresh token when the
`offline_access` scope is requested at login — which is why it is part of the
default `AUTH_SCOPES`. Without it, `session.RefreshToken` stays empty and the
session is deleted as soon as the first id_token expires (often minutes), which
users experience as constant logouts. The server logs a Warn
(`no refresh token received`) at token exchange when this happens.

Provider quirks:
- **Google** rejects `offline_access` (excluded automatically) and needs
  `AUTH_AUTHORIZATION_EXTRA_PARAMS="access_type=offline&prompt=consent"` instead.
- **Slack** rejects `offline_access` (excluded automatically); enable token
  rotation in the Slack app settings to get rotating `xoxe-1-...` refresh tokens.
- **Keycloak** issues session-bound refresh tokens even without `offline_access`;
  with it, sessions use long-lived offline tokens.
- On the refresh grant itself, `offline_access` is stripped
  (`AuthConfig.RefreshScopes`) — it only has meaning on the authorization request,
  and RFC 6749 §6 forbids requesting scopes beyond the original grant, which
  keeps sessions created before this scope was added refreshable.

## Security Features

- JWT signature validation using Keycloak's public keys
- Token issuer verification
- Token audience verification (if client ID configured)
- Token expiration checking
- JWKS caching with configurable TTL
- Role-based access control (RBAC)
- Resource-level permissions (network access)
- Owner-based peer access control

## Dependencies Added

- `github.com/golang-jwt/jwt/v5` - JWT parsing and validation

## Testing Recommendations

1. **No-Auth Mode**: Verify all endpoints work without authentication
2. **OIDC Mode**: 
   - Test first user gets admin role
   - Test subsequent users get default permissions
   - Test network authorization enforcement
   - Test peer ownership enforcement
   - Test admin can override ownership
3. **Agent**: Verify agents still work with token-based auth
4. **Token Validation**: Test with expired tokens, invalid issuers, etc.

## Next Steps

1. Update frontend to handle authentication:
   - Integrate Keycloak authentication
   - Store and send JWT tokens
   - Handle token expiration/refresh
   - Show appropriate UI based on user role

2. Consider additional features:
   - Token refresh mechanism
   - Session management
   - Audit logging for authorization decisions
   - More granular permissions (per-peer ACLs)

3. Production deployment:
   - Set up Keycloak server
   - Configure HTTPS
   - Set secure AUTH_ISSUER_URL
   - Configure token expiration policies

## Files Modified/Created

### Created:
- `internal/domain/auth/user.go`
- `internal/domain/auth/claims.go`
- `internal/domain/auth/repository.go`
- `internal/config/config.go`
- `internal/application/auth/service.go`
- `internal/application/auth/jwk.go`
- `internal/adapters/db/memory/user_repository.go`
- `internal/adapters/api/middleware/auth.go`
- `internal/adapters/api/user_handlers.go`

### Modified:
- `cmd/main.go` - Added auth initialization and middleware setup
- `internal/adapters/api/handler.go` - Updated route registration and peer handlers
- `internal/domain/network/peer.go` - Added OwnerID field
- `internal/application/network/service.go` - Updated AddPeer to accept ownerID
- `go.mod` - Added JWT library dependency

## Configuration

See `.env.example` for configuration options.
