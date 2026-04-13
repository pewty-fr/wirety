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

External DNS (internet) is unaffected — unauthenticated peers can still browse the web normally.

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
| Authenticated peer loses access after 24 hours | Expected — the whitelist TTL expired. The peer must re-authenticate. |
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
