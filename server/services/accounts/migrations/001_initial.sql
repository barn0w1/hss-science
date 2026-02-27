CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT NOT NULL,
    email_verified  BOOLEAN NOT NULL DEFAULT false,
    name            TEXT,
    given_name      TEXT,
    family_name     TEXT,
    picture         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE federated_identities (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL,
    provider_subject  TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_subject)
);

CREATE TABLE clients (
    id                          TEXT PRIMARY KEY,
    secret_hash                 TEXT NOT NULL DEFAULT '',
    redirect_uris               TEXT[] NOT NULL,
    post_logout_redirect_uris   TEXT[] NOT NULL DEFAULT '{}',
    application_type            TEXT NOT NULL DEFAULT 'web',
    auth_method                 TEXT NOT NULL DEFAULT 'client_secret_basic',
    response_types              TEXT[] NOT NULL,
    grant_types                 TEXT[] NOT NULL,
    access_token_type           TEXT NOT NULL DEFAULT 'jwt',
    id_token_lifetime_seconds   INTEGER NOT NULL DEFAULT 3600,
    clock_skew_seconds          INTEGER NOT NULL DEFAULT 0,
    id_token_userinfo_assertion BOOLEAN NOT NULL DEFAULT false,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auth_requests (
    id                    UUID PRIMARY KEY,
    client_id             TEXT NOT NULL,
    redirect_uri          TEXT NOT NULL,
    state                 TEXT,
    nonce                 TEXT,
    scopes                TEXT[],
    response_type         TEXT NOT NULL,
    response_mode         TEXT,
    code_challenge        TEXT,
    code_challenge_method TEXT,
    prompt                TEXT[],
    max_age               INTEGER,
    login_hint            TEXT,
    user_id               UUID,
    auth_time             TIMESTAMPTZ,
    amr                   TEXT[],
    is_done               BOOLEAN NOT NULL DEFAULT false,
    code                  TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX auth_requests_code_idx ON auth_requests (code) WHERE code IS NOT NULL;

CREATE TABLE tokens (
    id               UUID PRIMARY KEY,
    client_id        TEXT NOT NULL,
    subject          TEXT NOT NULL,
    audience         TEXT[],
    scopes           TEXT[],
    expiration       TIMESTAMPTZ NOT NULL,
    refresh_token_id UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_tokens (
    id               UUID PRIMARY KEY,
    token            TEXT NOT NULL UNIQUE,
    client_id        TEXT NOT NULL,
    user_id          UUID NOT NULL REFERENCES users(id),
    audience         TEXT[],
    scopes           TEXT[],
    auth_time        TIMESTAMPTZ NOT NULL,
    amr              TEXT[],
    access_token_id  UUID,
    expiration       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
