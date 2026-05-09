-- Migration: drop security-incident infrastructure
--
-- v2 simplification: the captive portal now performs an endpoint check on every
-- authenticated connection, which provides a stronger guarantee than after-the-
-- fact heartbeat-driven incident detection.  All incident state, endpoint-change
-- history, and per-network security configuration are removed.

DROP TABLE IF EXISTS security_incidents;
DROP TABLE IF EXISTS security_configs;
DROP TABLE IF EXISTS endpoint_changes;
