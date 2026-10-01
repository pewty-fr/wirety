-- 028: captive-portal access history
--
-- Records every captive-portal sign-in and every end of access with its cause
-- (session expired, tunnel inactive, public IP changed, revoked), so admins and
-- users can see why a peer had to go back through the captive portal.
CREATE TABLE IF NOT EXISTS captive_portal_events (
    id         BIGSERIAL PRIMARY KEY,
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    peer_id    TEXT NOT NULL REFERENCES peers(id)    ON DELETE CASCADE,
    peer_ip    TEXT NOT NULL,
    event      TEXT NOT NULL,
    detail     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS captive_portal_events_peer_idx
    ON captive_portal_events(network_id, peer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS captive_portal_events_created_idx
    ON captive_portal_events(created_at);
