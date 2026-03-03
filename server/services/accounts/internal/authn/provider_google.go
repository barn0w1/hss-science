package authn

import (
	"context"
	"fmt"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
)

type googleClaimsProvider struct {
	verifier *gooidc.IDTokenVerifier
}

func (g *googleClaimsProvider) FetchClaims(ctx context.Context, token *oauth2.Token) (*identity.FederatedClaims, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}
	idToken, err := g.verifier.Verify(ctx, rawIDToken)
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
	return &identity.FederatedClaims{
		Subject:       idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Picture:       claims.Picture,
	}, nil
}

func newGoogleProvider(ctx context.Context, clientID, clientSecret, callbackURL string) (*Provider, error) {
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

	return &Provider{
		Name:         "google",
		DisplayName:  "Sign in with Google",
		OAuth2Config: oauth2Cfg,
		Claims:       &googleClaimsProvider{verifier: verifier},
	}, nil
}
