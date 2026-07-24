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
By default Wirety requests `openid profile email offline_access`:

- `openid profile email` — user identification (email logged when resolving incidents).
- `offline_access` — makes the provider issue a **refresh token**. Without one the session cannot auto-renew and ends as soon as the first id_token expires (often just minutes). Providers such as Azure Entra ID and Dex never issue a refresh token unless this scope is requested.

Override the list with `AUTH_SCOPES` (space-separated) if your provider rejects unknown scopes. Two providers are handled automatically:

- **Google** rejects `offline_access` (it is excluded from the default for Google) and instead needs `AUTH_AUTHORIZATION_EXTRA_PARAMS=access_type=offline&prompt=consent` to issue a refresh token.
- **Slack** rejects `offline_access` (also excluded automatically); enable token rotation in the Slack app settings instead.

Note: with `offline_access`, some providers add a line to the consent screen (Entra ID shows "Maintain access to data you have given it access to").

## Verification
- Login flow redirects to provider.
- User email appears in incident resolution audit.
- The server does NOT log `no refresh token received` (Warn) after login — if it does, sessions will end at id_token expiry (see Scopes above).

## Troubleshooting
| Symptom | Cause | Fix |
|---------|-------|-----|
| 404 on callback | Redirect URI mismatch | Update provider config |
| Silent login failure | Clock skew | Sync server time |
| Email missing | Scope not granted | Add `email` scope |
| Logged out after a few minutes | No refresh token issued (`offline_access` missing/rejected) | Check `AUTH_SCOPES`; for Google set `AUTH_AUTHORIZATION_EXTRA_PARAMS=access_type=offline&prompt=consent`; for Slack enable token rotation |
