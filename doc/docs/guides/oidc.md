---
id: oidc
title: Server - OIDC
sidebar_position: 3
---

Goal: Connect Wirety Server to an OIDC provider for authenticated UI/API access.

## Steps
1. Create OIDC application (Client ID/Secret, redirect URIs) in provider.
2. Configure environment variables:
```bash
AUTH_ENABLED=true
AUTH_ISSUER_URL=https://keycloak.example.com/realms/wirety
AUTH_CLIENT_ID=wirety-client
AUTH_CLIENT_SECRET=your-client-secret
AUTH_JWKS_CACHE_TTL=3600  # Optional: JWKS cache duration in seconds (default: 3600)
```
3. Restart server deployment.
4. Frontend redirects unauthenticated users to provider; token stored in session.

## Scopes
Request `openid profile email` for user identification (email logged when resolving incidents).

## Verification
- Login flow redirects to provider.
- User email appears in incident resolution audit.

## Troubleshooting
| Symptom | Cause | Fix |
|---------|-------|-----|
| 404 on callback | Redirect URI mismatch | Update provider config |
| Silent login failure | Clock skew | Sync server time |
| Email missing | Scope not granted | Add `email` scope |
