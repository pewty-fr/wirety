---
id: agent
title: Agent
sidebar_position: 6
---

The Wirety Agent automates configuration retrieval and health reporting.

## Responsibilities
- Enrollment via token.
- Maintain WireGuard config up-to-date (poll/WebSocket-triggered).
- Heartbeat: hostname, uptime, endpoint observation, timestamps.

## CLI Options
```bash
wirety-agent [options]

Options:
  -server string
        Server base URL (no trailing /)
        (env: SERVER_URL, default: http://localhost:8080)
  -token string
        Enrollment token (required)
        (env: TOKEN)
  -config string
        Path to wireguard config file
        (env: WG_CONFIG_PATH)
  -apply string
        Apply method: wg-quick|syncconf
        (env: WG_APPLY_METHOD, default: syncconf)
  -nat-interfaces string
        Comma-separated list of NAT interfaces (env: NAT_INTERFACES)
        Default: auto-detect all egress interfaces from the routing table
  -portal-url string
        Captive portal page URL
        (env: CAPTIVE_PORTAL_URL, default: <SERVER_URL>/captive-portal)
  -server-host string
        Override the HTTP Host header for all requests to the server
        (env: SERVER_HOST, default: derived from -server URL)
        Useful when accessing the Wirety server by IP behind a reverse proxy
        that routes by hostname (e.g. SERVER_URL=http://10.0.0.7 SERVER_HOST=wirety.internal)
```

## Usage Example
```bash
# Run agent — NAT interfaces are auto-detected from the routing table
export TOKEN=<ENROLLMENT_TOKEN>
export SERVER_URL=https://wirety.example.com
wirety-agent

# Explicit NAT interfaces (jump peer with internet + private VLAN egress)
wirety-agent -server https://wirety.example.com -token <TOKEN> -nat-interfaces ens2,ens6

# Environment variable equivalent
export NAT_INTERFACES=ens2,ens6
wirety-agent

# Access server by IP (no DNS) with Host header for reverse proxy routing
wirety-agent -server http://10.0.0.7 -server-host wirety.internal -token <TOKEN>

# Environment variable equivalent
export SERVER_URL=http://10.0.0.7
export SERVER_HOST=wirety.internal
wirety-agent
```

## Reverse Proxy / No-DNS Access (`SERVER_HOST`)

When the Wirety server is deployed inside a private network behind a reverse proxy and no internal DNS is available, the agent can reach the server by IP while still sending the correct `Host` header for the proxy to route the request.

```bash
# Without DNS: connect to 10.0.0.7:80, but send "Host: wirety.internal"
# so Nginx / Caddy / Traefik routes the request to the Wirety backend.
wirety-agent \
  -server http://10.0.0.7 \
  -server-host wirety.internal \
  -token <TOKEN>
```

This is equivalent to:
```bash
curl http://10.0.0.7/api/v1/agent/resolve \
  -H "Host: wirety.internal" \
  -H "Authorization: Bearer <TOKEN>"
```

The `SERVER_HOST` override is applied to **all** outbound connections from the agent:
- Initial token resolution (`/api/v1/agent/resolve`)
- WebSocket connection (`/api/v1/ws`)
- Captive portal token creation (`/api/v1/captive-portal/token`)

## NAT Interface Detection

On jump peers, the agent adds a `MASQUERADE` rule for every egress interface so that forwarded traffic is correctly NATed regardless of which interface the routing table selects for a given destination.

By default the agent **auto-detects all egress interfaces** by scanning the routing table (`ip route show`) and keeping every interface that:
- Has an IPv4 address
- Is not loopback (`lo`)
- Is not the WireGuard interface itself

This means a jump peer with both an internet uplink (`ens2`) and a private VLAN interface (`ens6`) will automatically get `MASQUERADE` on both, allowing peers to reach resources behind either interface.

Use `NAT_INTERFACES` to override when auto-detection picks up unwanted interfaces:

```bash
# Only NAT through the private VLAN interface
export NAT_INTERFACES=ens6
```

## Host Prerequisites
| Requirement | Reason |
|-------------|--------|
| WireGuard kernel/module | Interface creation |
| curl / TLS libs | Enrollment requests |
| Sufficient permissions | Configure network interface |

## Security
- Token used only at enrollment; store ephemeral auth afterward.
- Private keys generated server-side; agent receives only public + config data.
- ACL blocking prevents agent from receiving updates (quarantine).

## Heartbeat Data
| Field | Description |
|-------|-------------|
| hostname | Reported system hostname |
| uptime | Seconds since boot |
| endpoint | Detected public endpoint |
| last_seen | Server timestamp |

## Future
- Automatic key rotation.
- Metrics emission (Prometheus).
