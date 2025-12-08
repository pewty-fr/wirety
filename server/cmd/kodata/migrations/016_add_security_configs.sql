-- Security configurations table
CREATE TABLE IF NOT EXISTS security_configs (
    id TEXT PRIMARY KEY,
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT true,
    session_conflict_threshold BIGINT NOT NULL DEFAULT 300000000000, -- 5 minutes in nanoseconds
    endpoint_change_threshold BIGINT NOT NULL DEFAULT 1800000000000, -- 30 minutes in nanoseconds
    max_endpoint_changes_per_day INTEGER NOT NULL DEFAULT 10,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    UNIQUE(network_id)
);

CREATE INDEX IF NOT EXISTS idx_security_configs_network_id ON security_configs(network_id);
