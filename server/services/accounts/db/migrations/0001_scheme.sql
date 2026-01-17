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

-- (WHERE discord_id = ?)
CREATE INDEX idx_users_discord_id ON users(discord_id);

-- Refresh Tokens Table
CREATE TABLE refresh_tokens (
    token_hash  VARCHAR(255) PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    
    user_agent  TEXT,
    ip_address  VARCHAR(45)
);

-- (WHERE user_id = ?)
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);