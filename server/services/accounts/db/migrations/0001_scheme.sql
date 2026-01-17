-- UUID生成拡張機能を有効化 (Postgres特有)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- --------------------------------------------------------
-- Users Table
-- --------------------------------------------------------
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    discord_id  VARCHAR(255) NOT NULL UNIQUE, -- Discord IDで検索・特定する
    name        VARCHAR(255) NOT NULL,
    avatar_url  TEXT,
    role        VARCHAR(50) NOT NULL DEFAULT 'member',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for faster lookup during login
CREATE INDEX idx_users_discord_id ON users(discord_id);


-- --------------------------------------------------------
-- Refresh Tokens Table (For SSO Session Management)
-- --------------------------------------------------------
-- AccessTokenが切れたら、これを使って再発行する。
-- ユーザーがログアウトしたら、この行を削除する。
CREATE TABLE refresh_tokens (
    token_hash  VARCHAR(255) PRIMARY KEY, -- トークンそのものではなくハッシュ化して保存がベストだが、部活レベルなら生トークンでも許容範囲
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- オプション: どのデバイスからのログインか記録しておくと、
    -- 「不審なログイン」を見つけたり、「全デバイスからログアウト」機能が作れる
    user_agent  TEXT, 
    ip_address  VARCHAR(45)
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);