-- Users Table
CREATE TABLE users (
    id          UUID PRIMARY KEY,
    discord_id  VARCHAR(255) NOT NULL UNIQUE,
    name        VARCHAR(255) NOT NULL,
    avatar_url  TEXT,
    role        VARCHAR(50) NOT NULL DEFAULT 'user',

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT users_role_check CHECK (role IN ('system_admin', 'moderator', 'user'))
);


-- For efficient lookup by Discord ID
CREATE INDEX idx_users_discord_id ON users(discord_id);

-- Sessions Table
CREATE TABLE sessions (
    token_hash  CHAR(64) PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at  TIMESTAMPTZ,

    user_agent  TEXT,
    ip_address  VARCHAR(45)
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_active_user_id ON sessions(user_id) WHERE revoked_at IS NULL;

-- Auth Codes Table
CREATE TABLE auth_codes (
    code_hash   CHAR(64) PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    audience    VARCHAR(100) NOT NULL, -- e.g. "drive"
    redirect_uri TEXT NOT NULL,

    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_auth_codes_user_id ON auth_codes(user_id);
CREATE INDEX idx_auth_codes_audience ON auth_codes(audience);
CREATE INDEX idx_auth_codes_expires_at ON auth_codes(expires_at);
