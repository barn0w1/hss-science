-- WARNING: Rolling back invalidates all active refresh token sessions.
-- Stored hashes cannot be reversed to raw values.
ALTER TABLE refresh_tokens RENAME COLUMN token_hash TO token;
