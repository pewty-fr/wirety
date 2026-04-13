---
id: isolate-peer
title: Peer - Isolation
sidebar_position: 2
---

Goal: Prevent lateral communication between one peer and others while retaining jump connectivity.

:::caution Deprecated approach removed
The `is_isolated` peer flag has been **removed**. Use the Groups & Policies system instead (see below).
:::

## Steps (current approach — Policies)

1. Create (or reuse) a **Group** containing only the peer you want to isolate.
2. Create a **Policy** with rules that deny peer-to-peer traffic while allowing traffic to/from the jump peer.
3. Attach the policy to the group.
4. Agent-based peers update automatically; static peers must download a new config.

See [Groups, Policies & Routes](../groups-policies-routes-overview) for full details.

## Verification
- Ping from the isolated peer to another regular peer fails.
- Ping to the jump peer succeeds.

## Use Cases
- Untrusted device.
- Staging environment host.
- Guest access.
