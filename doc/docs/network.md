---
id: network
title: Network
sidebar_position: 2
---

A Wirety Network encapsulates a WireGuard mesh identified by a CIDR, domain, and its collection of peers.

## Components
- CIDR: Address space allocated via IPAM. Each peer gets a unique address.
- Gateway / Jump Peers: Provide central routing + optional NAT for full encapsulation.
- Captive portal whitelist: Ties each authenticated peer to a specific public endpoint (`ip:port`); enforced both by the captive portal HTTP server and by iptables on the jump peer.

## IPv4 CIDR

Pick a private RFC 1918 prefix. The dashboard's "Suggest" button queries the server's IPAM for unused candidates sized for your expected peer count.

## IPv6 CIDR (dual-stack)

A network may optionally carry an IPv6 CIDR alongside its IPv4 CIDR. Use a **Unique Local Address** (ULA) prefix per [RFC 4193](https://datatracker.ietf.org/doc/html/rfc4193) — the IPv6 equivalent of RFC 1918. ULAs:

- Live in `fc00::/7`, with the L bit set in practice (so all real-world ULAs are `fd00::/8`).
- Are non-routable on the public internet (just like RFC 1918).
- Use a 40-bit *pseudo-random* Global ID, not a sequential one. Two independent VPNs choosing random Global IDs almost never collide if they ever end up bridged.

The dashboard's **"Suggest ULA prefixes"** button generates random `/64` prefixes client-side using `crypto.getRandomValues`. Each suggestion's Global ID is independently random per RFC 4193 §3.2.2 — clicking *Suggest* again gives a fresh set of candidates.

`/64` is the natural subnet size for IPv6 (peers are addressed at `/128`); you don't need to subnet further unless you have specific routing needs.

:::caution Don't use globally-routable IPv6 prefixes
Never paste a globally-routable prefix from your ISP into the dashboard — the network would conflict with real-world routing and mailing lists are full of "I forgot I configured my VPN with my home IPv6 prefix" stories. The "Suggest" button always produces ULAs.
:::

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
WebSocket notifier pushes update events so agents can refetch config after peer additions, captive portal whitelist updates, or policy changes.
