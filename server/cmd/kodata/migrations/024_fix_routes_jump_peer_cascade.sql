-- Migration: Change routes.jump_peer_id FK from RESTRICT to CASCADE
-- When a jump peer is deleted, any routes that referenced it become orphans
-- and should be removed automatically.

ALTER TABLE routes DROP CONSTRAINT IF EXISTS routes_jump_peer_id_fkey;
ALTER TABLE routes
    ADD CONSTRAINT routes_jump_peer_id_fkey
    FOREIGN KEY (jump_peer_id) REFERENCES peers(id) ON DELETE CASCADE;
