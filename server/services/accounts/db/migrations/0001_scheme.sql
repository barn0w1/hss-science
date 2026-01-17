-- --------------------------------------------------------
-- Users Table
-- --------------------------------------------------------
CREATE TABLE users (
    -- アプリ側で生成したUUIDが入るため、DEFAULTは不要
    id          UUID PRIMARY KEY,
    
    -- Discord IDはユニーク制約必須
    discord_id  VARCHAR(255) NOT NULL UNIQUE,
    
    name        VARCHAR(255) NOT NULL,
    avatar_url  TEXT,
    
    -- roleは文字列で保存（将来的にENUM型にする手もあるが、VARCHARの方が変更に強い）
    role        VARCHAR(50) NOT NULL DEFAULT 'user',
    
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

-- Login時の高速化用 (WHERE discord_id = ?)
CREATE INDEX idx_users_discord_id ON users(discord_id);


-- --------------------------------------------------------
-- Refresh Tokens Table
-- --------------------------------------------------------
CREATE TABLE refresh_tokens (
    token_hash  VARCHAR(255) PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    
    user_agent  TEXT,
    ip_address  VARCHAR(45)
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);