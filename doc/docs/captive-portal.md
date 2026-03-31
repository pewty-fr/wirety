---
id: captive-portal
title: Captive Portal
sidebar_position: 7
---

The captive portal enforces user authentication before granting network access through a jump peer.
When a new WireGuard peer connects, all their traffic is blocked until they authenticate via the Wirety web interface (or SSO provider).

## How It Works

```
Peer connects to WireGuard tunnel
        │
        ▼
iptables FORWARD DROP (WIRETY_JUMP chain)
        │
        ▼ (port 80 only)
DNAT → local redirect server (port 8081)
        │
        ▼
302 redirect → https://<server>/captive-portal?token=cpt_...&redirect=<original-url>
        │
        ▼
User authenticates (OIDC or simple auth)
        │
        ▼
Server whitelists peer IP (DB + WebSocket push to agent)
        │
        ▼
Agent re-syncs iptables: ACCEPT rule added for peer IP
        │
        ▼
Peer has full network access
```

## Agent Setup

The captive portal redirect server starts automatically when the agent receives its first policy. No extra configuration is required beyond what is already needed for jump peer operation.

The only required flag is `--portal-url` (or `CAPTIVE_PORTAL_URL` env), which defaults to `<SERVER_URL>/captive-portal`.

```bash
# Default — portal URL is derived from server URL
wirety-agent -server https://wirety.example.com -token <TOKEN>

# Explicit portal URL (e.g. if the captive portal is on a different domain)
wirety-agent -server https://wirety.example.com -token <TOKEN> \
  -portal-url https://wirety.example.com/captive-portal
```

The agent requires `net.ipv4.conf.all.route_localnet=1` to DNAT port 80 traffic to `127.0.0.1:8081`. This sysctl is set automatically.

## Token Lifecycle

| Token | TTL | Purpose |
|-------|-----|---------|
| Captive portal token (`cpt_…`) | 10 minutes | One-time URL token embedded in the redirect URL. Consumed on successful authentication and deleted. |
| Per-peer token cache (agent) | 9 minutes | In-memory cache on the agent to avoid creating a new DB token for every intercepted HTTP request (browser keepalives, retries, etc.). |

## Session Lifetime

| Mode | Session TTL | Notes |
|------|-------------|-------|
| OIDC | 30 days | Backed by OIDC refresh token. Access token is silently refreshed by the server middleware when it expires — no user action required. If the OIDC refresh token is revoked by the IdP, the session is invalidated on the next request. |
| Simple auth (`AUTH_ENABLED=false`) | 30 days | No external IdP involved; the access token effectively never expires server-side (100-year TTL). Session expires after 30 days of inactivity or explicit logout. |

The session is stored both as an **httpOnly cookie** (`wirety_session`, 30-day max-age) and as a `session_hash` in `localStorage` (used by the captive portal page to authenticate the peer). Both are written on login and cleared on logout.

Expired sessions are purged from the database automatically (`refresh_token_expires_at < NOW()`).

## Disconnect & Reconnect Behavior

### User session (browser)
The browser cookie and `session_hash` in localStorage survive disconnections — they are persistent (30-day TTL). When the user opens the captive portal page again, they are already considered authenticated by the Wirety UI and the portal flow proceeds automatically, without requiring a new login, as long as the session has not expired.

### Peer whitelist (iptables)
The captive portal whitelist is **persisted in the database**. When the agent restarts or reconnects after a network disruption:

1. The server pushes a policy update via WebSocket that includes the current whitelist.
2. The agent re-syncs iptables and re-adds `ACCEPT` rules for all whitelisted IPs.

As a result, **already-authenticated peers do not need to go through the captive portal again** after an agent restart, as long as their VPN IP has not changed.

:::caution
If a peer is reassigned a new VPN IP (e.g. after a long absence and IPAM recycles the address), the old whitelist entry no longer matches and the peer must re-authenticate through the captive portal.
:::

## Whitelist Management

The whitelist is per-jump-peer and stored in the `captive_portal_whitelist` table. Standard repository operations are available:

| Operation | When |
|-----------|------|
| `AddCaptivePortalWhitelist` | Peer completes captive portal authentication |
| `GetCaptivePortalWhitelist` | Agent requests policy sync (WebSocket push) |
| `RemoveCaptivePortalWhitelist` | Manual admin action (future UI) |
| `ClearCaptivePortalWhitelist` | Jump peer deregistration |

## Troubleshooting

| Symptom | Likely cause |
|---------|-------------|
| "No session found" on captive portal page | User has never logged in on this browser, or logged out. Open the Wirety dashboard and log in first, then retry accessing the network. |
| Captive portal token keeps being created every few seconds | Agent token cache is not working (check agent version). Should not happen in normal operation — the cache reuses the same token for 9 minutes per peer IP. |
| Peer bypasses captive portal without authenticating | Policy ACCEPT rules in `WIRETY_JUMP` may fire before the DROP rule for allowed destinations. See [known limitation](#known-limitation-policy-accept-bypass). |
| Authenticated peer loses access after agent restart | Whitelist was not restored — check WebSocket connectivity between agent and server. |

## Known Limitation: Policy ACCEPT Bypass

Policy-based iptables rules (generated from Groups & Policies) are added to the `WIRETY_JUMP` chain **before** the final DROP rule. This means a peer can reach destinations explicitly allowed by policy (e.g., `172.16.16.0/22`) without going through the captive portal first.

Only port 80 traffic is intercepted by DNAT; other protocols and ports are silently forwarded or dropped based on policy.

A future improvement will introduce a two-chain structure:
- `WIRETY_JUMP`: auth gate — DROP all non-whitelisted peers.
- `WIRETY_POLICY`: per-peer access control — applied only after a peer is whitelisted.
