package storage

import (
	"time"

	"github.com/lib/pq"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// allowedAdditionalScopes defines which non-standard OIDC scopes are permitted
// in access tokens and ID tokens. The zitadel framework handles openid, phone,
// and address internally; this filter applies to everything else.
var allowedAdditionalScopes = map[string]bool{
	oidc.ScopeEmail:         true,
	oidc.ScopeProfile:       true,
	oidc.ScopeOfflineAccess: true,
}

func filterAllowedScopes(scopes []string) []string {
	filtered := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if allowedAdditionalScopes[s] {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// Client represents an OAuth2/OIDC client stored in PostgreSQL.
// It implements the op.Client interface.
type Client struct {
	ID                        string         `db:"id"`
	SecretHash                *string        `db:"secret_hash"`
	ApplicationTypeStr        string         `db:"application_type"`
	AuthMethodStr             string         `db:"auth_method"`
	RedirectURIList           pq.StringArray `db:"redirect_uris"`
	PostLogoutRedirectURIList pq.StringArray `db:"post_logout_redirect_uris"`
	ResponseTypeList          pq.StringArray `db:"response_types"`
	GrantTypeList             pq.StringArray `db:"grant_types"`
	AccessTokenTypeStr        string         `db:"access_token_type"`
	IDTokenUserinfoAssertion  bool           `db:"id_token_userinfo_assertion"`
	ClockSkewSeconds          int            `db:"clock_skew_seconds"`
	IsServiceAccount          bool           `db:"is_service_account"`
	CreatedAt                 time.Time      `db:"created_at"`

	// idTokenTTL is set by the storage layer from global configuration.
	// It is not stored in the database.
	idTokenTTL time.Duration `db:"-"`
}

func (c *Client) GetID() string {
	return c.ID
}

func (c *Client) RedirectURIs() []string {
	return []string(c.RedirectURIList)
}

func (c *Client) PostLogoutRedirectURIs() []string {
	return []string(c.PostLogoutRedirectURIList)
}

func (c *Client) ApplicationType() op.ApplicationType {
	switch c.ApplicationTypeStr {
	case "native":
		return op.ApplicationTypeNative
	case "user_agent":
		return op.ApplicationTypeUserAgent
	default:
		return op.ApplicationTypeWeb
	}
}

func (c *Client) AuthMethod() oidc.AuthMethod {
	switch c.AuthMethodStr {
	case "client_secret_basic":
		return oidc.AuthMethodBasic
	case "client_secret_post":
		return oidc.AuthMethodPost
	case "private_key_jwt":
		return oidc.AuthMethodPrivateKeyJWT
	default:
		return oidc.AuthMethodNone
	}
}

func (c *Client) ResponseTypes() []oidc.ResponseType {
	types := make([]oidc.ResponseType, len(c.ResponseTypeList))
	for i, rt := range c.ResponseTypeList {
		types[i] = oidc.ResponseType(rt)
	}
	return types
}

func (c *Client) GrantTypes() []oidc.GrantType {
	types := make([]oidc.GrantType, len(c.GrantTypeList))
	for i, gt := range c.GrantTypeList {
		types[i] = oidc.GrantType(gt)
	}
	return types
}

// LoginURL returns the URL the OP will redirect to for user login.
// Routing through /login/select lets users choose among available providers.
func (c *Client) LoginURL(id string) string {
	return "/login/select?authRequestID=" + id
}

func (c *Client) AccessTokenType() op.AccessTokenType {
	if c.AccessTokenTypeStr == "jwt" {
		return op.AccessTokenTypeJWT
	}
	return op.AccessTokenTypeBearer
}

func (c *Client) IDTokenLifetime() time.Duration {
	if c.idTokenTTL > 0 {
		return c.idTokenTTL
	}
	return 1 * time.Hour
}

func (c *Client) DevMode() bool {
	return false
}

func (c *Client) RestrictAdditionalIdTokenScopes() func(scopes []string) []string {
	return filterAllowedScopes
}

func (c *Client) RestrictAdditionalAccessTokenScopes() func(scopes []string) []string {
	return filterAllowedScopes
}

func (c *Client) IsScopeAllowed(scope string) bool {
	return scope == oidc.ScopeOfflineAccess
}

func (c *Client) IDTokenUserinfoClaimsAssertion() bool {
	return c.IDTokenUserinfoAssertion
}

func (c *Client) ClockSkew() time.Duration {
	return time.Duration(c.ClockSkewSeconds) * time.Second
}
