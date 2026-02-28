package oidcprovider

import (
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/barn0w1/hss-science/server/services/accounts/model"
)

type Client struct {
	model *model.Client
}

func NewClient(m *model.Client) *Client {
	return &Client{model: m}
}

func (c *Client) GetID() string                        { return c.model.ID }
func (c *Client) RedirectURIs() []string               { return c.model.RedirectURIs }
func (c *Client) PostLogoutRedirectURIs() []string     { return c.model.PostLogoutRedirectURIs }
func (c *Client) LoginURL(id string) string            { return "/login?authRequestID=" + id }
func (c *Client) DevMode() bool                        { return false }
func (c *Client) IDTokenUserinfoClaimsAssertion() bool { return c.model.IDTokenUserinfoAssertion }

func (c *Client) ApplicationType() op.ApplicationType {
	switch c.model.ApplicationType {
	case "native":
		return op.ApplicationTypeNative
	case "user_agent":
		return op.ApplicationTypeUserAgent
	default:
		return op.ApplicationTypeWeb
	}
}

func (c *Client) AuthMethod() oidc.AuthMethod {
	switch c.model.AuthMethod {
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

func (c *Client) ResponseTypes() []oidc.ResponseType {
	result := make([]oidc.ResponseType, len(c.model.ResponseTypes))
	for i, rt := range c.model.ResponseTypes {
		result[i] = oidc.ResponseType(rt)
	}
	return result
}

func (c *Client) GrantTypes() []oidc.GrantType {
	result := make([]oidc.GrantType, len(c.model.GrantTypes))
	for i, gt := range c.model.GrantTypes {
		result[i] = oidc.GrantType(gt)
	}
	return result
}

func (c *Client) AccessTokenType() op.AccessTokenType {
	if c.model.AccessTokenType == "jwt" {
		return op.AccessTokenTypeJWT
	}
	return op.AccessTokenTypeBearer
}

func (c *Client) IDTokenLifetime() time.Duration {
	return time.Duration(c.model.IDTokenLifetimeSeconds) * time.Second
}

func (c *Client) ClockSkew() time.Duration {
	return time.Duration(c.model.ClockSkewSeconds) * time.Second
}

func (c *Client) IsScopeAllowed(_ string) bool { return false }

func (c *Client) RestrictAdditionalIdTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string { return scopes }
}

func (c *Client) RestrictAdditionalAccessTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string { return scopes }
}
