package oidcrp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Client struct {
	provider  *gooidc.Provider
	oauth2Cfg oauth2.Config
	verifier  *gooidc.IDTokenVerifier
}

func New(ctx context.Context, issuer, clientID, clientSecret, redirectURL string) (*Client, error) {
	provider, err := gooidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	verifier := provider.Verifier(&gooidc.Config{ClientID: clientID})
	return &Client{
		provider: provider,
		oauth2Cfg: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{gooidc.ScopeOpenID, "email", "profile", "offline_access"},
		},
		verifier: verifier,
	}, nil
}

func (c *Client) AuthCodeURL() (url, state, verifier string) {
	state = oauth2.GenerateVerifier()
	verifier = oauth2.GenerateVerifier()
	url = c.oauth2Cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
	return url, state, verifier
}

func (c *Client) Exchange(ctx context.Context, code, verifier string) (*oauth2.Token, *gooidc.IDToken, error) {
	token, err := c.oauth2Cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, nil, fmt.Errorf("token exchange: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, nil, fmt.Errorf("id_token missing from token response")
	}
	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, nil, fmt.Errorf("verify id_token: %w", err)
	}
	return token, idToken, nil
}

func (c *Client) EndSessionURL(idToken, postLogoutRedirectURI string) (string, error) {
	var raw struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := c.provider.Claims(&raw); err != nil {
		return "", fmt.Errorf("provider claims: %w", err)
	}
	if raw.EndSessionEndpoint == "" {
		return "", fmt.Errorf("end_session_endpoint not found in discovery document")
	}
	u := raw.EndSessionEndpoint + "?id_token_hint=" + idToken
	if postLogoutRedirectURI != "" {
		u += "&post_logout_redirect_uri=" + postLogoutRedirectURI
	}
	return u, nil
}

func (c *Client) RefreshToken(ctx context.Context, rawRefreshToken string) (*oauth2.Token, *gooidc.IDToken, error) {
	tokenSrc := c.oauth2Cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: rawRefreshToken})
	tok, err := tokenSrc.Token()
	if err != nil {
		return nil, nil, fmt.Errorf("token refresh: %w", err)
	}
	var idToken *gooidc.IDToken
	if rawIDToken, ok := tok.Extra("id_token").(string); ok && rawIDToken != "" {
		idToken, _ = c.verifier.Verify(ctx, rawIDToken)
	}
	return tok, idToken, nil
}

func ExtractDSID(rawAccessToken string) string {
	parts := strings.Split(rawAccessToken, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		DSID string `json:"dsid"`
	}
	_ = json.Unmarshal(payload, &claims)
	return claims.DSID
}
