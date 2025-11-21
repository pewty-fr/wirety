-- Migration 006: Remove domain column from networks table
-- The domain field is now computed as network_name.local instead of being stored

ALTER TABLE networks DROP COLUMN IF EXISTS domain;
