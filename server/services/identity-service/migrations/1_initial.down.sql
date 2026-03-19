ALTER TABLE auth_requests DROP CONSTRAINT IF EXISTS auth_requests_device_session_fk;
ALTER TABLE refresh_tokens DROP CONSTRAINT IF EXISTS refresh_tokens_device_session_fk;
DROP INDEX IF EXISTS refresh_tokens_device_session_idx;
DROP TABLE IF EXISTS device_sessions;

ALTER TABLE users
    DROP COLUMN IF EXISTS local_name,
    DROP COLUMN IF EXISTS local_picture;

DROP TABLE IF EXISTS refresh_tokens;
DROP INDEX IF EXISTS refresh_tokens_expiration_idx;
DROP INDEX IF EXISTS tokens_expiration_idx;
DROP INDEX IF EXISTS tokens_subject_client_idx;
DROP TABLE IF EXISTS tokens;
DROP INDEX IF EXISTS auth_requests_created_at_idx;
DROP INDEX IF EXISTS auth_requests_code_idx;
DROP TABLE IF EXISTS auth_requests;
DROP TABLE IF EXISTS clients;
DROP INDEX IF EXISTS federated_identities_user_id_idx;
DROP TABLE IF EXISTS federated_identities;
DROP TABLE IF EXISTS users;
