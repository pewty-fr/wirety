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

# No WireGuard needed — runs anywhere Docker runs (incl. Docker Desktop):
go test -tags e2e -run 'TestStackSmoke|TestCaptivePortalServerFlow' -v ./...
```

First run builds four images (server, agent, dex, peer); subsequent runs reuse
them (`KeepImage: true`). In CI this runs in the **E2E — testcontainers** job of
`.github/workflows/integration.yml`.

## What is asserted

`TestE2E` provisions one topology (network `corp` / `10.90.0.0/24`, domain
`e2e.internal`, a jump peer, a plain WireGuard `peer-a` owned by
`user@example.com`, two private nginx services routed through the jump — only
the first allowed by policy — and a DNS record `app` for it) and runs:

| Subtest | Proves |
|---------|--------|
| `policy_to_iptables` | The server-side policy materialises on the agent as a `WIRETY_POLICY` `ACCEPT` from peer-a's IP to the allowed service, nothing opens the denied one, and the chain is default-deny. **This is the "control the iptables added from the server" assertion.** |
| `private_dns` | The agent's DNS server serves the private-zone FQDN (`app.corp.e2e.internal`) and, because the query comes from an **unauthenticated** source, answers with the captive-portal IP (the jump WG IP) instead of the real service IP. |
| `captive_portal_connectivity` | peer-a brings its tunnel up. **Before auth**: DNS points at the portal, HTTP is intercepted with a 302 to `/captive-portal/start`, and `WIRETY_JUMP` does not whitelist it. The user then signs in with Dex and completes the portal flow as a browser would. **After auth**: `WIRETY_JUMP` sends peer-a to `WIRETY_POLICY`, the allowed service answers 200 through the tunnel, DNS returns the real IP, and the routed-but-not-allowed service stays unreachable. |

Two tests need no WireGuard and run anywhere Docker runs:

- `TestStackSmoke` checks the plumbing: images, DB, OIDC, REST, and that a custom
  `domain_suffix` is persisted.
- `TestCaptivePortalServerFlow` plays the agent (mints the captive token with the
  jump's enrollment token) and the browser, and checks the gates on whitelisting:
  the browser-binding cookie (phishing defense) and peer ownership, then the
  happy path.

## Next increments

1. **Full vs partial encapsulation**: peers with `0.0.0.0/0` vs split routes;
   assert DNS interception of external names and the default-route differences.
2. **Isolated vs shared peers**: peer-to-peer reachability per the isolation flag.
3. **IPv6 / dual-stack**: mirror the scenario with `cidr_v6` + `WIRETY6_*`.

## Layout

```
test/e2e/
  harness.go        container orchestration (network, postgres, dex, server, jump, peer, private-svc)
  api.go            minimal Wirety REST client (decoupled from server internals)
  oidc.go           browser-less Dex login + captive-portal /start and /authenticate
  iptables.go       exec + poll helpers to assert iptables chains inside the agent
  probes.go         dig / curl probes run inside containers
  scenario_basic_test.go   the full scenario (TestE2E)
  captive_flow_test.go     server-side captive-portal flow, no WireGuard
  smoke_test.go     non-WireGuard spine check (TestStackSmoke)
  images/
    peer.Dockerfile        plain WireGuard client (wg-quick + probes)
    dex-config.yaml        Dex config (issuer http://dex:5556/dex, static users)
```

Supporting Dockerfiles live with their component: `server/Dockerfile` and
`agent/Dockerfile.e2e` (root variant of the agent image, with diagnostic tools).

The Dex static users are `admin@example.com` and `user@example.com`, password
`password`. The **first** user the server sees is auto-promoted to administrator.
