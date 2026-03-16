CREATE TABLE users (
    id             TEXT        PRIMARY KEY,
    email          TEXT        NOT NULL DEFAULT '',
    email_verified BOOLEAN     NOT NULL DEFAULT false,
    name           TEXT        NOT NULL DEFAULT '',
    given_name     TEXT        NOT NULL DEFAULT '',
    family_name    TEXT        NOT NULL DEFAULT '',
    picture        TEXT        NOT NULL DEFAULT '',
    local_name    TEXT DEFAULT NULL,
    local_picture TEXT DEFAULT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE federated_identities (
    id                      TEXT        PRIMARY KEY,
    user_id                 TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider                TEXT        NOT NULL,
    provider_subject        TEXT        NOT NULL,
    provider_email          TEXT        NOT NULL DEFAULT '',
    provider_email_verified BOOLEAN     NOT NULL DEFAULT false,
    provider_display_name   TEXT        NOT NULL DEFAULT '',
    provider_given_name     TEXT        NOT NULL DEFAULT '',
    provider_family_name    TEXT        NOT NULL DEFAULT '',
    provider_picture_url    TEXT        NOT NULL DEFAULT '',
    last_login_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(provider, provider_subject)
);

CREATE INDEX federated_identities_user_id_idx
    ON federated_identities (user_id);

CREATE TABLE clients (
    id                          TEXT        PRIMARY KEY,
    secret_hash                 TEXT        NOT NULL DEFAULT '',
    redirect_uris               TEXT[]      NOT NULL DEFAULT '{}',
    post_logout_redirect_uris   TEXT[]      NOT NULL DEFAULT '{}',
    application_type            TEXT        NOT NULL DEFAULT 'web',
    auth_method                 TEXT        NOT NULL DEFAULT 'client_secret_basic',
    response_types              TEXT[]      NOT NULL DEFAULT '{}',
    grant_types                 TEXT[]      NOT NULL DEFAULT '{}',
    access_token_type           TEXT        NOT NULL DEFAULT 'jwt',
    allowed_scopes              TEXT[]      NOT NULL DEFAULT '{}',
    id_token_lifetime_seconds   INTEGER     NOT NULL DEFAULT 3600,
    clock_skew_seconds          INTEGER     NOT NULL DEFAULT 0,
    id_token_userinfo_assertion BOOLEAN     NOT NULL DEFAULT false,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auth_requests (
    id                    TEXT        PRIMARY KEY,
    client_id             TEXT        NOT NULL,
    redirect_uri          TEXT        NOT NULL,
    state                 TEXT,
    nonce                 TEXT,
    scopes                TEXT[]      NOT NULL DEFAULT '{}',
    response_type         TEXT        NOT NULL,
    response_mode         TEXT,
    code_challenge        TEXT,
    code_challenge_method TEXT,
    prompt                TEXT[],
    max_age               INTEGER,
    login_hint            TEXT,
    user_id               TEXT,
    auth_time             TIMESTAMPTZ,
    amr                   TEXT[],
    is_done               BOOLEAN     NOT NULL DEFAULT false,
    code                  TEXT,
    device_session_id     TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX auth_requests_code_idx
    ON auth_requests (code) WHERE code IS NOT NULL;

CREATE INDEX auth_requests_created_at_idx
    ON auth_requests (created_at);

CREATE TABLE tokens (
    id               TEXT        PRIMARY KEY,
    client_id        TEXT        NOT NULL,
    subject          TEXT        NOT NULL,
    audience         TEXT[]      NOT NULL DEFAULT '{}',
    scopes           TEXT[]      NOT NULL DEFAULT '{}',
    expiration       TIMESTAMPTZ NOT NULL,
    refresh_token_id TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tokens_subject_client_idx
    ON tokens (subject, client_id);

CREATE TABLE refresh_tokens (
    id               TEXT        PRIMARY KEY,
    token_hash       TEXT        NOT NULL UNIQUE,
    client_id        TEXT        NOT NULL,
    user_id          TEXT        NOT NULL REFERENCES users(id),
    audience         TEXT[]      NOT NULL DEFAULT '{}',
    scopes           TEXT[]      NOT NULL DEFAULT '{}',
    auth_time        TIMESTAMPTZ NOT NULL,
    amr              TEXT[]      NOT NULL DEFAULT '{}',
    access_token_id  TEXT,
    device_session_id TEXT,
    expiration       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tokens_expiration_idx ON tokens (expiration);
CREATE INDEX refresh_tokens_expiration_idx ON refresh_tokens (expiration);

CREATE TABLE device_sessions (
    id            TEXT        PRIMARY KEY,
    user_id       TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_agent    TEXT        NOT NULL DEFAULT '',
    ip_address    TEXT        NOT NULL DEFAULT '',
    device_name   TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMPTZ
);

CREATE INDEX device_sessions_user_id_idx ON device_sessions (user_id);
CREATE INDEX device_sessions_revoked_idx ON device_sessions (user_id) WHERE revoked_at IS NULL;

ALTER TABLE refresh_tokens
    ADD CONSTRAINT refresh_tokens_device_session_fk
    FOREIGN KEY (device_session_id) REFERENCES device_sessions(id) ON DELETE SET NULL;

CREATE INDEX refresh_tokens_device_session_idx ON refresh_tokens (device_session_id)
    WHERE device_session_id IS NOT NULL;

ALTER TABLE auth_requests
    ADD CONSTRAINT auth_requests_device_session_fk
    FOREIGN KEY (device_session_id) REFERENCES device_sessions(id) ON DELETE SET NULL;
