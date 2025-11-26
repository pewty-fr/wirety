-- Migration: add captive portal whitelist table
CREATE TABLE IF NOT EXISTS captive_portal_whitelist (
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    jump_peer_id TEXT NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    peer_ip TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (network_id, jump_peer_id, peer_ip)
);

CREATE INDEX IF NOT EXISTS captive_portal_whitelist_network_idx ON captive_portal_whitelist(network_id);
CREATE INDEX IF NOT EXISTS captive_portal_whitelist_jump_peer_idx ON captive_portal_whitelist(jump_peer_id);
