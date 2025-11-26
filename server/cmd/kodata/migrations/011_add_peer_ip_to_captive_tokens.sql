-- Migration: add peer_ip column to captive_portal_tokens
ALTER TABLE captive_portal_tokens ADD COLUMN IF NOT EXISTS peer_ip TEXT NOT NULL DEFAULT '';

-- Create index on peer_ip for faster lookups
CREATE INDEX IF NOT EXISTS captive_portal_tokens_peer_ip_idx ON captive_portal_tokens(peer_ip);
