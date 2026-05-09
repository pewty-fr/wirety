-- Migration: add peer_endpoint_ip to captive portal tables
--
-- The captive portal whitelist now stores the peer's public endpoint IP (no port)
-- alongside its WireGuard private IP.  When set, the jump peer verifies that the
-- peer is still connecting from the same public network before treating it as
-- authenticated.  A mismatch (stolen config used from a different network) triggers
-- re-authentication through the captive portal.
--
-- Both columns are nullable for backward compatibility with existing entries and
-- legacy agents that do not send the endpoint IP.

ALTER TABLE captive_portal_whitelist ADD COLUMN IF NOT EXISTS peer_endpoint_ip TEXT;
ALTER TABLE captive_portal_tokens    ADD COLUMN IF NOT EXISTS peer_endpoint_ip TEXT;
