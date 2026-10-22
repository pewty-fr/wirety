-- Deleting a network left its range in IPAM when the network still had peers
-- (the prefix still had addresses), which refused any new network overlapping
-- it. Drop the ranges no network uses; their addresses go with them (ON DELETE
-- CASCADE).
DELETE FROM ipam_prefixes
WHERE parent_cidr IS NULL
  AND cidr NOT IN (SELECT cidr FROM networks WHERE cidr IS NOT NULL AND cidr <> '')
  AND cidr NOT IN (SELECT cidr_v6 FROM networks WHERE cidr_v6 IS NOT NULL AND cidr_v6 <> '');
