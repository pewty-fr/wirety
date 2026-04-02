-- Add separate thresholds for port-only changes (NAT rebinding) vs IP-level changes (shared config).
-- Port-only changes are normal behavior; IP changes are suspicious.
ALTER TABLE security_configs
    ADD COLUMN IF NOT EXISTS port_change_threshold BIGINT NOT NULL DEFAULT 300000000000,       -- 5 minutes in nanoseconds
    ADD COLUMN IF NOT EXISTS max_port_changes_per_window INTEGER NOT NULL DEFAULT 5;
