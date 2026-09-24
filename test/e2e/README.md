# Wirety end-to-end tests (testcontainers)

Black-box e2e harness that replicates a real Wirety deployment in Docker and
asserts the behaviour a host with an agent exhibits: the **iptables state the
agent programs from server-side policy**, **WireGuard peer connectivity**, the
**captive-portal auth flow**, and **private DNS zone resolution** — with full or
partial encapsulation.

```
        ┌───────────┐        ┌──────────────────────────┐
        │  postgres │◄───────│  wirety-server (OIDC/DB)  │
        └───────────┘        └────────────┬─────────────┘
        ┌───────────┐   REST: networks,    │
        │    dex     │◄── peers, routes,   │  WS push (policy, whitelist,
        │  (OIDC)    │    DNS, groups,      │  peer_routes)
        └───────────┘    policies          ▼
                                   ┌──────────────┐   WG tunnel   ┌────────┐
                                   │  jump-agent  │◄─────────────►│ peer-a │
                                   │ WIRETY_JUMP  │               └────────┘
                                   │ WIRETY_POLICY│
                                   └──────┬───────┘
                                          │ forward / masquerade
                                          ▼
                                   ┌──────────────┐
                                   │ private-svc  │  (nginx, reached via a route,
                                   └──────────────┘   named in a private DNS zone)
```

Everything is a separate Go module (`wirety/test/e2e`) gated behind the `e2e`
build tag, so it never runs as part of `go test ./...` in the server or agent.

## Requirements

- **Linux host** with **Docker** and the **WireGuard kernel module**
  (`sudo modprobe wireguard`). The jump/peer containers are privileged and
  create real `wg` interfaces using the host kernel module.
- Docker Desktop on macOS/Windows generally **cannot** run the WireGuard parts
  (its VM kernel usually lacks the module). The smoke test (below) still works.
- Go ≥ 1.26.

## Running

```bash
cd test/e2e

# Full scenario (needs Linux + WireGuard kernel module):
go test -tags e2e -timeout 25m -v ./...

# Fast spine check — no WireGuard, runs anywhere Docker runs
# (server/dex images build, Postgres, OIDC admin token, REST API):
go test -tags e2e -run TestStackSmoke -v ./...
```

First run builds four images (server, agent, dex, peer); subsequent runs reuse
them (`KeepImage: true`). In CI this runs in the **E2E — testcontainers** job of
`.github/workflows/integration.yml`.

## What is asserted

`TestE2E` provisions one topology (network `corp` / `10.90.0.0/24`, domain
`e2e.internal`, a jump peer, `peer-a`, a route + DNS record to a private nginx,
and a group/policy that **allows peer-a → the private service**) and runs:

| Subtest | Proves |
|---------|--------|
| `policy_to_iptables` | The server-side policy materialises on the agent as a `WIRETY_POLICY` `ACCEPT` from peer-a's IP to the service CIDR, and the chain is default-deny. **This is the "control the iptables added from the server" assertion.** |
| `private_dns` | The agent's DNS server serves the private-zone FQDN (`app.corp.e2e.internal`) and, because the query comes from an **unauthenticated** source, answers with the captive-portal IP (the jump WG IP) instead of the real service IP. |
| `captive_portal_connectivity` | Brings peer-a's WireGuard tunnel up. **Extension point** — see below. |

`TestStackSmoke` validates the non-WireGuard spine only (images, DB, OIDC,
REST) and is the quickest way to confirm the harness plumbing.

## Status: vertical slice

This is the **first vertical slice**. The infrastructure (image builds, network,
OIDC password-grant admin auth, REST client, privileged agent container, iptables
inspection helpers, browser-less OIDC helpers in `oidc.go`) is complete and the
core asserts above pass. The following are the **next increments**, wired but not
yet asserting:

1. **Captive-portal-gated connectivity** (`captive_portal_connectivity`): after
   bringing the tunnel up, prove the peer is blocked, complete the captive-portal
   flow with `wiretySession()` (browser-less Dex login) + `/captive-portal/start`
   + `/captive-portal/authenticate`, assert the `WIRETY_JUMP` whitelist rule
   appears, then `curl` the private service (allow) and a denied CIDR (reject).
   The jump-peer `endpoint` must first be updated to the agent container's IP
   (`jump.ContainerIP`) so `peer-a`'s config resolves.
2. **Full vs partial encapsulation**: add peers with `full_encapsulation` on/off
   and assert the DNS interception / default-route differences.
3. **Isolated vs shared peers**: assert peer-to-peer reachability per the
   isolation flag.
4. **IPv6 / dual-stack**: mirror the scenario with `cidr_v6` + `WIRETY6_*`.

## Layout

```
test/e2e/
  harness.go        container orchestration (network, postgres, dex, server, jump, peer, private-svc)
  api.go            minimal Wirety REST client (decoupled from server internals)
  oidc.go           browser-less Dex: password-grant admin token + code-flow session
  iptables.go       exec + poll helpers to assert iptables chains inside the agent
  scenario_basic_test.go   the vertical-slice scenario (TestE2E)
  smoke_test.go     non-WireGuard spine check (TestStackSmoke)
  images/
    peer.Dockerfile        plain WireGuard client (wg-quick + probes)
    dex-config.yaml        Dex config (issuer http://dex:5556/dex, static users)
```

Supporting Dockerfiles live with their component: `server/Dockerfile` and
`agent/Dockerfile.e2e` (root variant of the agent image, with diagnostic tools).

The Dex static users are `admin@example.com` and `user@example.com`, password
`password`. The **first** user the server sees is auto-promoted to administrator.
