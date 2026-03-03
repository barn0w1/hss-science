package oidc

import "time"

type AuthRequest struct {
	ID                  string
	ClientID            string
	RedirectURI         string
	State               string
	Nonce               string
	Scopes              []string
	ResponseType        string
	ResponseMode        string
	CodeChallenge       string
	CodeChallengeMethod string
	Prompt              []string
	MaxAge              *int64
	LoginHint           string
	UserID              string
	AuthTime            time.Time
	AMR                 []string
	IsDone              bool
	Code                string
	CreatedAt           time.Time
}

type Client struct {
	ID                       string
	SecretHash               string
	RedirectURIs             []string
	PostLogoutRedirectURIs   []string
	ApplicationType          string
	AuthMethod               string
	ResponseTypes            []string
	GrantTypes               []string
	AccessTokenType          string
	AllowedScopes            []string
	IDTokenLifetimeSeconds   int
	ClockSkewSeconds         int
	IDTokenUserinfoAssertion bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type Token struct {
	ID             string
	ClientID       string
	Subject        string
	Audience       []string
	Scopes         []string
	Expiration     time.Time
	RefreshTokenID string
	CreatedAt      time.Time
}

type RefreshToken struct {
	ID            string
	Token         string
	ClientID      string
	UserID        string
	Audience      []string
	Scopes        []string
	AuthTime      time.Time
	AMR           []string
	AccessTokenID string
	Expiration    time.Time
	CreatedAt     time.Time
}
