-- Migration: Remove legacy ACL system and peer isolation flags

-- Drop ACL tables if they exist (they may not exist in current schema)
DROP TABLE IF EXISTS acl_rules CASCADE;
DROP TABLE IF EXISTS acls CASCADE;

-- Remove legacy peer flags
ALTER TABLE peers DROP COLUMN IF EXISTS is_isolated;
ALTER TABLE peers DROP COLUMN IF EXISTS full_encapsulation;
