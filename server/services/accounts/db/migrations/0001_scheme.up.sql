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

-- Sessions Table
CREATE TABLE sessions (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    user_agent  TEXT,
    ip_address  VARCHAR(45)
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Auth Codes Table
CREATE TABLE auth_codes (
    code        VARCHAR(255) PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    audience    VARCHAR(100) NOT NULL, -- e.g. "drive"
    redirect_uri TEXT NOT NULL,

    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_auth_codes_expires_at ON auth_codes(expires_at);
