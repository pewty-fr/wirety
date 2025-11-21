-- Migration: IPAM persistence tables
CREATE TABLE IF NOT EXISTS ipam_prefixes (
    cidr TEXT PRIMARY KEY,
    parent_cidr TEXT REFERENCES ipam_prefixes(cidr) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ipam_allocated_ips (
    ip TEXT PRIMARY KEY,
    prefix_cidr TEXT NOT NULL REFERENCES ipam_prefixes(cidr) ON DELETE CASCADE,
    allocated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS ipam_prefix_parent_idx ON ipam_prefixes(parent_cidr);
CREATE INDEX IF NOT EXISTS ipam_allocated_prefix_idx ON ipam_allocated_ips(prefix_cidr);
