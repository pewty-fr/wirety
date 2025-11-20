# SSO Authentication Implementation

## Overview

The frontend now supports SSO authentication using OpenID Connect (OIDC) via the backend API.

## Features

### Authentication Modes

1. **No-Auth Mode** (Development)
   - When `AUTH_ENABLED=false` on the server
   - Automatically logs in as default administrator
   - No login page required

2. **SSO Mode** (Production)
   - When `AUTH_ENABLED=true` on the server
   - Uses OpenID Connect (Keycloak or compatible provider)
   - Redirects to provider login page
   - Exchanges authorization code for access token

### Components

#### AuthContext (`/src/contexts/AuthContext.tsx`)
- Manages authentication state
- Fetches auth configuration from `/api/v1/auth/config`
- Handles OAuth callback and token exchange via `/api/v1/auth/token`
- Stores access token in localStorage
- Provides user information and authentication status

#### LoginPage (`/src/pages/auth/LoginPage.tsx`)
- Displays SSO login button when auth is enabled
- Auto-redirects to dashboard in no-auth mode
- Handles OAuth redirect flow

#### ProfilePage (`/src/pages/profile/ProfilePage.tsx`)
- Displays current user information
- Shows role (Administrator/User)
- Lists authorized networks (for non-admin users)
- Shows authentication mode and provider
- Provides sign-out functionality

### API Integration

#### Auth Endpoints

- `GET /api/v1/auth/config` - Fetch authentication configuration
  - Response: `{ enabled: boolean, issuer_url: string, client_id: string }`

- `POST /api/v1/auth/token` - Exchange authorization code for access token
  - Request: `{ code: string, redirect_uri: string }`
  - Response: `{ access_token: string, token_type: string, expires_in: number }`

#### User Endpoints

- `GET /api/v1/users/me` - Get current user information
  - Requires: `Authorization: Bearer <token>` header
  - Response: User object with id, email, name, role, authorized_networks

#### Automatic Token Injection

The API client automatically adds the `Authorization` header to all requests when a token is present in localStorage.

## Usage

### Development (No-Auth Mode)

```bash
# In server directory
AUTH_ENABLED=false go run cmd/server/main.go

# In front-2 directory
npm run dev
```

Access http://localhost:5173 - you'll be automatically logged in as administrator.

### Production (SSO Mode)

```bash
# Configure server with Keycloak
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://keycloak.example.com/realms/wirety
AUTH_CLIENT_ID=wirety-server
AUTH_CLIENT_SECRET=your-secret

# Start server
go run cmd/server/main.go

# Start frontend
npm run dev
```

1. Access http://localhost:5173
2. Click "Sign in with SSO"
3. Redirected to Keycloak login page
4. After authentication, redirected back with authorization code
5. Frontend exchanges code for access token
6. User is logged in and can access the application

### Profile Page

Access `/profile` to view:
- User name and email
- Role (Administrator or User)
- Authorized networks
- Authentication provider information
- Sign out button

## Navigation

The sidebar now includes:
- User profile section at the bottom (shows name and email)
- Clicking on profile section navigates to `/profile` page
- Theme toggle (Light/Dark/System)

## Security

- Access tokens stored in localStorage
- Automatically included in all API requests via Authorization header
- Token validation handled by backend
- Expired tokens automatically cleared on 401 responses
- Sign out clears token and redirects to Keycloak logout (in SSO mode)

## Flow Diagram

```
┌─────────────┐
│   Browser   │
└──────┬──────┘
       │
       │ 1. Load app
       ▼
┌─────────────────┐
│  AuthContext    │
│  - Fetch config │
│  - Check token  │
└──────┬──────────┘
       │
       ├─── No auth enabled ──▶ Auto-login as admin ──▶ Dashboard
       │
       └─── Auth enabled ──▶ Has token? ───┬─── Yes ──▶ Fetch user ──▶ Dashboard
                                            │
                                            └─── No ──▶ LoginPage
                                                        │
                                                        │ 2. Click "Sign in"
                                                        ▼
                                                  ┌──────────────┐
                                                  │   Keycloak   │
                                                  │    Login     │
                                                  └──────┬───────┘
                                                         │
                                                         │ 3. Authenticate
                                                         ▼
                                                  Redirect with code
                                                         │
                                                         │ 4. Exchange code
                                                         ▼
                                                  Store access token
                                                         │
                                                         │ 5. Fetch user
                                                         ▼
                                                     Dashboard
```

## First-Time Setup

When using SSO mode:
1. The first user to log in becomes an administrator
2. Administrators can then manage user permissions
3. New users get default permissions configured by admin

See server documentation (`/server/docs/AUTHENTICATION.md`) for more details on user management.
