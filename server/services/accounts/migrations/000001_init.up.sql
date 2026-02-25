-- accounts-idp database schema
-- Applied externally (not by the Go application)

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Internal user identities
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT NOT NULL UNIQUE,
    email_verified  BOOLEAN NOT NULL DEFAULT false,
    given_name      TEXT NOT NULL DEFAULT '',
    family_name     TEXT NOT NULL DEFAULT '',
    picture         TEXT NOT NULL DEFAULT '',
    locale          TEXT NOT NULL DEFAULT 'en',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- External identity mappings (e.g. Google sub -> internal user)
CREATE TABLE federated_identities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        TEXT NOT NULL,
    external_sub    TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, external_sub)
);

CREATE INDEX idx_federated_identities_user_id ON federated_identities (user_id);

-- OIDC clients (downstream relying parties)
CREATE TABLE clients (
    id                          TEXT PRIMARY KEY,
    secret_hash                 TEXT,
    application_type            TEXT NOT NULL DEFAULT 'web',
    auth_method                 TEXT NOT NULL DEFAULT 'none',
    redirect_uris               TEXT[] NOT NULL DEFAULT '{}',
    post_logout_redirect_uris   TEXT[] NOT NULL DEFAULT '{}',
    response_types              TEXT[] NOT NULL DEFAULT '{code}',
    grant_types                 TEXT[] NOT NULL DEFAULT '{authorization_code}',
    access_token_type           TEXT NOT NULL DEFAULT 'bearer',
    id_token_userinfo_assertion BOOLEAN NOT NULL DEFAULT false,
    clock_skew_seconds          INTEGER NOT NULL DEFAULT 0,
    is_service_account          BOOLEAN NOT NULL DEFAULT false,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Pending authorization requests
CREATE TABLE auth_requests (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id               TEXT NOT NULL,
    redirect_uri            TEXT NOT NULL,
    state                   TEXT NOT NULL DEFAULT '',
    nonce                   TEXT NOT NULL DEFAULT '',
    scopes                  TEXT[] NOT NULL,
    response_type           TEXT NOT NULL,
    response_mode           TEXT NOT NULL DEFAULT '',
    code_challenge          TEXT NOT NULL DEFAULT '',
    code_challenge_method   TEXT NOT NULL DEFAULT '',
    prompt                  TEXT[] NOT NULL DEFAULT '{}',
    login_hint              TEXT NOT NULL DEFAULT '',
    max_age_seconds         INTEGER,
    user_id                 UUID,
    done                    BOOLEAN NOT NULL DEFAULT false,
    auth_time               TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Authorization codes (short-lived, 1:1 with auth_requests)
CREATE TABLE auth_codes (
    code                TEXT PRIMARY KEY,
    auth_request_id     UUID NOT NULL REFERENCES auth_requests(id) ON DELETE CASCADE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Access tokens
CREATE TABLE access_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id       TEXT NOT NULL,
    subject         TEXT NOT NULL,
    audience        TEXT[] NOT NULL,
    scopes          TEXT[] NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_access_tokens_subject ON access_tokens (subject);

-- Refresh tokens (with rotation support)
CREATE TABLE refresh_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token           TEXT NOT NULL UNIQUE,
    client_id       TEXT NOT NULL,
    user_id         UUID NOT NULL REFERENCES users(id),
    audience        TEXT[] NOT NULL,
    scopes          TEXT[] NOT NULL,
    auth_time       TIMESTAMPTZ NOT NULL,
    amr             TEXT[] NOT NULL DEFAULT '{}',
    access_token_id UUID,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_token ON refresh_tokens (token);

-- RSA signing keys (persisted for multi-instance deployments)
CREATE TABLE signing_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    algorithm       TEXT NOT NULL DEFAULT 'RS256',
    private_key_pem BYTEA NOT NULL,
    public_key_pem  BYTEA NOT NULL,
    active          BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ
);
