package authn

import (
	"context"
	"fmt"
	"net/http"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// GoogleProvider implements AuthnProvider for Google OIDC.
type GoogleProvider struct {
	oauth2Config *oauth2.Config
	verifier     *gooidc.IDTokenVerifier
}

// NewGoogleProvider creates a new Google OIDC authentication provider.
// It performs OIDC discovery against Google's well-known endpoint.
func NewGoogleProvider(ctx context.Context, clientID, clientSecret, redirectURI string) (*GoogleProvider, error) {
	provider, err := gooidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("google oidc discovery: %w", err)
	}

	return &GoogleProvider{
		oauth2Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURI,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&gooidc.Config{
			ClientID: clientID,
		}),
	}, nil
}

func (g *GoogleProvider) Name() string {
	return "google"
}

// AuthURL returns the Google authorization URL with the given state parameter.
func (g *GoogleProvider) AuthURL(state string) string {
	return g.oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// HandleCallback exchanges the authorization code for tokens, verifies the
// Google ID token, and extracts the user's identity claims.
func (g *GoogleProvider) HandleCallback(ctx context.Context, r *http.Request) (*Identity, error) {
	code := r.URL.Query().Get("code")
	if code == "" {
		return nil, fmt.Errorf("missing authorization code")
	}

	// Check for upstream errors
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		desc := r.URL.Query().Get("error_description")
		return nil, fmt.Errorf("upstream error: %s: %s", errMsg, desc)
	}

	// Exchange authorization code for tokens
	oauth2Token, err := g.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange: %w", err)
	}

	// Extract and verify the ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := g.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("id_token verification: %w", err)
	}

	// Extract claims
	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
		Locale        string `json:"locale"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parse id_token claims: %w", err)
	}

	return &Identity{
		Provider:      "google",
		ExternalSub:   claims.Sub,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Picture:       claims.Picture,
		Locale:        claims.Locale,
	}, nil
}
