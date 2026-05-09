-- Migration: add captive portal security primitives
--   1. captive_portal_endpoint_denylist  — per-peer rogue source IP:port blocks
--   2. captive_portal_quarantine         — per-peer auth-failure strike state
--   3. peer_local_routes                 — per-peer reported AllowedIPs (heartbeat)

-- 1. Endpoint denylist: rogue WireGuard UDP sources that the jump peer must
--    drop at the physical-interface layer.  Each entry targets a specific peer
--    (wg_ip): when the agent observes that peer's WireGuard endpoint has been
--    overwritten by an unauthorised source, it reports the rogue source here.
CREATE TABLE IF NOT EXISTS captive_portal_endpoint_denylist (
    network_id   TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    jump_peer_id TEXT NOT NULL REFERENCES peers(id)    ON DELETE CASCADE,
    wg_ip        TEXT NOT NULL,                 -- targeted peer's WireGuard private IP
    blocked_ip   TEXT NOT NULL,                 -- rogue source IP
    blocked_port INTEGER NOT NULL DEFAULT 0,    -- rogue source port (0 = any)
    reason       TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (network_id, jump_peer_id, wg_ip, blocked_ip, blocked_port)
);

CREATE INDEX IF NOT EXISTS captive_portal_endpoint_denylist_jump_idx
    ON captive_portal_endpoint_denylist(network_id, jump_peer_id);

CREATE INDEX IF NOT EXISTS captive_portal_endpoint_denylist_expires_idx
    ON captive_portal_endpoint_denylist(expires_at);

-- 2. Quarantine: tracks consecutive failed/abandoned captive portal auth attempts.
--    A "strike" is recorded when an issued token expires without a successful
--    AuthenticateCaptivePortal call.  After QuarantineStrikeThreshold strikes
--    the peer is quarantined (blocked entirely) until quarantined_until passes.
CREATE TABLE IF NOT EXISTS captive_portal_quarantine (
    network_id        TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    peer_id           TEXT NOT NULL REFERENCES peers(id)    ON DELETE CASCADE,
    strikes           INTEGER NOT NULL DEFAULT 0,
    last_strike_at    TIMESTAMPTZ,
    quarantined_until TIMESTAMPTZ,
    PRIMARY KEY (network_id, peer_id)
);

CREATE INDEX IF NOT EXISTS captive_portal_quarantine_until_idx
    ON captive_portal_quarantine(quarantined_until)
    WHERE quarantined_until IS NOT NULL;

-- 2b. Track when a captive portal token is "consumed" (successfully exchanged
--     for a whitelist entry).  Tokens that expire without ever being consumed
--     count as a strike towards quarantine.
ALTER TABLE captive_portal_tokens
    ADD COLUMN IF NOT EXISTS consumed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS captive_portal_tokens_consumed_idx
    ON captive_portal_tokens(consumed_at);

-- 3. Peer-reported local routes: the peer's own AllowedIPs as configured in its
--    WireGuard config.  Reported via the agent heartbeat.  Consumed by the jump
--    peer's DNS server: full-tunnel peers (0.0.0.0/0 in their AllowedIPs) get
--    aggressive external-DNS redirection while unauthenticated; split-tunnel
--    peers leave external resolution alone (their external traffic doesn't
--    flow through the jump peer anyway).
CREATE TABLE IF NOT EXISTS peer_local_routes (
    network_id  TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    peer_id     TEXT NOT NULL REFERENCES peers(id)    ON DELETE CASCADE,
    allowed_ips TEXT NOT NULL DEFAULT '[]',  -- JSON array of CIDRs
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (network_id, peer_id)
);
