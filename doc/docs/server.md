---
id: server
title: Server
sidebar_position: 7
---

Wirety Server provides REST + WebSocket APIs, orchestrates peers, incidents, ACL, and IPAM.

## Environment Variables

### Core
| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_PORT` | Server HTTP port | `8080` |
| `CORS_ORIGIN` | Allowed CORS origin(s) — comma-separated for multiple origins (e.g. `https://app.example.com,https://admin.example.com`). `ALLOWED_ORIGIN` is a legacy alias. | `*` |
| `AUDIT_LOG` | Enable structured JSON audit logging to stdout | `false` |

### Authentication
| Variable | Description | Default |
|----------|-------------|---------|
| `AUTH_ENABLED` | Enable OIDC authentication (false = simple auth) | `false` |
| `AUTH_ISSUER_URL` | OIDC provider URL (e.g., `https://keycloak.example.com/realms/wirety`) | - |
| `AUTH_CLIENT_ID` | OIDC client ID | - |
| `AUTH_CLIENT_SECRET` | OIDC client secret | - |
| `AUTH_JWKS_CACHE_TTL` | JWKS cache duration in seconds | `3600` |
| `AUTH_PASSWORD` | Admin password for simple auth mode | auto-generated (logged at startup) |

### Database
| Variable | Description | Default |
|----------|-------------|---------|
| `DB_ENABLED` | Enable PostgreSQL persistence (false = in-memory) | `false` |
| `DB_DSN` | PostgreSQL connection string | `postgres://wirety:wirety@localhost:5432/wirety?sslmode=disable` |
| `DB_MIGRATIONS_DIR` | Path to SQL migration files | `cmd/kodata` |

## Authentication Modes

### Simple Auth (default, `AUTH_ENABLED=false`)
On first start, the server generates a random admin password and logs it:
```
WRN Simple auth enabled - generated admin password password=abc123 username=admin
```
Set a fixed password via `AUTH_PASSWORD` to avoid regeneration on restart.

Login via `POST /api/v1/auth/simple-login` with `{"username":"admin","password":"..."}`.

### OIDC (`AUTH_ENABLED=true`)
See the [OIDC guide](./guides/oidc) for full configuration. Users authenticate via your identity provider; their roles and network access are managed in the Wirety UI.

## API Tokens
Users can create long-lived API tokens (same permissions as their account) for scripting and integrations:

```bash
# Create a token
curl -X POST http://localhost:8080/api/v1/users/me/tokens \
  -H "Authorization: Bearer <session-token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-ci-token"}'

# Use a token
curl http://localhost:8080/api/v1/networks \
  -H "Authorization: Bearer wirety_<64-hex-chars>"
```

Tokens use the `wirety_` prefix and are accepted in both simple auth and OIDC modes. The raw token is shown only once at creation; only its SHA-256 hash is stored.

## MCP Server
An embedded [Model Context Protocol](https://modelcontextprotocol.io) server is available at `GET/POST /mcp` using the Streamable HTTP transport. It exposes Wirety capabilities as AI-callable tools (list/create/delete networks, peers, groups, policies, routes, incidents, and API tokens).

Authentication uses the same API tokens as the REST API:
```json
{
  "mcpServers": {
    "wirety": {
      "type": "http",
      "url": "http://localhost:8080/mcp",
      "headers": { "Authorization": "Bearer wirety_<token>" }
    }
  }
}
```

See the [MCP guide](./mcp) for Claude Desktop / Claude Code setup.

## Stored Data
- Peers (public key, endpoint, flags, token, additional allowed IPs).
- Networks (CIDR, domain, peer list).
- ACL BlockedPeers map.
- Incidents states + audit (resolvedBy).
- IPAM allocations.
- API tokens (hashed).

## Swagger / OpenAPI
Swagger documentation available at `/swagger/docs/index.html` when running the server. The API is documented with:
- Title: Wirety Server API
- Version: 1.0
- BasePath: /api/v1
- Security: Bearer token authentication (JWT or `wirety_` API token)

## Notifications
WebSocket channel emits network peer update events enabling agents to refresh configs.
