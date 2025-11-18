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
  -interface string
        WireGuard interface name
        (env: WG_INTERFACE, default: wg0)
  -config string
        Path to wireguard config file
        (env: WG_CONFIG_PATH)
  -apply string
        Apply method: wg-quick|syncconf
        (env: WG_APPLY_METHOD, default: wg-quick)
  -nat string
        NAT interface (eth0, etc.)
        (env: NAT_INTERFACE, default: eth0)
```

## Usage Example
```bash
# Run agent with environment variables
export TOKEN=<ENROLLMENT_TOKEN>
export SERVER_URL=https://wirety.example.com
wirety-agent

# Or use command-line flags
wirety-agent -server https://wirety.example.com -token <ENROLLMENT_TOKEN> -interface wg0 -nat eth0
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
