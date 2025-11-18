---
id: incidents
title: Incidents
sidebar_position: 5
---

Incidents are security anomalies leading to ACL-based blocking.

## Types
| Type | Trigger | Effect |
|------|---------|--------|
| Session Conflict | Multiple simultaneous agent sessions for same peer (within 5 minutes) | Peer blocked in ACL |
| Shared Config | Peer endpoint changes multiple times in less than 30 minutes | Peer blocked (possible config reuse) |
| Suspicious Activity | More than 10 endpoint changes per day | Peer blocked |

## Lifecycle
1. Detect condition (service logic sets incident state).
2. Add peer ID to ACL `BlockedPeers`.
3. WebSocket notifier broadcasts update; agents fetch new config excluding peer.
4. Resolution: authorized user invokes resolve; peer removed from ACL.

## Audit
User resolving incident captured from auth context (email). Falls back to `system` when absent.
