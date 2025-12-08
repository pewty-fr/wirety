-- Migration: Add groups, policies, routes, and DNS mappings

-- Groups table
CREATE TABLE IF NOT EXISTS groups (
    id TEXT PRIMARY KEY,
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(network_id, name)
);

CREATE INDEX IF NOT EXISTS idx_groups_network_id ON groups(network_id);

-- Group-Peer junction table (many-to-many)
CREATE TABLE IF NOT EXISTS group_peers (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    peer_id TEXT NOT NULL REFERENCES peers(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, peer_id)
);

CREATE INDEX IF NOT EXISTS idx_group_peers_peer_id ON group_peers(peer_id);
CREATE INDEX IF NOT EXISTS idx_group_peers_group_id ON group_peers(group_id);

-- Policies table
CREATE TABLE IF NOT EXISTS policies (
    id TEXT PRIMARY KEY,
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(network_id, name)
);

CREATE INDEX IF NOT EXISTS idx_policies_network_id ON policies(network_id);

-- Policy rules table
CREATE TABLE IF NOT EXISTS policy_rules (
    id TEXT PRIMARY KEY,
    policy_id TEXT NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    direction TEXT NOT NULL CHECK (direction IN ('input', 'output')),
    action TEXT NOT NULL CHECK (action IN ('allow', 'deny')),
    target TEXT NOT NULL,
    target_type TEXT NOT NULL CHECK (target_type IN ('cidr', 'peer', 'group')),
    description TEXT,
    rule_order INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_policy_rules_policy_id ON policy_rules(policy_id);

-- Group-Policy junction table (many-to-many)
CREATE TABLE IF NOT EXISTS group_policies (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    policy_id TEXT NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    attached_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    policy_order INTEGER NOT NULL,
    PRIMARY KEY (group_id, policy_id)
);

CREATE INDEX IF NOT EXISTS idx_group_policies_policy_id ON group_policies(policy_id);
CREATE INDEX IF NOT EXISTS idx_group_policies_group_id ON group_policies(group_id);

-- Routes table
CREATE TABLE IF NOT EXISTS routes (
    id TEXT PRIMARY KEY,
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    destination_cidr TEXT NOT NULL,
    jump_peer_id TEXT NOT NULL REFERENCES peers(id) ON DELETE RESTRICT,
    domain_suffix TEXT NOT NULL DEFAULT 'internal',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(network_id, name)
);

CREATE INDEX IF NOT EXISTS idx_routes_network_id ON routes(network_id);
CREATE INDEX IF NOT EXISTS idx_routes_jump_peer_id ON routes(jump_peer_id);

-- Group-Route junction table (many-to-many)
CREATE TABLE IF NOT EXISTS group_routes (
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    route_id TEXT NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    attached_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, route_id)
);

CREATE INDEX IF NOT EXISTS idx_group_routes_route_id ON group_routes(route_id);
CREATE INDEX IF NOT EXISTS idx_group_routes_group_id ON group_routes(group_id);

-- DNS mappings table
CREATE TABLE IF NOT EXISTS dns_mappings (
    id TEXT PRIMARY KEY,
    route_id TEXT NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(route_id, name)
);

CREATE INDEX IF NOT EXISTS idx_dns_mappings_route_id ON dns_mappings(route_id);

-- Network default groups junction table
CREATE TABLE IF NOT EXISTS network_default_groups (
    network_id TEXT NOT NULL REFERENCES networks(id) ON DELETE CASCADE,
    group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (network_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_network_default_groups_network_id ON network_default_groups(network_id);
CREATE INDEX IF NOT EXISTS idx_network_default_groups_group_id ON network_default_groups(group_id);

-- Add domain_suffix column to networks table
ALTER TABLE networks ADD COLUMN IF NOT EXISTS domain_suffix TEXT NOT NULL DEFAULT 'internal';
