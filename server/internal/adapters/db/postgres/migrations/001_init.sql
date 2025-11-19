-- Migration: initial schema for Wirety
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    authorized_networks TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS default_permissions (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    default_role TEXT NOT NULL,
    default_authorized_networks TEXT[] NOT NULL DEFAULT '{}'
);
INSERT INTO default_permissions (singleton, default_role) VALUES (TRUE, 'user') ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS networks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    cidr TEXT NOT NULL,
    domain TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS peers (
    id TEXT PRIMARY KEY,
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    public_key TEXT NOT NULL,
    private_key TEXT,
    address TEXT NOT NULL,
    endpoint TEXT,
    listen_port INTEGER,
    additional_allowed_ips TEXT[] NOT NULL DEFAULT '{}',
    token TEXT,
    is_jump BOOLEAN NOT NULL DEFAULT FALSE,
    jump_nat_interface TEXT,
    is_isolated BOOLEAN NOT NULL DEFAULT FALSE,
    full_encapsulation BOOLEAN NOT NULL DEFAULT FALSE,
    use_agent BOOLEAN NOT NULL DEFAULT TRUE,
    owner_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(network_id, public_key)
);
CREATE INDEX IF NOT EXISTS peers_network_idx ON peers(network_id);

CREATE TABLE IF NOT EXISTS peer_connections (
    peer1_id TEXT NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    peer2_id TEXT NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    preshared_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (peer1_id, peer2_id)
);

CREATE TABLE IF NOT EXISTS agent_sessions (
    session_id TEXT PRIMARY KEY,
    peer_id TEXT NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    hostname TEXT,
    system_uptime BIGINT,
    wireguard_uptime BIGINT,
    reported_endpoint TEXT,
    last_seen TIMESTAMPTZ NOT NULL,
    first_seen TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS agent_sessions_peer_idx ON agent_sessions(peer_id);

CREATE TABLE IF NOT EXISTS endpoint_changes (
    id BIGSERIAL PRIMARY KEY,
    peer_id TEXT NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    old_endpoint TEXT,
    new_endpoint TEXT,
    changed_at TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS endpoint_changes_peer_idx ON endpoint_changes(peer_id);

CREATE TABLE IF NOT EXISTS security_incidents (
    id TEXT PRIMARY KEY,
    peer_id TEXT NOT NULL REFERENCES peers(id) ON DELETE SET NULL,
    peer_name TEXT,
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE SET NULL,
    network_name TEXT,
    incident_type TEXT NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL,
    public_key TEXT,
    endpoints TEXT[] NOT NULL DEFAULT '{}',
    details TEXT,
    resolved BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_at TIMESTAMPTZ,
    resolved_by TEXT
);

-- Simple migration table
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
