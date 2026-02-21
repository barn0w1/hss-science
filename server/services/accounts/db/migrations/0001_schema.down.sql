-- 0001_schema.down.sql
-- Reverse migration: drop all Accounts service tables.

DROP TABLE IF EXISTS oauth_states;
DROP TABLE IF EXISTS auth_codes;
DROP TABLE IF EXISTS user_identities;
DROP TABLE IF EXISTS users;
