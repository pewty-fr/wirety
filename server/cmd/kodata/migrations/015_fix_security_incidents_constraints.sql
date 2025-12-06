-- Migration: fix security_incidents foreign key constraints
-- The issue: columns are NOT NULL but have ON DELETE SET NULL, which is contradictory

-- Option 1: Change to CASCADE delete (recommended)
-- When a network/peer is deleted, delete related security incidents
ALTER TABLE security_incidents DROP CONSTRAINT IF EXISTS security_incidents_network_id_fkey;
ALTER TABLE security_incidents DROP CONSTRAINT IF EXISTS security_incidents_peer_id_fkey;

ALTER TABLE security_incidents 
ADD CONSTRAINT security_incidents_network_id_fkey 
FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE CASCADE;

ALTER TABLE security_incidents 
ADD CONSTRAINT security_incidents_peer_id_fkey 
FOREIGN KEY (peer_id) REFERENCES peers(id) ON DELETE CASCADE;

-- Alternative Option 2: Allow NULL values (uncomment if you prefer this approach)
-- ALTER TABLE security_incidents ALTER COLUMN network_id DROP NOT NULL;
-- ALTER TABLE security_incidents ALTER COLUMN peer_id DROP NOT NULL;
-- 
-- ALTER TABLE security_incidents 
-- ADD CONSTRAINT security_incidents_network_id_fkey 
-- FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE SET NULL;
-- 
-- ALTER TABLE security_incidents 
-- ADD CONSTRAINT security_incidents_peer_id_fkey 
-- FOREIGN KEY (peer_id) REFERENCES peers(id) ON DELETE SET NULL;
