package authn

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/barn0w1/hss-science/server/services/identity-service/internal/identity"
)

type ClaimsProvider interface {
	FetchClaims(ctx context.Context, token *oauth2.Token) (*identity.FederatedClaims, error)
}

type Provider struct {
	Name         string
	DisplayName  string
	OAuth2Config *oauth2.Config
	Claims       ClaimsProvider
}

func NewProviders(ctx context.Context, cfg Config) ([]*Provider, error) {
	var providers []*Provider

	callbackURL := cfg.IssuerURL + "/login/callback"

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
