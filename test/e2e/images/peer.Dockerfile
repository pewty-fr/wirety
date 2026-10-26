# A plain WireGuard client used as a "regular peer" in the e2e harness.
# It runs no agent — the test writes the server-generated WireGuard config and
# brings the tunnel up with wg-quick, then execs curl/dig/wg to probe
# connectivity, DNS and iptables-gated access from the peer's point of view.
# Runs as root + needs NET_ADMIN to create the wg interface. Sleeps forever so
# the test drives it via `docker exec`.
FROM alpine:3.24
# bash: wg-quick is a bash script. iptables: NAT for a second device behind
# the same public IP (network namespace), in the attack scenarios.
RUN apk add --no-cache \
    bash \
    iptables \
    wireguard-tools \
    iproute2 \
    curl \
    bind-tools \
    && rm -rf /var/cache/apk/*
# Keep the container alive; the test execs into it.
ENTRYPOINT ["sleep", "infinity"]
