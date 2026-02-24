package drive

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// TokenExchanger abstracts OIDC operations for testability.
type TokenExchanger interface {
	AuthURL(state, codeChallenge string) string
	Exchange(ctx context.Context, code, codeVerifier string) (*IDClaims, error)
}

// IDClaims represents the relevant claims from a Google ID token.
type IDClaims struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// OIDCProvider wraps the go-oidc provider and oauth2 config.
type OIDCProvider struct {
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

// Compile-time check.
var _ TokenExchanger = (*OIDCProvider)(nil)

// NewOIDCProvider performs OIDC discovery against Google and returns a configured provider.
func NewOIDCProvider(ctx context.Context, cfg *Config) (*OIDCProvider, error) {
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}

	return &OIDCProvider{
		oauth2Config: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.GoogleClientID}),
	}, nil
}

// AuthURL generates the Google authorization URL with PKCE parameters.
func (p *OIDCProvider) AuthURL(state, codeChallenge string) string {
	return p.oauth2Config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// Exchange trades an authorization code for a verified ID token using PKCE.
func (p *OIDCProvider) Exchange(ctx context.Context, code, codeVerifier string) (*IDClaims, error) {
	token, err := p.oauth2Config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("code exchange: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}

	var claims IDClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parse claims: %w", err)
	}
	return &claims, nil
}

// PKCEState is the data stored in the encrypted cookie between /login and /callback.
type PKCEState struct {
	State        string `json:"state"`
	CodeVerifier string `json:"code_verifier"`
}

// GeneratePKCE creates a code_verifier and its S256 code_challenge.
func GeneratePKCE() (codeVerifier, codeChallenge string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}

	codeVerifier = base64.RawURLEncoding.EncodeToString(buf)
	h := sha256.Sum256([]byte(codeVerifier))
	codeChallenge = base64.RawURLEncoding.EncodeToString(h[:])

	return codeVerifier, codeChallenge, nil
}

// GenerateState creates a cryptographically random state string.
func GenerateState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
