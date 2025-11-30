-- Migration: Add priority column to groups table

-- Add priority column with default value of 100
ALTER TABLE groups ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 100;

-- Add check constraint to ensure priority is between 0 and 999
ALTER TABLE groups ADD CONSTRAINT check_group_priority CHECK (priority >= 0 AND priority <= 999);

-- Update existing quarantine groups to have priority 0 (highest priority)
UPDATE groups SET priority = 0 WHERE LOWER(name) = 'quarantine';

-- Create index on priority for efficient sorting
CREATE INDEX IF NOT EXISTS idx_groups_priority ON groups(priority);
