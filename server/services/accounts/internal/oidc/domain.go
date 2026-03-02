package oidc

import "time"

type AuthRequest struct {
	ID                  string    `db:"id"`
	ClientID            string    `db:"client_id"`
	RedirectURI         string    `db:"redirect_uri"`
	State               string    `db:"state"`
	Nonce               string    `db:"nonce"`
	Scopes              []string  `db:"scopes"`
	ResponseType        string    `db:"response_type"`
	ResponseMode        string    `db:"response_mode"`
	CodeChallenge       string    `db:"code_challenge"`
	CodeChallengeMethod string    `db:"code_challenge_method"`
	Prompt              []string  `db:"prompt"`
	MaxAge              *int64    `db:"max_age"`
	LoginHint           string    `db:"login_hint"`
	UserID              string    `db:"user_id"`
	AuthTime            time.Time `db:"auth_time"`
	AMR                 []string  `db:"amr"`
	IsDone              bool      `db:"is_done"`
	Code                string    `db:"code"`
	CreatedAt           time.Time `db:"created_at"`
}

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

type Token struct {
	ID             string    `db:"id"`
	ClientID       string    `db:"client_id"`
	Subject        string    `db:"subject"`
	Audience       []string  `db:"audience"`
	Scopes         []string  `db:"scopes"`
	Expiration     time.Time `db:"expiration"`
	RefreshTokenID string    `db:"refresh_token_id"`
	CreatedAt      time.Time `db:"created_at"`
}

type RefreshToken struct {
	ID            string    `db:"id"`
	Token         string    `db:"token"`
	ClientID      string    `db:"client_id"`
	UserID        string    `db:"user_id"`
	Audience      []string  `db:"audience"`
	Scopes        []string  `db:"scopes"`
	AuthTime      time.Time `db:"auth_time"`
	AMR           []string  `db:"amr"`
	AccessTokenID string    `db:"access_token_id"`
	Expiration    time.Time `db:"expiration"`
	CreatedAt     time.Time `db:"created_at"`
}
