-- WARNING: The secret_hash below is a PLACEHOLDER.
-- Before first deployment, generate a real bcrypt hash:
--   htpasswd -nbBC 10 "" 'your-secret' | cut -d: -f2
-- or in Go:
--   hash, _ := bcrypt.GenerateFromPassword([]byte("your-secret"), bcrypt.DefaultCost)
-- Replace '$2a$10$PLACEHOLDER' with the generated hash.

INSERT INTO clients (id, secret_hash, redirect_uris, post_logout_redirect_uris, response_types, grant_types, access_token_type, allowed_scopes)
VALUES (
    'myaccount-bff',
    '$2a$10$PLACEHOLDER',
    '{"https://myaccount.hss-science.org/api/v1/auth/callback"}',
    '{"https://myaccount.hss-science.org/"}',
    '{"code"}',
    '{"authorization_code","refresh_token"}',
    'jwt',
    '{"openid","email","profile","offline_access"}'
);
