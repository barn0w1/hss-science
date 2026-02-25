DROP INDEX IF EXISTS idx_refresh_tokens_token;
DROP TABLE IF EXISTS refresh_tokens;

DROP INDEX IF EXISTS idx_access_tokens_subject;
DROP TABLE IF EXISTS access_tokens;

DROP TABLE IF EXISTS auth_codes;
DROP TABLE IF EXISTS auth_requests;

DROP TABLE IF EXISTS clients;

DROP INDEX IF EXISTS idx_federated_identities_user_id;
DROP TABLE IF EXISTS federated_identities;

DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS signing_keys;

-- pgcrypto extension is deliberately NOT dropped here as it might be used by other schemas/databases.