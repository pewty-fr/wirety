---
id: peers
title: Peers
sidebar_position: 3
---

Three logical peer types exist:

## Jump Peer
- Acts as central hub and router.
- Requires agent; enrollment token generated on creation.
- Has listen port + NAT interface.
- Provides routing for encapsulated traffic and additional allowed IP ranges.

## Regular Dynamic Peer (Agent-Based)
- `use_agent = true`.
- Receives token; agent handles config updates, endpoints, heartbeat.
- Suitable for servers or managed hosts.

## Regular Static Peer
- `use_agent = false`.
- Receives a one-time WireGuard config (private key never sent again outside config generation process).
- Ideal for phones, laptops, lightweight devices.

## Common Fields
| Field | Description |
|-------|-------------|
| name | Display name |
| address | Allocated from network CIDR |
| public_key | Peer WireGuard public key |
| endpoint | IP:Port when applicable |
| is_isolated | Isolation flag (no lateral regular peer traffic) |
| full_encapsulation | Route all traffic through jump peer |
| additional_allowed_ips | Extra CIDR ranges accessible via tunnel |

## Tokens & Security
Tokens allow agent enrollment; they should be treated as secrets. Token revocation is accomplished by deleting the peer from the server, which immediately invalidates the token and prevents further agent enrollment or configuration updates.

## ACL Interaction
Blocking a peer (incident) prevents its configuration from being distributed or updated, effectively quarantining it.
