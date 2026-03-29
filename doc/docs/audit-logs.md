---
id: audit-logs
title: Audit Logs
sidebar_position: 11
---

Wirety emits structured audit logs for every security-relevant action across the server and every agent. Audit logs are separate from the application's operational logs: they are always JSON, always on **stdout**, and designed for ingestion by log aggregation systems (Loki, Elasticsearch, Splunk, etc.).

## Enabling Audit Logs

Set the `AUDIT_LOG=true` environment variable on the **server** and/or each **agent** instance.

```bash
# Server
AUDIT_LOG=true ./wirety-server

# Agent
AUDIT_LOG=true wirety-agent -server https://wirety.example.com -token <TOKEN>
```

When `AUDIT_LOG` is `false` (the default) the audit logger is a true no-op — no allocations occur, so there is zero performance impact.

## Log Format

Every audit event is a single JSON line on `stdout`:

```json
{
  "level": "info",
  "log_type": "audit",
  "time": 1748000000,
  "actor_id": "alice",
  "actor_email": "alice@example.com",
  "remote_ip": "10.0.0.5",
  "action": "peer.create",
  "network_id": "net-abc123",
  "peer_id": "peer-xyz789",
  "peer_name": "office-laptop",
  "message": "audit"
}
```

| Field | Always present | Description |
|-------|---------------|-------------|
| `log_type` | ✓ | Always `"audit"` — use this to filter audit lines from application logs |
| `time` | ✓ | Unix timestamp |
| `action` | ✓ | What happened (see tables below) |
| `message` | ✓ | Always `"audit"` |
| `actor_id` | Server only | ID of the authenticated user who triggered the action |
| `actor_email` | Server only | Email of the authenticated user |
| `remote_ip` | Server only | Client IP address |
| `peer_id` / `network_id` | Agent only | Identity of the agent that performed the action |

---

## Server Events

Server audit events are emitted after every successful mutating API call.

### Authentication

| `action` | Trigger |
|----------|---------|
| `auth.login` | Successful simple-auth login (`POST /auth/login`) |

### Networks

| `action` | Fields | Trigger |
|----------|--------|---------|
| `network.create` | `network_id`, `network_name` | Network created |
| `network.update` | `network_id`, `network_name` | Network configuration updated |
| `network.delete` | `network_id` | Network deleted |
| `acl.update` | `network_id` | ACL block list updated |

### Peers

| `action` | Fields | Trigger |
|----------|--------|---------|
| `peer.create` | `network_id`, `peer_id`, `peer_name` | Peer created |
| `peer.update` | `network_id`, `peer_id`, `peer_name` | Peer updated |
| `peer.delete` | `network_id`, `peer_id` | Peer deleted |

### Groups

| `action` | Fields | Trigger |
|----------|--------|---------|
| `group.create` | `network_id`, `group_id`, `group_name` | Group created |
| `group.update` | `network_id`, `group_id`, `group_name` | Group updated |
| `group.delete` | `network_id`, `group_id` | Group deleted |
| `group.peer.add` | `network_id`, `group_id`, `peer_id` | Peer added to group |
| `group.peer.remove` | `network_id`, `group_id`, `peer_id` | Peer removed from group |

### Policies

| `action` | Fields | Trigger |
|----------|--------|---------|
| `policy.create` | `network_id`, `policy_id`, `policy_name` | Policy created |
| `policy.update` | `network_id`, `policy_id`, `policy_name` | Policy updated |
| `policy.delete` | `network_id`, `policy_id` | Policy deleted |
| `policy.rule.add` | `network_id`, `policy_id`, `rule_id` | Rule added to policy |
| `policy.rule.remove` | `network_id`, `policy_id`, `rule_id` | Rule removed from policy |
| `policy.group.attach` | `network_id`, `group_id`, `policy_id` | Policy attached to group |
| `policy.group.detach` | `network_id`, `group_id`, `policy_id` | Policy detached from group |
| `policy.group.reorder` | `network_id`, `group_id` | Policy priority order changed |

### Routes

| `action` | Fields | Trigger |
|----------|--------|---------|
| `route.create` | `network_id`, `route_id`, `route_name` | Route created |
| `route.update` | `network_id`, `route_id`, `route_name` | Route updated |
| `route.delete` | `network_id`, `route_id` | Route deleted |
| `route.group.attach` | `network_id`, `group_id`, `route_id` | Route attached to group |
| `route.group.detach` | `network_id`, `group_id`, `route_id` | Route detached from group |

### Users

| `action` | Fields | Trigger |
|----------|--------|---------|
| `user.update` | `target_user_id` | User role or networks updated |
| `user.delete` | `target_user_id` | User deleted |
| `user.defaults.update` | — | Default network permissions changed |

---

## Agent Events

Agent audit events are emitted by each agent process. The `peer_id` and `network_id` fields identify which agent produced the event.

### Configuration & Firewall

| `action` | Fields | Trigger |
|----------|--------|---------|
| `config.sync` | — | WireGuard configuration successfully written and applied |
| `firewall.sync` | `rule_count` | iptables rules successfully applied |
| `dns.update` | `domain`, `peer_count` | DNS server peer table updated |
| `peer.rename` | `old_name`, `new_name`, `new_interface` | Peer renamed, interface migrated |

### Tunnel Sessions

The agent monitors WireGuard handshake timestamps every 15 seconds. A peer is considered **connected** when its latest handshake is within the last 180 seconds (WireGuard's standard inactivity threshold).

| `action` | Fields | Trigger |
|----------|--------|---------|
| `tunnel.connected` | `peer_name`, `peer_pubkey`, `endpoint`, `handshake_at` | Peer established a WireGuard handshake |
| `tunnel.disconnected` | `peer_name`, `peer_pubkey`, `endpoint`, `last_handshake`, `session_duration` | Peer stopped sending handshakes |

Example tunnel events:

```json
{"level":"info","log_type":"audit","time":1748000100,"actor_type":"agent","peer_id":"peer-jump-1","network_id":"net-abc123","action":"tunnel.connected","peer_name":"office-laptop","peer_pubkey":"abc123...","endpoint":"203.0.113.42:51820","handshake_at":1748000098,"message":"audit"}
{"level":"info","log_type":"audit","time":1748003700,"actor_type":"agent","peer_id":"peer-jump-1","network_id":"net-abc123","action":"tunnel.disconnected","peer_name":"office-laptop","peer_pubkey":"abc123...","endpoint":"203.0.113.42:51820","last_handshake":1748003520,"session_duration":3600000000000,"message":"audit"}
```

Note: tunnel events are also emitted as regular (non-audit) log lines with `"event":"tunnel.connected"` / `"event":"tunnel.disconnected"` for human-readable monitoring.

---

## Operational Log (non-audit)

The server and agent also emit human-readable **operational logs** to `stderr` via zerolog's `ConsoleWriter`. These are not audit logs — do not rely on them for compliance. Use `AUDIT_LOG=true` + stdout for that purpose.

---

## Integration Examples

### Docker / docker-compose

```yaml
services:
  wirety-server:
    image: rg.fr-par.scw.cloud/wirety/server:latest
    environment:
      AUDIT_LOG: "true"
    logging:
      driver: json-file
```

Redirect stdout to your log shipper; stderr carries the operational console log.

### Kubernetes

```yaml
env:
  - name: AUDIT_LOG
    value: "true"
```

With a sidecar log shipper (Fluent Bit, Vector) configured to collect stdout from the wirety pods.

### Filtering with jq

```bash
# All audit events from the last run
docker logs wirety-server 2>/dev/null | jq 'select(.log_type == "audit")'

# All peer creations
docker logs wirety-server 2>/dev/null | jq 'select(.action == "peer.create")'

# All tunnel sessions from a specific agent
cat agent.log | jq 'select(.log_type == "audit" and .peer_id == "peer-jump-1" and (.action | startswith("tunnel.")))'
```

### Loki / Grafana

Label selector example (assuming Docker log driver):

```logql
{container="wirety-server"} | json | log_type="audit"
```

Filter by action:

```logql
{container="wirety-server"} | json | log_type="audit" | action=~"peer\\..*"
```
