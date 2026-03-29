-- API tokens for programmatic access (MCP, CI/CD, scripts, etc.)
CREATE TABLE IF NOT EXISTS api_tokens (
    id           TEXT        NOT NULL PRIMARY KEY,
    user_id      TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    token_hash   TEXT        NOT NULL UNIQUE,  -- SHA-256 hex of the raw token
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ,                  -- NULL = never expires
    last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS api_tokens_user_id_idx ON api_tokens (user_id);
CREATE INDEX IF NOT EXISTS api_tokens_hash_idx    ON api_tokens (token_hash);
