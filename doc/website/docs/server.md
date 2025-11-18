---
id: server
title: Server
sidebar_position: 7
---

Wirety Server provides REST + WebSocket APIs, orchestrates peers, incidents, ACL, and IPAM.

## Environment Variables
| Variable | Description | Default |
|----------|-------------|---------|
| HTTP_PORT | Server HTTP port | 8080 |
| AUTH_ENABLED | Enable OIDC authentication | false |
| AUTH_ISSUER_URL | OIDC provider URL (e.g., https://keycloak.example.com/realms/wirety) | - |
| AUTH_CLIENT_ID | OIDC client ID | - |
| AUTH_CLIENT_SECRET | OIDC client secret | - |
| AUTH_JWKS_CACHE_TTL | JWKS cache duration in seconds | 3600 |

## Stored Data
- Peers (public key, endpoint, flags, token, additional allowed IPs).
- Networks (CIDR, domain, peer list).
- ACL BlockedPeers map.
- Incidents states + audit (resolvedBy).
- IPAM allocations.

## Swagger / OpenAPI
Swagger documentation available at `/swagger/docs/index.html` when running the server. The API is documented with:
- Title: Wirety Server API
- Version: 1.0
- BasePath: /api/v1
- Security: Bearer token authentication (JWT)

## Notifications
WebSocket channel emits network peer update events enabling agents to refresh configs.
