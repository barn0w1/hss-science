package adapter

import (
	"testing"
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	oidcdom "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc"
)

func testDomainClient() *oidcdom.Client {
	return &oidcdom.Client{
		ID:                       "test-client",
		SecretHash:               "$2a$10$abc",
		RedirectURIs:             []string{"https://app.example.com/callback"},
		PostLogoutRedirectURIs:   []string{"https://app.example.com/"},
		ApplicationType:          "web",
		AuthMethod:               "client_secret_basic",
		ResponseTypes:            []string{"code"},
		GrantTypes:               []string{"authorization_code", "refresh_token"},
		AccessTokenType:          "jwt",
		IDTokenLifetimeSeconds:   3600,
		ClockSkewSeconds:         5,
		IDTokenUserinfoAssertion: true,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}
}

func TestClient_GetID(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	if c.GetID() != "test-client" {
		t.Errorf("expected test-client, got %s", c.GetID())
	}
}

func TestClient_RedirectURIs(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	uris := c.RedirectURIs()
	if len(uris) != 1 || uris[0] != "https://app.example.com/callback" {
		t.Errorf("unexpected redirect URIs: %v", uris)
	}
}

func TestClient_PostLogoutRedirectURIs(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	uris := c.PostLogoutRedirectURIs()
	if len(uris) != 1 || uris[0] != "https://app.example.com/" {
		t.Errorf("unexpected post-logout redirect URIs: %v", uris)
	}
}

func TestClient_LoginURL(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	url := c.LoginURL("req-123")
	expected := "/login?authRequestID=req-123"
	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}

func TestClient_DevMode(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	if c.DevMode() {
		t.Error("expected DevMode to be false")
	}
}

func TestClient_IsScopeAllowed(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	if c.IsScopeAllowed("openid") {
		t.Error("expected IsScopeAllowed to return false")
	}
}

func TestClient_ApplicationType_Web(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	if c.ApplicationType() != op.ApplicationTypeWeb {
		t.Errorf("expected web, got %d", c.ApplicationType())
	}
}

func TestClient_ApplicationType_Native(t *testing.T) {
	m := testDomainClient()
	m.ApplicationType = "native"
	c := NewClientAdapter(m)
	if c.ApplicationType() != op.ApplicationTypeNative {
		t.Errorf("expected native, got %d", c.ApplicationType())
	}
}

func TestClient_ApplicationType_UserAgent(t *testing.T) {
	m := testDomainClient()
	m.ApplicationType = "user_agent"
	c := NewClientAdapter(m)
	if c.ApplicationType() != op.ApplicationTypeUserAgent {
		t.Errorf("expected user_agent, got %d", c.ApplicationType())
	}
}

func TestClient_AuthMethod_Basic(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	if c.AuthMethod() != oidc.AuthMethodBasic {
		t.Errorf("expected basic, got %s", c.AuthMethod())
	}
}

func TestClient_AuthMethod_Post(t *testing.T) {
	m := testDomainClient()
	m.AuthMethod = "client_secret_post"
	c := NewClientAdapter(m)
	if c.AuthMethod() != oidc.AuthMethodPost {
		t.Errorf("expected post, got %s", c.AuthMethod())
	}
}

func TestClient_AuthMethod_None(t *testing.T) {
	m := testDomainClient()
	m.AuthMethod = "none"
	c := NewClientAdapter(m)
	if c.AuthMethod() != oidc.AuthMethodNone {
		t.Errorf("expected none, got %s", c.AuthMethod())
	}
}

func TestClient_AuthMethod_PrivateKeyJWT(t *testing.T) {
	m := testDomainClient()
	m.AuthMethod = "private_key_jwt"
	c := NewClientAdapter(m)
	if c.AuthMethod() != oidc.AuthMethodPrivateKeyJWT {
		t.Errorf("expected private_key_jwt, got %s", c.AuthMethod())
	}
}

func TestClient_ResponseTypes(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	rts := c.ResponseTypes()
	if len(rts) != 1 || rts[0] != oidc.ResponseTypeCode {
		t.Errorf("unexpected response types: %v", rts)
	}
}

func TestClient_GrantTypes(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	gts := c.GrantTypes()
	if len(gts) != 2 {
		t.Fatalf("expected 2 grant types, got %d", len(gts))
	}
	if gts[0] != oidc.GrantTypeCode {
		t.Errorf("expected authorization_code, got %s", gts[0])
	}
}

func TestClient_AccessTokenType_JWT(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	if c.AccessTokenType() != op.AccessTokenTypeJWT {
		t.Errorf("expected JWT, got %d", c.AccessTokenType())
	}
}

func TestClient_AccessTokenType_Bearer(t *testing.T) {
	m := testDomainClient()
	m.AccessTokenType = "bearer"
	c := NewClientAdapter(m)
	if c.AccessTokenType() != op.AccessTokenTypeBearer {
		t.Errorf("expected Bearer, got %d", c.AccessTokenType())
	}
}

func TestClient_IDTokenLifetime(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	if c.IDTokenLifetime() != time.Hour {
		t.Errorf("expected 1h, got %v", c.IDTokenLifetime())
	}
}

func TestClient_ClockSkew(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	if c.ClockSkew() != 5*time.Second {
		t.Errorf("expected 5s, got %v", c.ClockSkew())
	}
}

func TestClient_IDTokenUserinfoClaimsAssertion(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	if !c.IDTokenUserinfoClaimsAssertion() {
		t.Error("expected IDTokenUserinfoClaimsAssertion to be true")
	}
}

func TestClient_RestrictAdditionalIdTokenScopes(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	fn := c.RestrictAdditionalIdTokenScopes()
	scopes := fn([]string{"openid", "email", "profile"})
	if len(scopes) != 3 {
		t.Errorf("expected 3 scopes returned, got %d", len(scopes))
	}
}

func TestClient_RestrictAdditionalAccessTokenScopes(t *testing.T) {
	c := NewClientAdapter(testDomainClient())
	fn := c.RestrictAdditionalAccessTokenScopes()
	scopes := fn([]string{"openid", "email"})
	if len(scopes) != 2 {
		t.Errorf("expected 2 scopes returned, got %d", len(scopes))
	}
}
