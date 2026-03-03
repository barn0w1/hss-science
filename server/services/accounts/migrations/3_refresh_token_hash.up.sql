ALTER TABLE refresh_tokens RENAME COLUMN token TO token_hash;

UPDATE refresh_tokens
SET token_hash = encode(sha256(token_hash::bytea), 'hex');
