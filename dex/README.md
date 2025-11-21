# Dex – Local OIDC Identity Provider for Wirety (Test Setup)

This directory provides a lightweight local [Dex](https://github.com/dexidp/dex) OpenID Connect provider for developing and testing Wirety's authentication flow.

## Features
- SQLite storage (persistent via Docker volume)
- Single static client: `wirety`
- Single test user (password DB)
- Ready-to-run Docker & Compose setup
- Emits ID Tokens/JWKS consumable by Wirety server

## Configuration
Main config: `config.yaml`

Key sections:
- `issuer`: `http://localhost:5556/dex` – must match Wirety `AUTH_ISSUER_URL`
- `staticClients`: defines OIDC client used by Wirety
- `staticPasswords`: one demo user for password authentication

### Static Client
```
id: wirety
secret: wirety-secret
redirectURIs:
	- http://localhost:8080/callback
	- http://localhost:8080/oauth/callback
```
Adjust redirect URIs to what your Wirety frontend/server expects for login completion.

### Test User
```
email: test@example.com
username: test
password: password (bcrypt hash stored in config)
```

```
email: test2@example.com
username: test2
password: password (bcrypt hash stored in config)
```

## Running Dex

### Using Docker Compose
```bash
cd dex
docker compose build
docker compose up -d
```

Dex will be available at: `http://localhost:5556/dex`
JWKS endpoint: `http://localhost:5556/dex/keys`
Authorization endpoint: `http://localhost:5556/dex/auth`
Token endpoint: `http://localhost:5556/dex/token`

### Logs
```bash
docker logs -f dex
```

## Wirety Server Environment
Start Wirety server with OIDC enabled:
```bash
export AUTH_ENABLED=true
export AUTH_ISSUER_URL=http://localhost:5556/dex
export AUTH_CLIENT_ID=wirety
export AUTH_CLIENT_SECRET=wirety-secret
export AUTH_JWKS_CACHE_TTL=3600

# Example server start (adjust path/module):
go run ./server/cmd
```

## Testing Login Flow
1. Navigate your frontend to the login route that triggers OIDC (e.g., clicking "Login").
2. You should be redirected to Dex's login screen.
3. Use credentials: `test@example.com` / `password`.
4. After successful auth, Dex redirects back to Wirety; Wirety validates ID token signature via JWKS.

## Regenerating Password Hash
If you want a different password:
```bash
htpasswd -bnBC 10 "" newpass | tr -d ':\n'
```
Replace the hash in `config.yaml` under `staticPasswords`.

## Customizing Redirect URIs
Ensure every redirect URI is:
- Exact match in `staticClients.redirectURIs`
- Registered in frontend/server OIDC handler

Common patterns:
- `http://localhost:8080/oauth/callback`
- `http://localhost:3000/callback`

## Refresh Tokens
`grantTypes` include `refresh_token`; ensure Wirety stores and exchanges refresh tokens if long-lived sessions are needed.

## Notes
- This setup is NOT production-ready (no external connector, single secret, plaintext config).
- For production: use Postgres storage, rotate client secrets, configure secure HTTPS issuer.

---
Happy testing! Let me know if you’d like a Helm chart variant or Keycloak integration next.
