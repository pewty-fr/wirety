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
             DNS returns the resource's REAL IP (not the portal IP)
                  │
                  ├── HTTP  → forwarded, then DNAT'd on :80 to the portal
                  └── HTTPS → forwarded, blocked with a TCP reset
                              (not intercepted — no cert, no HSTS error)
        │
        ▼
Captive portal HTTP server (listening on <wg-ip>:80)
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

The captive portal HTTP server starts automatically when the agent receives its first policy. No extra configuration is required beyond what is already needed for jump peer operation.

The only optional flag is `-portal-url` (or `CAPTIVE_PORTAL_URL` env), which defaults to `<SERVER_URL>/captive-portal`.

```bash
# Default — portal URL is derived from server URL
wirety-agent -server https://wirety.example.com -token <TOKEN>

# Explicit portal URL (e.g. if the captive portal is on a different domain)
wirety-agent -server https://wirety.example.com -token <TOKEN> \
  -portal-url https://wirety.example.com/captive-portal
```

The agent listens directly on the WireGuard interface IP on port 80 (e.g. `10.255.0.1:80`). No DNAT-to-localhost rule or `route_localnet` sysctl is required. The captive portal does **not** run an HTTPS listener — unauthenticated HTTPS to an internal resource is blocked with a TCP reset rather than intercepted (see [HTTPS handling](#https-handling)).

:::caution Port availability
The agent binds to **port 80** on the WireGuard interface IP. Ensure nothing else is already listening on that address/port combination on the jump peer host.
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

#### Internal VPN domain resolution

DNS queries for **internal VPN domain names** (peer hostnames, route FQDNs) always resolve to the resource's **real IP** — the same answer for authenticated and unauthenticated peers. DNS is **not** the access boundary; the jump peer's iptables is. An unauthenticated peer that learns the real IP still cannot reach the resource:

- **HTTPS** (`:443`) → forwarded → the `WIRETY_JUMP` chain rejects it with a TCP reset. The connection fails fast with no certificate involved — so there is never an HSTS error, even for HSTS-only apps.
- **HTTP** (`:80`) → forwarded → a nat `PREROUTING` DNAT redirects it to the local captive portal, which serves the `302` to the authentication page.

Resolving the real IP (never the portal IP) means the browser never caches the portal IP for an internal hostname, so once the peer authenticates the resource is reachable **immediately** — there is no stale-DNS window to wait out (browsers such as Firefox cache for ~60 s regardless of TTL).

For **full-tunnel peers** the agent is more aggressive: every external A/AAAA query from an unauthenticated full-tunnel peer is redirected to the captive portal IP. This is necessary because full-tunnel peers route every external connection through the jump peer — without this their browser would resolve real IPs and have its connections dropped silently by the FORWARD chain, with no captive-portal redirect ever firing. The agent learns each peer's `AllowedIPs` from the heartbeat (`local_allowed_ips`) so it applies this only to the peers that need it. Split-tunnel peers use external DNS normally — their external traffic doesn't cross the jump peer anyway.

```
Unauthenticated peer resolves server1.wg.example.com
  → DNS returns 10.255.0.2 (real peer IP)
  → HTTP  → DNAT'd to the captive portal → redirect to auth page
  → HTTPS → TCP reset (blocked by iptables, not intercepted)

Authenticated peer resolves server1.wg.example.com
  → DNS returns 10.255.0.2 (real peer IP)
  → Connection goes to the private resource directly
```

:::info DNS requirement
Both probe interception and internal domain interception only work when the WireGuard config sets `DNS = <jump-peer-wg-ip>` so the peer uses the jump peer's DNS server.
:::

### HTTP probe responses

The HTTP server (`:80`) handles intercepted requests with this logic:

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

## HTTPS handling

The captive portal is **HTTP-only** — the agent does not run an HTTPS listener and never intercepts a peer's TLS connection. An unauthenticated peer that attempts HTTPS to an internal resource has its connection **reset** (`WIRETY_JUMP` rejects `:443` with a TCP reset). The browser fails fast, with no certificate exchanged.

This is a deliberate design choice. Injecting a captive portal into an HTTPS session for the app's own hostname would require serving a certificate the client trusts for that hostname — i.e. a TLS man-in-the-middle. An earlier version did this with an in-memory self-signed certificate, but it was unavoidably broken:

- For **HSTS-preloaded** hosts (every major SSO provider, and any app that sends `Strict-Transport-Security`) the browser hard-blocks the mismatched certificate with **no bypass** — an unrecoverable error page.
- For HTTPS-only internal apps it forced a downgrade to `http://` after authentication, which those apps reject.

Removing the interception means **no HSTS dead-ends** and HTTPS-only apps keep working. Portal discovery for unauthenticated peers is carried entirely over HTTP:

- **OS captive-portal detection** — the OS probes (plain HTTP) are DNS-intercepted to the jump peer and answered on `:80`, raising the native "Sign in to network" banner. The `:443` TCP reset additionally nudges iOS/Android to run their detection.
- **The dashboard sign-in popup** — the Wirety web app polls each device's state and, when one needs sign-in, offers an on-demand link to the portal (over HTTP).

Once the peer authenticates, DNS resolves the app to its **real IP** and HTTPS works untouched — the portal is never in the TLS path.

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
| **Default** (no token, not quarantined) | New peer that just connected | Only DNS to the jump peer and the captive-portal HTTP server on the jump peer's WG IP — enough to trigger the redirect (HTTPS is reset until the peer authenticates) |

This replaces the previous design where unauthenticated peers had unrestricted external HTTPS access (intended for the OIDC redirect, but also a usable internet bypass). The grant is now per-peer and time-bounded.

### Endpoint stability window

When a whitelisted peer's WireGuard endpoint changes (different `ip:port` from `wg show endpoints`), the peer is held out of the iptables whitelist for **10 seconds** of stability before being re-admitted. This prevents the oscillation that occurs when two devices share the same WireGuard private key — each keepalive overrides the recorded endpoint, and without the stability window the legitimate peer would gain and lose access every ~25 s.

### Stolen / shared WireGuard config — three layers of defence

Wirety distinguishes carefully between **legitimate single endpoint changes** (NAT rebinding, mobile network handover, fresh tunnel) and **rogue takeovers** (a second device with the same private key competing for the peer slot). Conflating the two would be catastrophic — denylisting a rogue is right; denylisting the legitimate user after a NAT rebind would lock them out for 24 h with no recovery path.

The three layers fire in order, escalating only when there's clear evidence the previous layer is insufficient:

**Layer 1 — Endpoint binding (whitelist requires endpoint match).** Each whitelist entry stores the peer's full public endpoint (`ip:port`) at authentication time. The 300 ms firewall re-sync compares the live endpoint from `wg show endpoints` against the stored one and drops the peer from the iptables whitelist on any mismatch. Cost to a legitimate user who roamed: a brief loss of access until they re-authenticate via the captive portal (which they CAN reach, because the captive portal is on the jump peer's WG IP and only the iptables FORWARD chain is affected). Cost to a rogue: same thing — they need to authenticate, but they fail the SSO ownership check, so they get nothing.

**Layer 2 — Endpoint stability window (10 s).** When any endpoint change is observed, the peer is held out of the iptables whitelist for 10 s of stability before being re-admitted. *Symmetric* — doesn't decide who's "rogue", just refuses to commit during turbulence. Catches both NAT rebinds (they flip once and stabilise) and oscillations (they keep flipping, so the timer never expires and nobody gets access).

**Layer 3 — Physical-interface denylist — only on confirmed oscillation.** This is the heavy hammer: an iptables `-p udp --dport <wg-port> -s <rogue-ip> --sport <rogue-port> -j DROP` rule on the egress interface, BEFORE WireGuard decapsulates. The rogue source can no longer complete WireGuard handshakes at all. **It only fires when the agent observes the endpoint flip stored→foreign at least twice within 60 seconds** — the unambiguous signature of two devices simultaneously sending handshakes. A single endpoint change (NAT rebind, roam, etc.) never trips this layer; layers 1+2 handle it gracefully.

#### Why "oscillation" is the only safe trigger for the denylist

| Pattern observed by `wg show endpoints` | What it really is | What we do |
|---|---|---|
| `A` → `B`, then stable at `B` | NAT rebound, user roamed, fresh tunnel | Drop peer from whitelist (Layer 1+2). User re-auths via captive portal. **No denylist.** |
| `A` → `B` → `A` → `B` (oscillating) | Two devices competing for the slot | Denylist `B` (Layer 3). Legitimate user stops being interrupted. |
| `A` → `B` → `A`, then stable at `A` | Brief blip (single rogue handshake that didn't repeat, or a transient mis-route) | One flip counted, but the second flip never comes. Counter resets after 60 s. **No denylist.** |

The trade-off: the rogue gets a brief window of disruption before they're identified. Layer 2 (the stability window) prevents either side from having stable access during that window, so the rogue cannot exploit it to do useful work — and the moment they earn their second flip, Layer 3 cuts them off completely.

#### After a denylist fires

Once a foreign endpoint is denylisted on the physical interface:
- The rogue's WireGuard handshakes are dropped before reaching the kernel.
- WireGuard's recorded endpoint stops oscillating and settles back on the legitimate user's value.
- The legitimate user's iptables whitelist rule reappears after the 10 s stability window.
- Normal service resumes, with the rogue locked out for 24 h.

When the legitimate peer next authenticates via the captive portal (from any source — including a future genuine roam), the server clears the entire endpoint denylist for that peer's wgIP. SSO ownership is the cryptographic ground truth; if the user proved it, prior "rogue" attribution is overridden.

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
| Port 80 already in use on jump peer | Something else is bound to `<wg-ip>:80`. The agent logs an error and the captive portal will not function. |
| HTTPS to an internal resource fails before authentication | Expected — the captive portal does not intercept HTTPS; unauthenticated `:443` is reset. Trigger the portal over HTTP, or use the dashboard sign-in popup. After authenticating, HTTPS works normally. |

## Reverse Proxy and Virtual Host Isolation

When the Wirety server is deployed behind a reverse proxy that also serves other applications on the same IP and port, unauthenticated peers could reach those other apps before completing captive portal authentication.

For an HTTPS server the agent closes this with an **SNI proxy** on the jump peer:

1. In `nat PREROUTING`, connections from **unauthenticated** peers to the server's IP:port are redirected (`WIRETY_SNI` / `WIRETY6_SNI` chains) to the proxy on `<wg-ip>:3129` (`HTTPS_PROXY_PORT`). Authenticated peers are excluded and keep reaching the server directly.
2. The proxy reads the TLS ClientHello and checks its **SNI** (Server Name Indication, sent in clear) against the allowed host names: `SERVER_HOST`, the hosts of `SERVER_URL` and `CAPTIVE_PORTAL_URL`, and the OIDC issuer host (the IdP often shares the same ingress).
3. An allowed connection is relayed byte for byte to the server. Anything else — another vhost, no SNI, non-TLS — is closed.

The proxy **never decrypts** anything: TLS stays end to end between the peer and the server, and the peer sees the server's own certificate.

```
nat WIRETY_SNI:   -s <authenticatedPeerIP> -j RETURN
                  -d <serverIP> -p tcp --dport 443 -j REDIRECT --to-ports 3129
filter WIRETY_JUMP:
  Rule 0:  -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
  Rule 1:  -d <serverIP> -p tcp --dport 443 -j ACCEPT   (authenticated peers — unauthenticated ones were redirected above)
  Rule 2:  -s <authenticatedPeerIP> -j WIRETY_POLICY
  Rule 3:  -j DROP                                       (everyone else)
```

Before authentication the peer can therefore sign in on `https://wirety.example.com`, while `https://docs.example.com` on the same ingress IP is refused. After authentication the policy applies as usual.

### Limitations

**Plain-HTTP server or no host name:** SNI only exists for TLS. With an `http://` `SERVER_URL`, or when no host name is known (bare-IP `SERVER_URL` without `SERVER_HOST` or a hostname in `CAPTIVE_PORTAL_URL`), the proxy is disabled and every vhost on the server's IP:port is reachable before authentication. The agent logs a warning at startup in the latter case.

**Encrypted Client Hello (ECH):** a client using ECH hides the real SNI; the proxy then sees the public name only and refuses the connection unless that name is allowed. Internal hosts do not publish ECH configurations, so browsers send a plain SNI to them.

## Kernel Module Requirements

The captive portal firewall rules depend on these kernel modules:

| Module | Purpose |
|--------|---------|
| `nf_conntrack` | Conntrack state matching — allows ongoing TCP sessions to pass without re-checking every packet |
| `nft_compat` | xtables compatibility layer for `iptables-nft` (xtables matches through the nf_tables backend). No-op on legacy iptables. |

Virtual-host isolation needs no kernel module: it is done by the agent's SNI proxy in user space (see [Reverse Proxy and Virtual Host Isolation](#reverse-proxy-and-virtual-host-isolation)).

**The agent loads these automatically at startup** via `modprobe`. No manual action is required on most systems — the modules ship with the kernel on all mainstream distros (Debian, Ubuntu, RHEL, Alpine).

If a module fails to load, the agent logs a warning and continues with degraded behaviour:

```
WARN  failed to load kernel module — functionality may be degraded
      module=nf_conntrack purpose="conntrack state matching (ESTABLISHED/RELATED)"
```

To make the modules persist across reboots independently of the agent:

```bash
# Debian / Ubuntu
echo -e "nf_conntrack\nnft_compat" >> /etc/modules

# RHEL / CentOS / Fedora
cat > /etc/modules-load.d/wirety.conf <<EOF
nf_conntrack
nft_compat
EOF
```

On minimal or embedded kernels where the modules are not compiled in, install the extras package:

```bash
# Debian / Ubuntu
apt-get install linux-modules-extra-$(uname -r)
```
