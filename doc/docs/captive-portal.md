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
        ├─── Full tunnel (AllowedIPs = 0.0.0.0/0)
        │         │
        │         ▼
        │    OS captive portal detection fires automatically
        │    (CNA on macOS/iOS, NCSI on Windows)
        │         │
        │         ▼
        │    HTTP probe → jump peer WG IP:80
        │
        └─── Split tunnel (AllowedIPs = private range only)
                  │
                  ▼
             Peer tries to reach a private resource
             (e.g. server1.wg.example.com)
                  │
                  ▼
             DNS: internal VPN domain → captive portal IP
                  │
                  ▼
             HTTP/HTTPS request → jump peer WG IP:80/443
        │
        ▼
Captive portal HTTP server (listening on <wg-ip>:80)
Captive portal HTTPS server (listening on <wg-ip>:443, self-signed)
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

The captive portal HTTP and HTTPS servers start automatically when the agent receives its first policy. No extra configuration is required beyond what is already needed for jump peer operation.

The only optional flag is `-portal-url` (or `CAPTIVE_PORTAL_URL` env), which defaults to `<SERVER_URL>/captive-portal`.

```bash
# Default — portal URL is derived from server URL
wirety-agent -server https://wirety.example.com -token <TOKEN>

# Explicit portal URL (e.g. if the captive portal is on a different domain)
wirety-agent -server https://wirety.example.com -token <TOKEN> \
  -portal-url https://wirety.example.com/captive-portal
```

The agent listens directly on the WireGuard interface IP on both port 80 and port 443 (e.g. `10.255.0.1:80` and `10.255.0.1:443`). No DNAT rule or `route_localnet` sysctl is required.

:::caution Port availability
The agent binds to **port 80 and port 443** on the WireGuard interface IP. Ensure nothing else is already listening on those address/port combinations on the jump peer host.
:::

## OS Captive Portal Detection

Modern operating systems send HTTP probes to well-known URLs when joining a network to check for captive portals. The agent intercepts these at two levels.

### Full-tunnel peers (`AllowedIPs = 0.0.0.0/0`)

When all traffic is routed through the VPN, the OS captive portal detection fires automatically:

- **macOS / iOS** — Captive Network Assistant (CNA) sends HTTP probes through the tunnel
- **Windows** — Network Connectivity Status Indicator (NCSI) sends HTTP probes through the tunnel
- **Android / Linux** — connectivity checks go through the tunnel

The probes hit the agent's HTTP server on `<wg-ip>:80` and receive a redirect to the authentication page.

### Split-tunnel peers (`AllowedIPs` = private range only)

When only private traffic is routed through the VPN, OS probes go to the physical network (not the tunnel) and cannot be intercepted. Instead, the agent intercepts DNS queries for well-known probe domains and for internal VPN resources.

#### Probe domain DNS interception

The agent's DNS server resolves well-known probe domains to the jump peer's WireGuard IP, so OS-initiated probes travel through the tunnel:

| OS | Probe domain |
|----|-------------|
| Android / Chrome | `connectivitycheck.gstatic.com` |
| Android / Chrome | `clients3.google.com` |
| Apple (iOS / macOS) | `captive.apple.com` |
| Apple (iOS / macOS) | `www.apple.com` |
| Windows | `www.msftconnecttest.com` |
| Firefox | `detectportal.firefox.com` |
| GNOME | `nmcheck.gnome.org` |
| Debian | `network-test.debian.org` |

AAAA queries for all probe domains return NODATA to force IPv4, preventing peers that prefer IPv6 from bypassing interception.

#### Internal VPN domain interception

For unauthenticated peers, all DNS queries for **internal VPN domain names** (peer hostnames, route FQDNs) are resolved to the captive portal IP instead of the real peer IP. This means any attempt to reach a private resource redirects the peer to the authentication page.

For **full-tunnel peers** the agent is more aggressive: every external A/AAAA query from an unauthenticated full-tunnel peer is also redirected to the captive portal IP. This is necessary because full-tunnel peers route every external connection through the jump peer — without DNS interception their browser would resolve real IPs and have its connections dropped silently by the FORWARD chain, with no captive-portal redirect ever firing. The agent learns each peer's `AllowedIPs` from the heartbeat (`local_allowed_ips`) so it can apply this redirection only to the peers that need it. Split-tunnel peers continue to use external DNS normally — their external traffic doesn't cross the jump peer anyway.

```
Unauthenticated peer resolves server1.wg.example.com
  → DNS returns 10.255.0.1 (captive portal IP, TTL 5s)
  → HTTP/HTTPS request hits captive portal server
  → Redirect to authentication page

Authenticated peer resolves server1.wg.example.com
  → DNS returns 10.255.0.2 (real peer IP, TTL 60s)
  → Connection goes to the private resource directly
```

:::info DNS requirement
Both probe interception and internal domain interception only work when the WireGuard config sets `DNS = <jump-peer-wg-ip>` so the peer uses the jump peer's DNS server.
:::

### HTTP and HTTPS probe responses

Both the HTTP server (`:80`) and the HTTPS server (`:443`) handle intercepted requests with the same logic:

| Peer state | Behaviour |
|-----------|-----------|
| **Unauthenticated** | Returns `302` redirect to the captive portal authentication page |
| **Authenticated** | Returns the OS-specific success response — OS dismisses the captive portal notification |

OS-specific success responses (served to authenticated peers):

| OS | Path | Response |
|----|------|---------|
| Google / Android | `/generate_204` | `204 No Content` |
| Apple | `/hotspot-detect.html` | `200` + `<HTML>...Success...</HTML>` |
| Windows | `/connecttest.txt` | `200` + `Microsoft Connect Test` |
| Firefox | `/success.txt` | `200` + `success\n` |
| GNOME / Debian | any | `204 No Content` |

## HTTPS Captive Portal Server

The agent runs a self-signed HTTPS server on `<wg-ip>:443` alongside the HTTP server. This handles unauthenticated peers attempting HTTPS access to internal VPN resources.

### Self-signed certificate

The certificate is generated in memory at agent startup (never written to disk) and covers:

- **IP SAN** — the WireGuard interface IP
- **Wildcard DNS SAN** — `*.<vpnDomain>` (e.g. `*.wg.example.com`) so internal peer hostnames match the cert

The VPN domain for the wildcard is taken from the DNS configuration pushed by the server.

### Browser behaviour

Because the certificate is self-signed (not issued by a trusted CA), browsers show a security warning. The behaviour differs depending on the domain:

| Domain type | Browser behaviour |
|-------------|------------------|
| **Internal VPN domain** (e.g. `server1.wg.example.com`) | Warning page with "Proceed anyway" option — user can bypass and be redirected to the captive portal |
| **Public domain in HSTS preload list** (e.g. `google.com`) | Hard-blocked — no bypass available. Peers using HTTPS-only browsers will need to try an HTTP URL or use the direct captive portal URL |

:::info HTTPS limitation
Intercepting HTTPS for public HSTS-preloaded domains is not feasible: browsers hard-block such connections regardless of certificate content. The HTTPS server is primarily useful for internal VPN domains. For peers using full-tunnel mode, the OS captive portal detection (which uses plain HTTP probes) handles the redirect without any certificate interaction.
:::

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

## Security: Three-Tier Authentication Gate

The `WIRETY_JUMP` chain on the jump peer enforces a strict three-tier model for every peer:

| Tier | Who | What they can reach |
|------|-----|---------------------|
| **Authenticated** | Peers in the captive-portal whitelist whose live WireGuard endpoint matches the IP:port recorded at authentication time | Full network access (subject to the policy chain) |
| **Pending Auth** | Peers that have been issued a captive-portal token in the last 10 minutes but have not yet completed SSO | External HTTPS only — enough for the OIDC redirect chain (Slack/GitHub/Google), nothing else |
| **Quarantined** | Peers that abandoned 3 consecutive token issuances without completing SSO | Nothing. Even the captive portal redirect is suppressed until quarantine expires (1 h) or an admin clears it |
| **Default** (no token, not quarantined) | New peer that just connected | Only DNS to the jump peer and the captive-portal HTTP/HTTPS server on the jump peer's WG IP — enough to trigger the redirect |

This replaces the previous design where unauthenticated peers had unrestricted external HTTPS access (intended for the OIDC redirect, but also a usable internet bypass). The grant is now per-peer and time-bounded.

### Endpoint stability window

When a whitelisted peer's WireGuard endpoint changes (different `ip:port` from `wg show endpoints`), the peer is held out of the iptables whitelist for **10 seconds** of stability before being re-admitted. This prevents the oscillation that occurs when two devices share the same WireGuard private key — each keepalive overrides the recorded endpoint, and without the stability window the legitimate peer would gain and lose access every ~25 s.

### Stolen / shared WireGuard config

If a user's WireGuard private key is stolen or shared, the attacker connects from a different public source — different `ip:port` on `wg show endpoints`. Wirety has multiple layers of defence:

**Layer 1 — Endpoint binding.** Each whitelist entry stores the peer's full public endpoint (`ip:port`) at authentication time. Any mismatch on the 300 ms firewall re-sync drops the peer from the iptables whitelist.

**Layer 2 — Endpoint stability.** As described above, 10 seconds of stable endpoint is required before re-admittance, preventing oscillating-endpoint exploitation.

**Layer 3 — Physical-interface denylist.** When the agent observes an authenticated peer's endpoint flip to a foreign source, it reports a "takeover" to the server. The server persists the rogue source in `captive_portal_endpoint_denylist` (24-hour TTL) and pushes it back to the jump peer's WebSocket. The agent then writes an `iptables -p udp --dport <wg-port> -s <rogue-ip> --sport <rogue-port> -j DROP` rule on the **physical** (egress) interface, **before** the WireGuard kernel module sees the packet. The rogue source can no longer complete WireGuard handshakes — the legitimate peer's stored endpoint is never overwritten again.

The combination is the key: stability stops the *symptoms*, the denylist stops the *cause*. A user whose key is shared can keep working; the second device disappears completely. The only escape for the rogue is to come from a different public IP — at which point the denylist entry doesn't match and the rogue is back to square one (must complete SSO, which they cannot if they aren't the peer's owner).

### Quarantine after repeated abandonments

The strike counter `captive_portal_quarantine.strikes` increments by 1 every time a captive-portal token expires without a successful SSO conversion. After **3 strikes** the peer enters quarantine for 1 hour. While quarantined:

- No new tokens are issued (the agent's `/api/v1/captive-portal/token` request is rejected — although the `cleanup` loop is the actual strike trigger)
- No "pending auth" HTTPS grant is given
- The peer is in tier 0 (explicit DROP) — even the captive portal redirect doesn't fire

A successful SSO authentication clears all strikes. An admin can clear the quarantine state manually from the database (`DELETE FROM captive_portal_quarantine WHERE peer_id = '…'`).

### Shared config — intentional sharing
If a user *intentionally* shares their WireGuard config with someone else, the shared device cannot complete SSO unless that person uses the original owner's credentials — captive-portal auth checks that the Wirety session's user ID matches the peer's owner. Attempting to authenticate as a different user (even an admin) returns an ownership error.

## Forcing Re-authentication: Revoke Connection

Administrators and peer owners can force a peer to re-authenticate from the dashboard. In the **Peer Detail** modal, a **"Revoke Auth"** button removes the peer from the captive-portal whitelist across all jump peers in the network. The next request from the peer is redirected to the captive portal and SSO is required to regain access.

Use this when:
- You suspect a peer's WireGuard config has leaked.
- You are rotating credentials.
- You want to force a stale session to refresh (e.g. after group/policy changes).

The peer record itself is untouched — only the authenticated session state is cleared. The peer can re-authenticate immediately by hitting the captive portal.

The corresponding API endpoint is `POST /networks/{networkId}/peers/{peerId}/revoke-auth` — see [API Reference](api-reference).

## Database Tables

| Table | Purpose | TTL |
|-------|---------|-----|
| `captive_portal_whitelist` | Authenticated peers (full access tier). Each row binds a peer's WireGuard IP to the public endpoint observed at SSO time. | 24 h |
| `captive_portal_tokens` | In-flight auth tokens (pending tier).  `consumed_at IS NULL` after expiry counts as 1 strike. | 10 min |
| `captive_portal_endpoint_denylist` | Rogue WireGuard sources to drop at the jump peer's physical interface. Populated from agent-reported takeovers. Cleared automatically when the targeted peer next re-authenticates from any source. | 24 h |
| `captive_portal_quarantine` | Per-peer auth-failure strike count and quarantine end time. | 1 h after 3rd strike; cleared on successful auth |
| `peer_local_routes` | Each peer's locally-configured `AllowedIPs`, reported via heartbeat. Used by the jump peer's DNS to decide route-aware redirection for unauthenticated peers. | Latest heartbeat wins |

Background cleanup tasks (server):

| Operation | Cadence |
|-----------|---------|
| `CleanupExpiredCaptivePortalWhitelist` | Hourly |
| `CleanupExpiredCaptivePortalTokens` (also records strikes for unconsumed tokens) | Every 2 minutes |
| `CleanupExpiredEndpointDenylist` | Every 2 minutes |
| `CleanupExpiredSessions` | Hourly |

## Troubleshooting

| Symptom | Likely cause |
|---------|-------------|
| Captive portal page says "not available" | `AUTH_ENABLED=false` — enable OIDC to use captive portal. |
| "access denied: this peer belongs to another user" | Logged in as the wrong Wirety user. Click "Sign in with a different account" and log in as the peer's owner. |
| "access denied: this peer has no owner" | The peer was created by an admin without assigning an owner. Assign an owner in the Wirety dashboard. |
| Peer can't reach the captive portal at all (browser shows "site unreachable") | Either the peer is **quarantined** (3 abandoned auth attempts in the last hour) or the peer's traffic isn't going through the jump peer at all. Check `captive_portal_quarantine` for the peer ID; clear the row to release. |
| Authenticated peer loses access after 24 hours | Expected — the whitelist TTL expired. The peer must re-authenticate. |
| Authenticated peer loses access after a sudden endpoint change | Expected — the WireGuard endpoint stability window holds peers out of the iptables whitelist for 10 s after any endpoint change to prevent oscillation between two devices using the same key. Wait 10 s; the legitimate peer regains access automatically. |
| Authenticated peer loses access permanently after the legitimate user moves networks | The new public source might have been denylisted as a "rogue takeover". Use the dashboard's **Revoke Auth** button to clear the whitelist entry; the next captive-portal auth from the new endpoint will succeed and clear the denylist as a side effect. |
| Authenticated peer loses access after agent restart | Whitelist was not restored — check WebSocket connectivity between agent and server. |
| OS captive portal popup does not appear (split-tunnel) | Peer's WireGuard config may not set `DNS = <jump-peer-wg-ip>`. Without this, probe domains and internal domain queries bypass the tunnel DNS. Check the peer's WireGuard config. |
| OS captive portal popup does not appear (full-tunnel) | CNA/NCSI fires automatically for full-tunnel peers. If it does not trigger, try disconnecting and reconnecting to WireGuard. |
| OS captive portal popup persists after authentication | DNS TTL (5–10s) may not have expired yet. Wait a few seconds; the next probe will receive a success response. |
| Internal domain resolves to captive portal IP after authentication | Stale DNS cache on the peer. The short TTL (5s) should expire quickly. Flush the DNS cache manually if needed (`sudo dscacheutil -flushcache` on macOS). |
| Port 80 or 443 already in use on jump peer | Something else is bound to `<wg-ip>:80` or `<wg-ip>:443`. The agent logs an error and the captive portal will not function. |
| Browser hard-blocks HTTPS redirect for external domain | Expected — public HSTS-preloaded domains cannot be intercepted. Use the direct captive portal URL, or try an HTTP URL or an internal VPN domain URL to trigger the redirect. |

## Reverse Proxy and Virtual Host Isolation

When the Wirety server is deployed behind a reverse proxy that also serves other applications on the same IP and port, unauthenticated peers could reach those other apps before completing captive portal authentication.

The agent mitigates this with three layers of filtering applied in `WIRETY_JUMP`:

| Layer | Rule | Protects against |
|-------|------|-----------------|
| **IP** | Destination must match the resolved server IP | Unrelated servers |
| **Port** | `--dport` derived from the server URL scheme (`443` for https, `80` for http, or explicit) | Other ports on the same server |
| **Hostname** | L7 string match on the virtual hostname | Other vhosts behind the same reverse proxy |

### Hostname filtering

For HTTPS, the TLS **SNI** (Server Name Indication) is sent cleartext inside the TLS ClientHello. The agent uses an iptables `string` match to verify the SNI matches the Wirety hostname before allowing the connection.

For HTTP, the `Host:` request header is matched in the same way.

Because string matching only works on the first packet of a TCP session, a conntrack `ESTABLISHED,RELATED` rule at the top of `WIRETY_JUMP` allows subsequent packets of already-accepted sessions through without re-checking the hostname.

```
Rule 0:  -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
Rule 1a: -d <serverIP> -p tcp --dport 443 -m string --algo bm --string "wirety.example.com" -j ACCEPT  (HTTPS)
Rule 1b: -d <serverIP> -p tcp --dport 80  -m string --algo bm --string "Host: wirety.example.com" -j ACCEPT  (HTTP)
Rule 2:  -s <whitelistedPeerIP> -j WIRETY_POLICY  (authenticated peers)
Rule 3:  -j DROP                                   (everyone else)
```

### Limitations

**String-match evasion:** The `xt_string` kernel module scans the raw packet payload. A peer who crafts a packet containing the Wirety hostname as arbitrary data while using a different TLS SNI could pass the filter. The reverse proxy's own routing would still use the correct SNI, but the filtered IP-layer connection would be accepted. This is a soft boundary, not a cryptographic one.

**Bare-IP server URL:** If `SERVER_URL` is set to a bare IP address (e.g. `http://10.0.0.7`) instead of a hostname, no SNI/Host matching is possible — the agent falls back to port-only filtering, and all vhosts on that IP:port are reachable before authentication. Use a hostname in `SERVER_URL` whenever possible. See [`SERVER_HOST`](agent#reverse-proxy--no-dns-access-server_host) for connecting by IP while still enabling hostname filtering.

**Module unavailability:** If the `xt_string` kernel module is not loaded, the agent logs a warning and falls back to port-only filtering automatically.

## Kernel Module Requirements

The captive portal firewall rules depend on two kernel modules:

| Module | Purpose |
|--------|---------|
| `nf_conntrack` | Conntrack state matching — allows ongoing TCP sessions to pass without re-checking every packet |
| `nft_compat` | xtables compatibility layer for `iptables-nft` — allows `xt_string` to be used through the nf_tables backend. No-op on legacy iptables. |
| `xt_string` | Payload string matching — SNI / Host-header vhost isolation. Works on both legacy iptables and `iptables-nft` (via `nft_compat`). |

**The agent loads these automatically at startup** via `modprobe`. No manual action is required on most systems — the modules ship with the kernel on all mainstream distros (Debian, Ubuntu, RHEL, Alpine).

:::info iptables-nft
On modern Debian/Ubuntu systems `iptables` is `iptables-nft` by default. The `nft_compat` module bridges the xtables extension interface into nftables, making `xt_string` available on both backends. If `nft_compat` or `xt_string` cannot be loaded, the agent falls back to port-only filtering automatically.
:::

If a module fails to load, the agent logs a warning and continues with degraded behaviour:

```
WARN  failed to load kernel module — functionality may be degraded
      module=xt_string purpose="payload string matching (SNI / Host-header vhost isolation)"
```

To make the modules persist across reboots independently of the agent:

```bash
# Debian / Ubuntu
echo -e "nf_conntrack\nxt_string" >> /etc/modules

# RHEL / CentOS / Fedora
cat > /etc/modules-load.d/wirety.conf <<EOF
nf_conntrack
xt_string
EOF
```

On minimal or embedded kernels where the modules are not compiled in, install the extras package:

```bash
# Debian / Ubuntu
apt-get install linux-modules-extra-$(uname -r)
```
