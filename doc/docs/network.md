---
id: network
title: Network
sidebar_position: 2
---

A Wirety Network encapsulates a WireGuard mesh identified by a CIDR, domain, and its collection of peers.

## Components
- CIDR: Address space allocated via IPAM. Each peer gets a unique address.
- Gateway / Jump Peers: Provide central routing + optional NAT for full encapsulation.
- ACL: Holds blocked peer IDs due to incidents; affects generated configs.

## CIDR Management
Changing a network CIDR is disallowed if any static regular peer exists. Rationale: static peers require manual reconfiguration and could lose connectivity silently.

Process when CIDR changes (only when allowed):
1. Release old IPs from IPAM.
2. Allocate new IP per peer.
3. Update peer records; notify via WebSocket.

## Full Encapsulation
When a regular peer sets `full_encapsulation = true`, `0.0.0.0/0` is routed through jump peers (plus additional allowed IPs). For dynamic peers, agent refreshes config automatically.

## Isolation
`is_isolated = true` prevents regular peer to regular peer connectivity (except via jump). Jump peers remain reachable for routing.

## Additional Allowed IPs
Configured as CIDR list on peer. Validated format (e.g. `10.10.0.0/16`). Added to AllowedIPs for that peer in WireGuard config generation.

## Notifications
WebSocket notifier pushes update events so agents can refetch config after ACL changes, peer additions, or incident resolutions.
