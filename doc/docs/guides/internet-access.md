---
id: internet-access
title: Peer - Internet Access
sidebar_position: 1
---

Goal: Route a regular peer's full traffic through a jump peer to provide centralized egress.

## Steps
1. Ensure jump peer has NAT interface configured.
2. Edit regular peer: set `full_encapsulation = true`.
3. (Optional) Add additional CIDRs for internal ranges.
4. Agent-based peers: wait for config refresh; static peers: download new config.

## Verification
- Regular peer default route now points to tunnel interface.
- External IP matches jump peer's public IP.

## Notes
Performance depends on jump peer bandwidth and latency. Consider scaling multiple jump peers.
