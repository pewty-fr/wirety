-- 027: dual-stack routes and DNS mappings
--
-- A route can now declare a destination CIDR for IPv4 (destination_cidr) AND/OR
-- IPv6 (destination_cidr_v6).  At least one must be set.  Dual-stack routes
-- (both columns populated) end up advertising BOTH CIDRs in the AllowedIPs of
-- attached peers' WireGuard configs — letting a single "internet" route grant
-- full-tunnel for both families with one route entity instead of two.
--
-- Same idea for dns_mappings: a single record can now hold both an IPv4 and
-- an IPv6 address (ip_address + ip_address_v6).  When a peer queries
-- "name.route.suffix":
--   - A query    → returns ip_address    (or NODATA if not set)
--   - AAAA query → returns ip_address_v6 (or NODATA if not set)
-- so admins can manage one record per service instead of two parallel rows.
--
-- Backward compat: columns are added nullable; existing rows that stored an
-- IPv6 value in the "v4-named" column (because there was no other place to
-- put it before this migration) are MOVED to the new _v6 column.  Detection
-- by string content (presence of ":" → IPv6).  After the move we add CHECK
-- constraints so future inserts must populate at least one family.

-- ── routes ─────────────────────────────────────────────────────────────────
ALTER TABLE routes ALTER COLUMN destination_cidr DROP NOT NULL;
ALTER TABLE routes ADD COLUMN destination_cidr_v6 TEXT;

UPDATE routes
   SET destination_cidr_v6 = destination_cidr,
       destination_cidr    = NULL
 WHERE destination_cidr LIKE '%:%';

ALTER TABLE routes
  ADD CONSTRAINT routes_destination_at_least_one_family
  CHECK (destination_cidr IS NOT NULL OR destination_cidr_v6 IS NOT NULL);

-- ── dns_mappings ───────────────────────────────────────────────────────────
ALTER TABLE dns_mappings ALTER COLUMN ip_address DROP NOT NULL;
ALTER TABLE dns_mappings ADD COLUMN ip_address_v6 TEXT;

UPDATE dns_mappings
   SET ip_address_v6 = ip_address,
       ip_address    = NULL
 WHERE ip_address LIKE '%:%';

ALTER TABLE dns_mappings
  ADD CONSTRAINT dns_mappings_address_at_least_one_family
  CHECK (ip_address IS NOT NULL OR ip_address_v6 IS NOT NULL);
