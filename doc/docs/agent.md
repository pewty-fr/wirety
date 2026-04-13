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
  -skip-tls-verify
        Disable TLS certificate verification for connections to the server
        (env: SKIP_TLS_VERIFY, default: false)
        Use only when the server uses a self-signed or internally-signed certificate
        that the agent host cannot verify. Never use in production with public certificates.
  -log-level string
        Log verbosity: trace|debug|info|warn|error|fatal
        (env: LOG_LEVEL, default: info)
  -log-format string
        Log output format: text|json
        (env: LOG_FORMAT, default: text)
  -audit-log
        Emit JSON audit events to stdout
        (env: AUDIT_LOG, default: false)
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

# Reverse proxy with self-signed certificate (disable TLS verification)
wirety-agent \
  -server https://10.0.0.1 \
  -server-host wirety.internal \
  -portal-url https://wirety.internal/captive-portal \
  -skip-tls-verify \
  -token <TOKEN>

# Structured JSON logs at debug level (useful with log aggregators such as Loki / Datadog)
wirety-agent -log-format json -log-level debug

# Minimal output in production (warnings and above only)
wirety-agent -log-level warn

# Enable audit log alongside normal operation
wirety-agent -audit-log -log-format json

# Equivalent using environment variables
export LOG_FORMAT=json
export LOG_LEVEL=debug
export AUDIT_LOG=true
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

### Captive portal vhost isolation with `SERVER_HOST`

The captive portal iptables rules derive the virtual hostname for SNI/Host filtering from `SERVER_URL`, not from `SERVER_HOST`. When `SERVER_URL` contains a bare IP (e.g. `http://10.0.0.7`), the agent cannot perform hostname-level filtering and falls back to port-only filtering — other virtual hosts on the same IP:port become reachable before authentication completes.

To get both no-DNS access **and** hostname isolation, use a resolvable hostname in `SERVER_URL` and omit `SERVER_HOST`:

```bash
# Preferred: DNS resolves wirety.internal → 10.0.0.7
# Agent connects to IP, SNI/Host filtering uses "wirety.internal"
wirety-agent -server http://wirety.internal -token <TOKEN>
```

If DNS is genuinely unavailable, `SERVER_HOST` + a bare-IP `SERVER_URL` still works — you simply lose vhost isolation for unauthenticated peers. See [Reverse Proxy and Virtual Host Isolation](captive-portal#reverse-proxy-and-virtual-host-isolation) for the full security implications.

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
| Sufficient permissions | Configure network interface, run iptables |
| Port 80 free on WireGuard interface IP | Captive portal HTTP server binds to `<wg-ip>:80` |
| Port 443 free on WireGuard interface IP | Captive portal HTTPS server binds to `<wg-ip>:443` (self-signed cert) |
| `nf_conntrack` kernel module | Conntrack state matching in captive portal firewall rules |
| `xt_string` kernel module | SNI / Host-header vhost isolation in captive portal firewall rules |

The agent calls `modprobe nf_conntrack` and `modprobe xt_string` automatically at startup. These modules ship with the kernel on all mainstream distributions and require no manual installation. If either module is unavailable, the agent logs a warning and continues with degraded captive portal vhost isolation. See [Kernel Module Requirements](captive-portal#kernel-module-requirements) for persistence and troubleshooting.

## Logging

The agent uses [zerolog](https://github.com/rs/zerolog) for structured logging. Both the level and format are configurable via CLI flag or environment variable — the flag takes precedence.

### `LOG_LEVEL`

Controls which log entries are emitted.

| Value | When to use |
|-------|-------------|
| `trace` | Deep protocol-level debugging (very verbose) |
| `debug` | Development and integration testing |
| `info` | Normal production operation *(default)* |
| `warn` | Only warnings and errors |
| `error` | Only errors and fatal events |
| `fatal` | Silent except on crashes |

### `LOG_FORMAT`

| Value | Output | When to use |
|-------|--------|-------------|
| `text` | Coloured, human-readable console output *(default)* | Local development, direct terminal access |
| `json` | One JSON object per line | Log aggregators (Loki, Datadog, Elastic, etc.) |

**`text` sample:**
```
2:47PM INF websocket connected url=wss://wirety.example.com/api/v1/ws
2:47PM INF DNS server starting addr=10.255.0.1:53
```

**`json` sample:**
```json
{"level":"info","time":1744563600,"message":"websocket connected","url":"wss://wirety.example.com/api/v1/ws"}
{"level":"info","time":1744563600,"message":"DNS server starting","addr":"10.255.0.1:53"}
```

:::tip Production recommendation
Use `LOG_FORMAT=json` and `LOG_LEVEL=info` in production so that log lines are machine-parseable and can be ingested without extra parsing rules.
:::

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
