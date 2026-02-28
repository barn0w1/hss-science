INSERT INTO clients (id, secret_hash, redirect_uris, post_logout_redirect_uris, response_types, grant_types, access_token_type)
VALUES (
    'myaccount-bff',
    '$2a$10$PLACEHOLDER',
    '{"https://myaccount.hss-science.org/api/v1/auth/callback"}',
    '{"https://myaccount.hss-science.org/"}',
    '{"code"}',
    '{"authorization_code","refresh_token"}',
    'jwt'
);
