-- Migration: Fix IPAM allocation consistency issues
-- This migration addresses duplicate key violations in ipam_allocated_ips

-- Step 1: Add a function to handle IP allocation conflicts
CREATE OR REPLACE FUNCTION safe_allocate_ip(p_prefix_cidr TEXT, p_ip TEXT)
RETURNS BOOLEAN AS $$
BEGIN
    INSERT INTO ipam_allocated_ips (prefix_cidr, ip, allocated_at) 
    VALUES (p_prefix_cidr, p_ip, NOW())
    ON CONFLICT (ip) DO NOTHING;
    
    -- Return true if the IP was newly allocated, false if it was already allocated
    RETURN NOT EXISTS (
        SELECT 1 FROM ipam_allocated_ips 
        WHERE ip = p_ip AND prefix_cidr != p_prefix_cidr
    );
END;
$$ LANGUAGE plpgsql;

-- Step 2: Clean up any potential orphaned allocations
-- Remove allocations for IPs that don't belong to their prefix
DELETE FROM ipam_allocated_ips a
WHERE NOT EXISTS (
    SELECT 1 FROM ipam_prefixes p 
    WHERE p.cidr = a.prefix_cidr
);

-- Step 3: Add a check constraint to ensure IPs belong to their prefix (optional, can be heavy)
-- We'll add this as a separate function that can be called for validation
CREATE OR REPLACE FUNCTION validate_ip_in_prefix(p_ip TEXT, p_cidr TEXT)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN (inet(p_ip) << inet(p_cidr));
EXCEPTION WHEN OTHERS THEN
    RETURN FALSE;
END;
$$ LANGUAGE plpgsql;

-- Step 4: Add an index to improve performance on IP lookups
CREATE INDEX IF NOT EXISTS ipam_allocated_ips_ip_idx ON ipam_allocated_ips(ip);

-- Step 5: Clean up any duplicate or invalid entries that might exist
WITH duplicates AS (
    SELECT ip, MIN(allocated_at) as first_allocation
    FROM ipam_allocated_ips
    GROUP BY ip
    HAVING COUNT(*) > 1
)
DELETE FROM ipam_allocated_ips a
WHERE EXISTS (
    SELECT 1 FROM duplicates d 
    WHERE a.ip = d.ip AND a.allocated_at > d.first_allocation
);
