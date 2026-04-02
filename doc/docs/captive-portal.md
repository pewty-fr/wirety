---
id: captive-portal
title: Captive Portal
sidebar_position: 7
---

The captive portal enforces user authentication before granting network access through a jump peer.
When a new WireGuard peer connects, all their traffic is blocked until they authenticate via the Wirety web interface.

:::info OIDC required
The captive portal is **disabled when `AUTH_ENABLED=false`** (simple auth / shared admin password). Because simple auth has no per-user identity, peer ownership cannot be enforced. Both the token creation endpoint (agent-side) and the authentication endpoint (browser-side) return `403` in this mode.
:::

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
User authenticates with their OIDC account
        │
        ▼
Server checks: authenticated user == peer owner  ──► reject if mismatch
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

## Requirements

- `AUTH_ENABLED=true` (OIDC) — captive portal is disabled in simple-auth mode.
- The peer must have an **owner** (set when a user creates the peer). Admin-created ownerless peers cannot use the captive portal.
- The authenticated user must be the peer's owner. Neither another user nor an administrator can authenticate on behalf of someone else's peer.

## Agent Setup

The captive portal redirect server starts automatically when the agent receives its first policy. No extra configuration is required beyond what is already needed for jump peer operation.

The only optional flag is `-portal-url` (or `CAPTIVE_PORTAL_URL` env), which defaults to `<SERVER_URL>/captive-portal`.

```bash
# Default — portal URL is derived from server URL
wirety-agent -server https://wirety.example.com -token <TOKEN>

# Explicit portal URL (e.g. if the captive portal is on a different domain)
wirety-agent -server https://wirety.example.com -token <TOKEN> \
  -portal-url https://wirety.example.com/captive-portal
```

The agent requires `net.ipv4.conf.all.route_localnet=1` to DNAT port 80 traffic to `127.0.0.1:8081`. This sysctl is set automatically.

## Ownership Enforcement

The server enforces strict ownership during captive portal authentication:

| Peer type | Authenticated as | Result |
|-----------|-----------------|--------|
| Peer with owner | Peer's owner | ✅ Whitelisted |
| Peer with owner | Different user | ❌ `access denied: this peer belongs to another user` |
| Peer with owner | Administrator | ❌ `access denied: this peer belongs to another user` |
| Ownerless peer (admin-created) | Any user | ❌ `access denied: this peer has no owner and cannot be authenticated via captive portal` |
| Any peer | Any user | ❌ `captive portal is not available when AUTH_ENABLED=false` (if OIDC disabled) |

When authentication fails with an ownership error, the captive portal page shows a **"Sign in with a different account"** button that clears the current session and reloads, allowing the correct user to authenticate.

## Token Lifecycle

| Token | TTL | Purpose |
|-------|-----|---------|
| Captive portal token (`cpt_…`) | 10 minutes | URL token embedded in the redirect URL. Kept alive (not deleted on first use) to handle the race condition where the agent hasn't yet synced iptables before the browser follows the post-auth redirect. Expires naturally. |
| Per-peer token cache (agent) | 9 minutes | In-memory cache on the agent to avoid creating a new DB token for every intercepted HTTP request. |

## Session Lifetime

Sessions use httpOnly cookies exclusively — no localStorage. The cookie is automatically sent with every request to the Wirety domain, including the captive portal authenticate endpoint.

| Mode | Session TTL | Notes |
|------|-------------|-------|
| OIDC | 30 days | Backed by OIDC refresh token. Access token is silently refreshed by the server middleware. If the IdP revokes the refresh token, the session is invalidated on the next request. |
| Simple auth (`AUTH_ENABLED=false`) | 30 days | Captive portal is **disabled** in this mode. |

Expired sessions are purged from the database automatically (`refresh_token_expires_at < NOW()`).

## Disconnect & Reconnect Behavior

### User session (browser)
The browser cookie is persistent (30-day TTL). When the user opens the captive portal page again after a reconnect, they are already considered authenticated and the portal flow proceeds automatically without a new login.

### Peer whitelist (iptables)
The captive portal whitelist is persisted in the database with a **24-hour TTL**. When the agent restarts or reconnects:

1. The server pushes a policy update via WebSocket including the current (non-expired) whitelist.
2. The agent re-syncs iptables and re-adds `ACCEPT` rules.

Already-authenticated peers do not need to re-authenticate after an agent restart, as long as their VPN IP has not changed and the 24-hour TTL has not elapsed.

:::caution
If a peer is reassigned a new VPN IP (e.g. after a long absence and IPAM recycles the address), the old whitelist entry no longer matches and the peer must re-authenticate.
:::

## Security

### Stolen WireGuard config
If a user's WireGuard config (private key) is stolen, the attacker connects with the same VPN IP and would normally inherit the whitelist entry. Two defences limit the damage:

**Whitelist TTL (24 hours):** Whitelist entries expire after 24 hours. The attacker's access ends when the entry expires, even if the theft is undetected.

**Automatic revocation on security incident:** Wirety's endpoint-change detection flags suspicious activity (e.g. the peer connecting from a new public IP). When an incident is raised and the peer is quarantined, the captive portal whitelist entry is revoked immediately across all jump peers. The iptables `ACCEPT` rule is removed on the next agent sync.

### Shared WireGuard config (intentional)
If a user shares their WireGuard config with another person, that person will connect with the same VPN IP but will not be able to pass the captive portal: authentication checks that the Wirety session belongs to the peer's owner. Attempting to authenticate as a different user — even an administrator — results in an ownership error.

## Whitelist Management

The whitelist is per-jump-peer and stored in the `captive_portal_whitelist` table.

| Operation | When |
|-----------|------|
| `AddCaptivePortalWhitelist` | Peer completes captive portal authentication (upserts with 24h TTL) |
| `GetCaptivePortalWhitelist` | Agent requests policy sync — filters out expired entries |
| `RemoveCaptivePortalWhitelistByPeerIP` | Security incident detected (quarantine) |
| `ClearCaptivePortalWhitelist` | Jump peer deregistration |
| `CleanupExpiredCaptivePortalWhitelist` | Hourly background job |

## Troubleshooting

| Symptom | Likely cause |
|---------|-------------|
| Captive portal page says "not available" | `AUTH_ENABLED=false` — enable OIDC to use captive portal. |
| "access denied: this peer belongs to another user" | Logged in as the wrong Wirety user. Click "Sign in with a different account" and log in as the peer's owner. |
| "access denied: this peer has no owner" | The peer was created by an admin without assigning an owner. Assign an owner in the Wirety dashboard. |
| Peer bypasses captive portal without authenticating | Policy ACCEPT rules in `WIRETY_JUMP` may fire before the DROP rule. See [known limitation](#known-limitation-policy-accept-bypass). |
| Authenticated peer loses access after 24 hours | Expected — the whitelist TTL expired. The peer must re-authenticate. |
| Authenticated peer loses access after agent restart | Whitelist was not restored — check WebSocket connectivity between agent and server. |

## Known Limitation: Policy ACCEPT Bypass

Policy-based iptables rules are added to the `WIRETY_JUMP` chain **before** the final DROP rule. This means a peer can reach destinations explicitly allowed by policy without going through the captive portal first. Only port 80 traffic is intercepted by DNAT.

A future improvement will introduce a two-chain structure:
- `WIRETY_JUMP`: auth gate — DROP all non-whitelisted peers.
- `WIRETY_POLICY`: per-peer access control — applied only after a peer is whitelisted.
