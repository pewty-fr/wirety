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
- Docker Desktop on macOS runs the full suite too when its VM kernel ships
  WireGuard (recent versions do; check with
  `docker run --rm --privileged alpine ip link add wg9 type wireguard`).
  Without it, the tests below that need no WireGuard still run.
- Go ≥ 1.26.

## Running

```bash
cd test/e2e

# Full scenario (needs Linux + WireGuard kernel module):
go test -tags e2e -timeout 25m -v ./...

# No WireGuard needed — runs anywhere Docker runs (incl. Docker Desktop):
go test -tags e2e -run 'TestStackSmoke|TestCaptivePortalServerFlow' -v ./...  # incl. ...FlowIPv6
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
| `private_dns` | The agent's DNS server resolves the private-zone FQDN (`app.corp.e2e.internal`) to the **real** service IP even for an unauthenticated source (DNS is not the access boundary, iptables is), while an OS captive-portal probe host (`captive.apple.com`) is steered to the portal. |
| `captive_portal_connectivity` | peer-a brings its tunnel up. **Before auth**: DNS gives the real service IP, yet HTTP to it is intercepted with a 302 to `/captive-portal/start`; the probe host points at the portal; `WIRETY_JUMP` does not whitelist peer-a. The user then signs in with Dex and completes the portal flow as a browser would. **After auth**: `WIRETY_JUMP` sends peer-a to `WIRETY_POLICY`, the allowed service answers 200 through the tunnel, the probe host is no longer steered to the portal, and the routed-but-not-allowed service stays unreachable. |

`TestE2EIPv6` runs the same topology **dual-stack**: the docker network gets an
IPv6 ULA subnet, the Wirety network a `cidr_v6`, and the routes, DNS record and
policy carry both families.

| Subtest | Proves |
|---------|--------|
| `policy_to_ip6tables` | The IPv6 policy materialises as a `WIRETY6_POLICY` `ACCEPT` from peer-a's IPv6 to the service's IPv6, nothing opens the denied service, and the chain is default-deny. |
| `captive_portal_dual_stack` | **Before auth**: `A`/`AAAA` return the real addresses, HTTP to the service's **IPv6** address is intercepted (no IPv6 bypass), the probe host queried over the agent's **IPv6** DNS listener points at the portal, and `WIRETY6_JUMP` does not whitelist peer-a. The user authenticates the token that was issued for the **IPv6** address. **After auth**: both `WIRETY_JUMP` and `WIRETY6_JUMP` open, the service answers 200 over IPv6 and IPv4, the probe host is released over IPv6 too, and the denied service stays unreachable over IPv6. |

`TestE2ESNIProxy` puts the server behind a **shared TLS ingress** (one IP:443
serving the Wirety host and another app) and runs the agent the way it is
deployed behind an ingress (`-server https://<ip> -server-host … -portal-url …`).
Before auth the peer reaches the Wirety host through the agent's SNI proxy but
not the other app on the same IP:443; after auth it reaches both.

`TestAttackScenarios` plays an attacker against a live deployment (victim device
signed in, a second device that never signs in, an attacker holding a copy of the
victim's WireGuard config):

| Subtest | Attack | Expected defense |
|---------|--------|------------------|
| `unauthenticated_device_blocked` | Reach the private service without signing in (HTTP, ICMP, HTTPS) | HTTP redirected to the portal, the rest dropped / reset |
| `spoofed_vpn_source_dropped` | Forge the signed-in device's VPN IP as source | Dropped by WireGuard cryptokey routing on the jump |
| `regular_agent_token_cannot_mint_portal_tokens` | Use a regular device's enrollment token to mint a portal token | `403`: only jump peers issue tokens |
| `stolen_config_owner_offline` | Use the stolen config while the owner is offline | Session bound to the signing-in public IP: portal only |
| `phishing_link_shows_attacker_ip` | Send the victim a portal link triggered from the stolen config | The portal page shows the attacker's public IP |
| `stolen_config_concurrent_use_denylisted` | Use the stolen config while the owner is online | Oscillation detected, attacker's source dropped at the jump, owner keeps access |
| `ssh_on_private_host_unauthenticated` | Reach SSH (port 22) on the private host without signing in | Dropped |
| `ssh_on_jump_host_unauthenticated` | Reach SSH on the jump host itself (its WireGuard IP) without signing in | Dropped by `WIRETY_INPUT` |
| `ssh_on_jump_host_authenticated` | Reach SSH on the jump host from a signed-in device (the jump runs with `-jump-host-ports 22/tcp`) | Allowed: opened to signed-in peers |
| `jump_host_port_not_opened` | Reach another service of the jump host (port 9100) from a signed-in device | Dropped: policies govern what is routed through the jump, not the jump itself |
| `jump_services_reachable_unauthenticated` | — (non-regression) | Before signing in, DNS and ping to the jump still work |
| `ssh_on_jump_host_from_private_network` | — (no lock-out) | SSH to the jump from its private network (bastion path) is not filtered |

`TestE2ESNIProxy` also tries **domain fronting** (allowed SNI, another vhost's
`Host`); the test ingress is hardened to refuse a Host/SNI mismatch, as
production ingresses must be.

`TestCaptivePortalSessionLifecycle` follows captive-portal sessions with a short
`CAPTIVE_PORTAL_SESSION_TTL`. Signed in, the peer (flagged `use_agent`) reaches
a private host over HTTP and SSH and the jump host over SSH (opened with
`-jump-host-ports 22/tcp`), keeps SSH sessions open to both, and stays signed in
across jump heartbeats.

| Subtest | Proves |
|---------|--------|
| `access_survives_heartbeats` | The access and the open SSH session last across jump heartbeats. |
| `expired_session_cuts_http_and_ssh` | When the session duration is reached, HTTP goes back to the portal, new SSH connections are dropped, and the SSH sessions opened while signed in (private host and jump) stop carrying data. |
| `revoked_session_cuts_http_and_ssh` | After signing in again, a **Revoke Auth** from the dashboard cuts HTTP and SSH the same way. |

The portal page then tells why the previous session ended, and the access
history records sign-in → expired → sign-in → revoked, each sign-in bound to
the device's public IP.

`TestSessionSurvivesTokenRefresh` keeps a dashboard session busy with bursts of
concurrent API calls (like the frontend's polling) across six OIDC token
lifetimes (Dex with 10 s tokens and rotating refresh tokens): every call must
succeed, the server refreshing the tokens transparently. It needs no WireGuard.

Three more tests need no WireGuard and run anywhere Docker runs:

- `TestStackSmoke` checks the plumbing: images, DB, OIDC, REST, and that a custom
  `domain_suffix` is persisted.
- `TestCaptivePortalServerFlow` plays the agent (mints the captive token with the
  jump's enrollment token) and the browser, and checks the gates on whitelisting:
  the browser-binding cookie (phishing defense) and peer ownership, then the
  happy path.
- `TestCaptivePortalServerFlowIPv6` does the same for a dual-stack peer whose
  captive token was issued for its IPv6 address.

## Next increments

1. **Full vs partial encapsulation**: peers with `0.0.0.0/0` vs split routes;
   assert DNS interception of external names and the default-route differences.
2. **Isolated vs shared peers**: peer-to-peer reachability per the isolation flag.
3. **IPv6-only networks** (no IPv4 `cidr`).

## Layout

```
test/e2e/
  harness.go        container orchestration (network, postgres, dex, server, jump, peer, private-svc)
  api.go            minimal Wirety REST client (decoupled from server internals)
  oidc.go           browser-less Dex login + captive-portal /start and /authenticate
  iptables.go       exec + poll helpers to assert iptables chains inside the agent
  probes.go         dig / curl probes run inside containers
  scenario_basic_test.go   the full scenario (TestE2E)
  scenario_ipv6_test.go    the dual-stack scenario (TestE2EIPv6)
  scenario_sni_test.go     shared-ingress vhost isolation (TestE2ESNIProxy)
  captive_flow_test.go     server-side captive-portal flow, no WireGuard
  attack_test.go           attack scenarios against a live deployment
  ssh.go                   fake SSH server, banner probe and long-lived session
  captive_session_test.go  captive-portal session duration, expiry and access history
  session_refresh_test.go  OIDC token refresh under concurrent load
  smoke_test.go     non-WireGuard spine check (TestStackSmoke)
  images/
    peer.Dockerfile        plain WireGuard client (wg-quick + probes)
    dex-config.yaml        Dex config (issuer http://dex:5556/dex, static users)
```

Supporting Dockerfiles live with their component: `server/Dockerfile` and
`agent/Dockerfile.e2e` (root variant of the agent image, with diagnostic tools).

The Dex static users are `admin@example.com` and `user@example.com`, password
`password`. The **first** user the server sees is auto-promoted to administrator.
