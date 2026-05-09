-- Migration: add peer_endpoint to captive portal tables
--
-- The captive portal whitelist stores the peer's full public endpoint
-- ("ip:port", strict) alongside its WireGuard private IP.  When set, the
-- jump peer requires the peer's current source IP+port to match the one
-- recorded at authentication time — any change (different network, NAT port
-- rebinding, tunnel restart) triggers re-authentication.
--
-- Both columns are nullable for backward compatibility with existing entries
-- and legacy agents that do not send the endpoint.

ALTER TABLE captive_portal_whitelist ADD COLUMN IF NOT EXISTS peer_endpoint TEXT;
ALTER TABLE captive_portal_tokens    ADD COLUMN IF NOT EXISTS peer_endpoint TEXT;
