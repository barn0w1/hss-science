package adapter

import (
	"testing"
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"

	oidcdom "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc"
)

func testDomainAuthRequest() *oidcdom.AuthRequest {
	now := time.Now().UTC()
	return &oidcdom.AuthRequest{
		ID:                  "ar-123",
		ClientID:            "test-client",
		RedirectURI:         "https://app.example.com/callback",
		State:               "xyz",
		Nonce:               "abc",
		Scopes:              []string{"openid", "email", "profile"},
		ResponseType:        "code",
		ResponseMode:        "query",
		CodeChallenge:       "ch4ll3ng3",
		CodeChallengeMethod: "S256",
		UserID:              "user-456",
		AuthTime:            now,
		AMR:                 []string{"federated"},
		IsDone:              false,
	}
}

func TestAuthRequest_GetID(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	if ar.GetID() != "ar-123" {
		t.Errorf("expected ar-123, got %s", ar.GetID())
	}
}

func TestAuthRequest_GetACR(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	if ar.GetACR() != "" {
		t.Errorf("expected empty ACR, got %s", ar.GetACR())
	}
}

func TestAuthRequest_GetAMR(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	amr := ar.GetAMR()
	if len(amr) != 1 || amr[0] != "federated" {
		t.Errorf("unexpected AMR: %v", amr)
	}
}

func TestAuthRequest_GetClientID(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	if ar.GetClientID() != "test-client" {
		t.Errorf("expected test-client, got %s", ar.GetClientID())
	}
}

func TestAuthRequest_GetRedirectURI(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	if ar.GetRedirectURI() != "https://app.example.com/callback" {
		t.Errorf("unexpected redirect URI: %s", ar.GetRedirectURI())
	}
}

func TestAuthRequest_GetScopes(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	scopes := ar.GetScopes()
	if len(scopes) != 3 {
		t.Fatalf("expected 3 scopes, got %d", len(scopes))
	}
}

func TestAuthRequest_GetState(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	if ar.GetState() != "xyz" {
		t.Errorf("expected xyz, got %s", ar.GetState())
	}
}

func TestAuthRequest_GetSubject(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	if ar.GetSubject() != "user-456" {
		t.Errorf("expected user-456, got %s", ar.GetSubject())
	}
}

func TestAuthRequest_GetNonce(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	if ar.GetNonce() != "abc" {
		t.Errorf("expected abc, got %s", ar.GetNonce())
	}
}

func TestAuthRequest_GetAuthTime(t *testing.T) {
	m := testDomainAuthRequest()
	ar := NewAuthRequest(m)
	if !ar.GetAuthTime().Equal(m.AuthTime) {
		t.Errorf("auth time mismatch")
	}
}

func TestAuthRequest_Done(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	if ar.Done() {
		t.Error("expected Done to be false")
	}

	m := testDomainAuthRequest()
	m.IsDone = true
	ar2 := NewAuthRequest(m)
	if !ar2.Done() {
		t.Error("expected Done to be true")
	}
}

func TestAuthRequest_GetAudience(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	aud := ar.GetAudience()
	if len(aud) != 1 || aud[0] != "test-client" {
		t.Errorf("expected audience [test-client], got %v", aud)
	}
}

func TestAuthRequest_GetResponseType(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	if ar.GetResponseType() != oidc.ResponseTypeCode {
		t.Errorf("expected code, got %s", ar.GetResponseType())
	}
}

func TestAuthRequest_GetResponseMode(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	if ar.GetResponseMode() != oidc.ResponseModeQuery {
		t.Errorf("expected query, got %s", ar.GetResponseMode())
	}
}

func TestAuthRequest_GetCodeChallenge(t *testing.T) {
	ar := NewAuthRequest(testDomainAuthRequest())
	cc := ar.GetCodeChallenge()
	if cc == nil {
		t.Fatal("expected non-nil code challenge")
	}
	if cc.Challenge != "ch4ll3ng3" {
		t.Errorf("expected ch4ll3ng3, got %s", cc.Challenge)
	}
	if cc.Method != oidc.CodeChallengeMethodS256 {
		t.Errorf("expected S256, got %s", cc.Method)
	}
}

func TestAuthRequest_GetCodeChallenge_Empty(t *testing.T) {
	m := testDomainAuthRequest()
	m.CodeChallenge = ""
	ar := NewAuthRequest(m)
	if ar.GetCodeChallenge() != nil {
		t.Error("expected nil code challenge when empty")
	}
}

func TestRefreshTokenRequest_Methods(t *testing.T) {
	now := time.Now().UTC()
	rt := &oidcdom.RefreshToken{
		ID:       "rt-1",
		Token:    "tok",
		ClientID: "client-1",
		UserID:   "user-1",
		Audience: []string{"client-1"},
		Scopes:   []string{"openid", "email"},
		AuthTime: now,
		AMR:      []string{"federated"},
	}
	rtr := NewRefreshTokenRequest(rt)

	if rtr.GetSubject() != "user-1" {
		t.Errorf("expected user-1, got %s", rtr.GetSubject())
	}
	if rtr.GetClientID() != "client-1" {
		t.Errorf("expected client-1, got %s", rtr.GetClientID())
	}
	if len(rtr.GetScopes()) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(rtr.GetScopes()))
	}
	if len(rtr.GetAudience()) != 1 {
		t.Errorf("expected 1 audience, got %d", len(rtr.GetAudience()))
	}
	if len(rtr.GetAMR()) != 1 {
		t.Errorf("expected 1 AMR, got %d", len(rtr.GetAMR()))
	}
	if !rtr.GetAuthTime().Equal(now) {
		t.Error("auth time mismatch")
	}

	rtr.SetCurrentScopes([]string{"openid"})
	if len(rtr.GetScopes()) != 1 {
		t.Errorf("expected 1 scope after SetCurrentScopes, got %d", len(rtr.GetScopes()))
	}
}
