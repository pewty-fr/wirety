---
id: isolate-peer
title: Peer - Isolatation
sidebar_position: 2
---

Goal: Prevent lateral communication between one regular peer and others while retaining jump connectivity.

## Steps
1. Edit peer: toggle `is_isolated = true`.
2. Save changes; agent refresh or static config download.
3. Jump peer routes still allowed; direct peer-to-peer sessions excluded from AllowedIPs.

## Verification
- Ping from isolated peer to another regular peer fails.
- Ping to jump peer succeeds.

## Use Cases
- Untrusted device.
- Staging environment host.
