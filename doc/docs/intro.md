---
id: intro
title: Get Started
sidebar_position: 1
---

Welcome to Wirety: a secure, dynamic WireGuard mesh with agent-based automation, ACL-driven incident response, and flexible peer types.

## Overview
Wirety orchestrates a WireGuard overlay by distinguishing Jump Peers (traffic hubs) and Regular Peers which can be either Dynamic (agent-enrolled) or Static (manual config). Security incidents are handled via ACL blocking instead of connection deletion for safer, auditable containment.

## Architecture Highlights
- Jump peers route and optionally NAT traffic for regular peers.
- Dynamic peers enroll using an agent + token; static peers receive a generated WireGuard config.
- ACL holds `BlockedPeers` for incident containment; resolving removes the peer from ACL.
- Private keys never leave the server API responses (`json:"-"`).

## Prerequisites
| Component | Purpose | Status |
|-----------|---------|--------|
| Kubernetes cluster | Run Wirety Helm chart | Required |
| DNS entry | Expose front + server (e.g. `wirety.example.com`) | Recommended |
| WireGuard installed | For static peers (phones, laptops) | Conditional |
| curl + bash | For agent install | Required on dynamic hosts |

## Install (Helm)
```bash
# Install Wirety using OCI registry
helm install wirety oci://rg.fr-par.scw.cloud/wirety/chart/wirety \
  --version <version> \
  --namespace wirety \
  --create-namespace \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=wirety.example.com \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix
```
For detailed deployment options, see the [Deployment Guide](./deployment).

## Create a Jump Peer
1. Log into Wirety Frontend.
2. Navigate: Networks -> Select network -> Add Peer -> Jump Server.
3. Provide name, endpoint (`PUBLIC_IP:51820`), listen port if different.
4. Save. A token is generated (accessible via "View Token").

## Install the Agent (Jump Peer)
Retrieve the token from the peer view, then on the jump host:
```bash
# Download the agent binary
curl -fsSL https://github.com/pewty-fr/wirety/releases/latest/download/wirety-agent-linux-amd64 -o /usr/local/bin/wirety-agent
chmod +x /usr/local/bin/wirety-agent

# Run the agent with the enrollment token
wirety-agent -server https://wirety.example.com -token <ENROLLMENT_TOKEN>

# Or set environment variables and run
export SERVER_URL=https://wirety.example.com
export TOKEN=<ENROLLMENT_TOKEN>
wirety-agent
```
The agent establishes a heartbeat (hostname, uptime, endpoint) and retrieves updated config when ACL or peers change.

## Add a Static Peer (e.g. Phone)
1. Add Regular Peer (toggle OFF "Use Agent").
2. Download/View WireGuard config from peer view.
3. Import into WireGuard mobile app.
4. Activate tunnel. Traffic may route via jump depending on encapsulation settings.

## Verify Connectivity
- Ping between regular peers (unless isolated).
- Confirm allowed IPs in config reflect additional ranges if configured.

## Next Steps
- Explore Network constraints (CIDR changes blocked when static peers exist).
- Review Incident handling and resolution flows.
- Integrate OIDC for centralized auth (see Guides / OIDC).
