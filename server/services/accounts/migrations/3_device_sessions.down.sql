ALTER TABLE auth_requests DROP COLUMN IF EXISTS device_session_id;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS device_session_id;
DROP TABLE IF EXISTS device_sessions;
