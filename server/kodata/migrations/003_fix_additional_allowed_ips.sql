-- Migration: Fix additional_allowed_ips null constraint issue
-- This migration addresses the issue where additional_allowed_ips might be inserted as NULL
-- violating the NOT NULL constraint

-- Step 1: Temporarily allow NULL values for existing data
ALTER TABLE peers ALTER COLUMN additional_allowed_ips DROP NOT NULL;

-- Step 2: Update any existing NULL values to empty arrays
UPDATE peers SET additional_allowed_ips = '{}' WHERE additional_allowed_ips IS NULL;

-- Step 3: Re-add the NOT NULL constraint with proper default
ALTER TABLE peers ALTER COLUMN additional_allowed_ips SET NOT NULL;
ALTER TABLE peers ALTER COLUMN additional_allowed_ips SET DEFAULT '{}';

-- Step 4: Create a trigger function to ensure empty arrays instead of NULL
CREATE OR REPLACE FUNCTION ensure_non_null_additional_allowed_ips()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.additional_allowed_ips IS NULL THEN
        NEW.additional_allowed_ips := '{}';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Step 5: Create the trigger
DROP TRIGGER IF EXISTS peers_ensure_additional_allowed_ips ON peers;
CREATE TRIGGER peers_ensure_additional_allowed_ips
    BEFORE INSERT OR UPDATE ON peers
    FOR EACH ROW
    EXECUTE FUNCTION ensure_non_null_additional_allowed_ips();
