CREATE TABLE users (
    id           TEXT PRIMARY KEY, -- Discord ID (Snowflake)
    username     TEXT NOT NULL,
    avatar_url   TEXT NOT NULL,
    role         TEXT NOT NULL DEFAULT 'member', -- guest, member, admin
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE sessions (
    refresh_token TEXT PRIMARY KEY, -- Opaque Token
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_agent    TEXT NOT NULL,
    client_ip     TEXT NOT NULL,
    expires_at    TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 検索用インデックス
CREATE INDEX idx_sessions_user_id ON sessions(user_id);