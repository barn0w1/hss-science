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
    ADD COLUMN device_session_id TEXT REFERENCES device_sessions(id) ON DELETE SET NULL;

CREATE INDEX refresh_tokens_device_session_idx ON refresh_tokens (device_session_id)
    WHERE device_session_id IS NOT NULL;

ALTER TABLE auth_requests
    ADD COLUMN device_session_id TEXT REFERENCES device_sessions(id) ON DELETE SET NULL;
