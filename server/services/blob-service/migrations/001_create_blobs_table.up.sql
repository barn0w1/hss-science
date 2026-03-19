CREATE TABLE blobs (
    id           CHAR(64)    PRIMARY KEY,
    size_bytes   BIGINT      NOT NULL,
    content_type TEXT        NOT NULL DEFAULT '',
    r2_key       CHAR(64)    NOT NULL,
    state        TEXT        NOT NULL DEFAULT 'PENDING'
                             CHECK (state IN ('PENDING', 'COMMITTED')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    committed_at TIMESTAMPTZ
);
