---
id: internet-access
title: Peer - Internet Access
sidebar_position: 1
---

Goal: Route a regular peer's full traffic through a jump peer to provide centralized egress.

:::caution Deprecated approach removed
The `full_encapsulation` peer flag has been **removed**. Use the Groups & Policies system instead (see below).
:::

## Steps (current approach — Policies)

1. Ensure the jump peer has a NAT interface configured.
2. Create (or reuse) a **Group** containing the regular peer.
3. Create a **Policy** on that group with a route rule that sets the jump peer as the default gateway (`0.0.0.0/0`).
4. Attach the policy to the group.
5. Agent-based peers will receive the updated config automatically; static peers must download a new config.

See [Groups, Policies & Routes](../groups-policies-routes-overview) for full details.

## Verification
- Regular peer's default route now points to the tunnel interface.
- External IP matches the jump peer's public IP.

## Notes
Performance depends on jump peer bandwidth and latency. Consider scaling multiple jump peers.
