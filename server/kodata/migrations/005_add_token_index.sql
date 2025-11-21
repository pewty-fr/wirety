-- Migration: Add index on peers.token for efficient agent token resolution
-- This migration adds an index to improve the performance of token lookups for agent resolution

-- Add index on token column for efficient lookups
CREATE INDEX IF NOT EXISTS peers_token_idx ON peers(token) WHERE token IS NOT NULL;
