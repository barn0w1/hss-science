-- Users Table
CREATE TABLE users (
    id          UUID PRIMARY KEY,
    discord_id  VARCHAR(255) NOT NULL UNIQUE,
    name        VARCHAR(255) NOT NULL,
    avatar_url  TEXT,
    role        VARCHAR(50) NOT NULL DEFAULT 'user',

    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

-- For efficient lookup by Discord ID
CREATE INDEX idx_users_discord_id ON users(discord_id);

-- Refresh Tokens Table
CREATE TABLE refresh_tokens (
    token_hash  VARCHAR(255) PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    
    -- NULL = valid, otherwise considered revoked (logged out) at that time
    revoked_at  TIMESTAMPTZ,
    
    -- For audit logs
    user_agent  TEXT,
    ip_address  VARCHAR(45)
);

-- For listing sessions per user and bulk logout
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- For efficient cleanup of expired tokens by a periodic job
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
