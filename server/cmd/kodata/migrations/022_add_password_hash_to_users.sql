-- Migration: add password_hash column to users
--
-- v2 simplification: when AUTH_ENABLED=false, an admin can create local users
-- via the dashboard.  Each local user gets a bcrypt password hash stored here
-- and can log in with email + password.  OIDC-managed users have NULL here —
-- their identity is verified by the OIDC provider on every session refresh.

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;
