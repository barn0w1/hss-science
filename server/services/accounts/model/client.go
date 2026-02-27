package model

import "time"

type Client struct {
	ID                       string    `db:"id"`
	SecretHash               string    `db:"secret_hash"`
	RedirectURIs             []string  `db:"redirect_uris"`
	PostLogoutRedirectURIs   []string  `db:"post_logout_redirect_uris"`
	ApplicationType          string    `db:"application_type"`
	AuthMethod               string    `db:"auth_method"`
	ResponseTypes            []string  `db:"response_types"`
	GrantTypes               []string  `db:"grant_types"`
	AccessTokenType          string    `db:"access_token_type"`
	IDTokenLifetimeSeconds   int       `db:"id_token_lifetime_seconds"`
	ClockSkewSeconds         int       `db:"clock_skew_seconds"`
	IDTokenUserinfoAssertion bool      `db:"id_token_userinfo_assertion"`
	CreatedAt                time.Time `db:"created_at"`
	UpdatedAt                time.Time `db:"updated_at"`
}
