-- Migration: add captive portal authentication tokens
CREATE TABLE IF NOT EXISTS captive_portal_tokens (
    token TEXT PRIMARY KEY,
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    jump_peer_id TEXT NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS captive_portal_tokens_network_idx ON captive_portal_tokens(network_id);
CREATE INDEX IF NOT EXISTS captive_portal_tokens_jump_peer_idx ON captive_portal_tokens(jump_peer_id);
CREATE INDEX IF NOT EXISTS captive_portal_tokens_expires_idx ON captive_portal_tokens(expires_at);
