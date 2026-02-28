package login

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"

	"github.com/barn0w1/hss-science/server/services/accounts/config"
)

type UpstreamProvider struct {
	Name         string
	DisplayName  string
	OAuth2Config *oauth2.Config
	OIDCVerifier *gooidc.IDTokenVerifier
	UserInfoFunc func(ctx context.Context, token *oauth2.Token) (*UpstreamClaims, error)
}

type UpstreamClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	GivenName     string
	FamilyName    string
	Picture       string
}

func NewUpstreamProviders(ctx context.Context, cfg *config.Config) ([]*UpstreamProvider, error) {
	var providers []*UpstreamProvider

	callbackURL := cfg.Issuer + "/login/callback"

	if cfg.GoogleClientID != "" {
		p, err := newGoogleProvider(ctx, cfg.GoogleClientID, cfg.GoogleClientSecret, callbackURL)
		if err != nil {
			return nil, fmt.Errorf("google provider: %w", err)
		}
		providers = append(providers, p)
	}

	if cfg.GitHubClientID != "" {
		providers = append(providers, newGitHubProvider(cfg.GitHubClientID, cfg.GitHubClientSecret, callbackURL))
	}

	return providers, nil
}

func newGoogleProvider(ctx context.Context, clientID, clientSecret, callbackURL string) (*UpstreamProvider, error) {
	oidcProvider, err := gooidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	verifier := oidcProvider.Verifier(&gooidc.Config{ClientID: clientID})

	oauth2Cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  callbackURL,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"openid", "email", "profile"},
	}

	return &UpstreamProvider{
		Name:         "google",
		DisplayName:  "Sign in with Google",
		OAuth2Config: oauth2Cfg,
		OIDCVerifier: verifier,
		UserInfoFunc: func(ctx context.Context, token *oauth2.Token) (*UpstreamClaims, error) {
			rawIDToken, ok := token.Extra("id_token").(string)
			if !ok {
				return nil, fmt.Errorf("no id_token in token response")
			}
			idToken, err := verifier.Verify(ctx, rawIDToken)
			if err != nil {
				return nil, fmt.Errorf("verifying id_token: %w", err)
			}
			var claims struct {
				Email         string `json:"email"`
				EmailVerified bool   `json:"email_verified"`
				Name          string `json:"name"`
				GivenName     string `json:"given_name"`
				FamilyName    string `json:"family_name"`
				Picture       string `json:"picture"`
			}
			if err := idToken.Claims(&claims); err != nil {
				return nil, fmt.Errorf("parsing id_token claims: %w", err)
			}
			return &UpstreamClaims{
				Subject:       idToken.Subject,
				Email:         claims.Email,
				EmailVerified: claims.EmailVerified,
				Name:          claims.Name,
				GivenName:     claims.GivenName,
				FamilyName:    claims.FamilyName,
				Picture:       claims.Picture,
			}, nil
		},
	}, nil
}

func newGitHubProvider(clientID, clientSecret, callbackURL string) *UpstreamProvider {
	oauth2Cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  callbackURL,
		Endpoint:     github.Endpoint,
		Scopes:       []string{"read:user", "user:email"},
	}

	return &UpstreamProvider{
		Name:         "github",
		DisplayName:  "Sign in with GitHub",
		OAuth2Config: oauth2Cfg,
		OIDCVerifier: nil,
		UserInfoFunc: func(ctx context.Context, token *oauth2.Token) (*UpstreamClaims, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
			if err != nil {
				return nil, err
			}
			token.SetAuthHeader(req)
			httpClient := &http.Client{Timeout: 10 * time.Second}
			resp, err := httpClient.Do(req) //nolint:gosec // URL is a hardcoded constant
			if err != nil {
				return nil, fmt.Errorf("github user API: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return nil, fmt.Errorf("github user API returned %d: %s", resp.StatusCode, body)
			}
			var ghUser struct {
				ID        int64  `json:"id"`
				Email     string `json:"email"`
				Name      string `json:"name"`
				AvatarURL string `json:"avatar_url"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
				return nil, fmt.Errorf("decoding github user: %w", err)
			}
			return &UpstreamClaims{
				Subject: strconv.FormatInt(ghUser.ID, 10),
				Email:   ghUser.Email,
				Name:    ghUser.Name,
				Picture: ghUser.AvatarURL,
			}, nil
		},
	}
}
