-- 023_add_ipv6_support.sql
-- Add IPv6 CIDR to networks and IPv6 address to peers for dual-stack support.

ALTER TABLE networks ADD COLUMN IF NOT EXISTS cidr_v6 TEXT;
ALTER TABLE peers    ADD COLUMN IF NOT EXISTS address_v6 TEXT;
