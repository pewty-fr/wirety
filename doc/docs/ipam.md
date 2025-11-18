---
id: ipam
title: IPAM
sidebar_position: 4
---

Wirety uses an internal IP Address Management (IPAM) component:

## Allocation
- On peer creation, the next free IP within network CIDR is acquired.
- IP assigned is persisted in repository.

## Release
- On peer deletion or CIDR migration, address is released back.
- Logging warns if release fails but continues processing.

## CIDR Change Constraints
Static peers (non-agent) block CIDR changes to avoid manual reconfig burden.

## Collision Avoidance
Repository ensures uniqueness; attempts to allocate an already taken IP fail and retried.

## Future Enhancements
- Reserved IP ranges.
- Bulk allocation preview.
- Metrics export of utilization.
