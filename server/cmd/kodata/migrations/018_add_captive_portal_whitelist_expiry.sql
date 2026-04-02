-- Migration: add expiry to captive portal whitelist entries
-- Entries without expires_at are treated as never-expiring (legacy behaviour).
ALTER TABLE captive_portal_whitelist
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS captive_portal_whitelist_expires_idx
    ON captive_portal_whitelist(expires_at)
    WHERE expires_at IS NOT NULL;
