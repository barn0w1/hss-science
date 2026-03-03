package adapter

import (
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	oidcdom "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
)

type ClientAdapter struct {
	domain *oidcdom.Client
}

func NewClientAdapter(c *oidcdom.Client) *ClientAdapter {
	return &ClientAdapter{domain: c}
}

func (c *ClientAdapter) GetID() string                    { return c.domain.ID }
func (c *ClientAdapter) RedirectURIs() []string           { return c.domain.RedirectURIs }
func (c *ClientAdapter) PostLogoutRedirectURIs() []string { return c.domain.PostLogoutRedirectURIs }
func (c *ClientAdapter) LoginURL(id string) string        { return "/login?authRequestID=" + id }
func (c *ClientAdapter) DevMode() bool                    { return false }
func (c *ClientAdapter) IDTokenUserinfoClaimsAssertion() bool {
	return c.domain.IDTokenUserinfoAssertion
}

func (c *ClientAdapter) ApplicationType() op.ApplicationType {
	switch c.domain.ApplicationType {
	case "native":
		return op.ApplicationTypeNative
	case "user_agent":
		return op.ApplicationTypeUserAgent
	default:
		return op.ApplicationTypeWeb
	}
}

func (c *ClientAdapter) AuthMethod() oidc.AuthMethod {
	switch c.domain.AuthMethod {
	case "client_secret_post":
		return oidc.AuthMethodPost
	case "none":
		return oidc.AuthMethodNone
	case "private_key_jwt":
		return oidc.AuthMethodPrivateKeyJWT
	default:
		return oidc.AuthMethodBasic
	}
}

func (c *ClientAdapter) ResponseTypes() []oidc.ResponseType {
	result := make([]oidc.ResponseType, len(c.domain.ResponseTypes))
	for i, rt := range c.domain.ResponseTypes {
		result[i] = oidc.ResponseType(rt)
	}
	return result
}

func (c *ClientAdapter) GrantTypes() []oidc.GrantType {
	result := make([]oidc.GrantType, len(c.domain.GrantTypes))
	for i, gt := range c.domain.GrantTypes {
		result[i] = oidc.GrantType(gt)
	}
	return result
}

func (c *ClientAdapter) AccessTokenType() op.AccessTokenType {
	if c.domain.AccessTokenType == "jwt" {
		return op.AccessTokenTypeJWT
	}
	return op.AccessTokenTypeBearer
}

func (c *ClientAdapter) IDTokenLifetime() time.Duration {
	return time.Duration(c.domain.IDTokenLifetimeSeconds) * time.Second
}

func (c *ClientAdapter) ClockSkew() time.Duration {
	return time.Duration(c.domain.ClockSkewSeconds) * time.Second
}

func (c *ClientAdapter) IsScopeAllowed(scope string) bool {
	if len(c.domain.AllowedScopes) == 0 {
		return false
	}
	for _, s := range c.domain.AllowedScopes {
		if s == scope {
			return true
		}
	}
	return false
}

func (c *ClientAdapter) RestrictAdditionalIdTokenScopes() func(scopes []string) []string {
	return c.filterScopes
}

func (c *ClientAdapter) RestrictAdditionalAccessTokenScopes() func(scopes []string) []string {
	return c.filterScopes
}

func (c *ClientAdapter) filterScopes(scopes []string) []string {
	if len(c.domain.AllowedScopes) == 0 {
		return scopes
	}
	allowed := make(map[string]struct{}, len(c.domain.AllowedScopes))
	for _, s := range c.domain.AllowedScopes {
		allowed[s] = struct{}{}
	}
	filtered := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if _, ok := allowed[s]; ok {
			filtered = append(filtered, s)
		}
	}
	return filtered
}
