-- 0001_schema.up.sql
-- Core tables for the Accounts (Auth) service.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- users: The canonical internal user record.
CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name TEXT NOT NULL DEFAULT '',
    avatar_url   TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- user_identities: Links external provider identities to internal users.
-- Designed to support multiple providers (Discord, Google, GitHub, etc.).
CREATE TABLE user_identities (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider      TEXT        NOT NULL,  -- e.g. "discord"
    provider_id   TEXT        NOT NULL,  -- external user ID from the provider
    email         TEXT        NOT NULL DEFAULT '',
    display_name  TEXT        NOT NULL DEFAULT '',
    avatar_url    TEXT        NOT NULL DEFAULT '',
    access_token  TEXT        NOT NULL DEFAULT '',
    refresh_token TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (provider, provider_id)
);

CREATE INDEX idx_user_identities_user_id ON user_identities(user_id);

-- auth_codes: Short-lived internal authorization codes for the SSO flow.
CREATE TABLE auth_codes (
    code        TEXT PRIMARY KEY,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri TEXT       NOT NULL DEFAULT '',
    client_state TEXT       NOT NULL DEFAULT '',
    used        BOOLEAN     NOT NULL DEFAULT FALSE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_auth_codes_expires_at ON auth_codes(expires_at);

-- oauth_states: Temporary storage for OAuth state parameters to prevent CSRF.
CREATE TABLE oauth_states (
    state        TEXT PRIMARY KEY,
    provider     TEXT        NOT NULL,
    redirect_uri TEXT        NOT NULL DEFAULT '',
    client_state TEXT        NOT NULL DEFAULT '',
    expires_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_oauth_states_expires_at ON oauth_states(expires_at);
