package adapter

import (
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"

	oidcdom "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
)

type AuthRequest struct {
	domain *oidcdom.AuthRequest
}

func NewAuthRequest(ar *oidcdom.AuthRequest) *AuthRequest {
	return &AuthRequest{domain: ar}
}

func (a *AuthRequest) GetID() string              { return a.domain.ID }
func (a *AuthRequest) GetACR() string             { return "" }
func (a *AuthRequest) GetAMR() []string           { return a.domain.AMR }
func (a *AuthRequest) GetClientID() string        { return a.domain.ClientID }
func (a *AuthRequest) GetRedirectURI() string     { return a.domain.RedirectURI }
func (a *AuthRequest) GetScopes() []string        { return a.domain.Scopes }
func (a *AuthRequest) GetState() string           { return a.domain.State }
func (a *AuthRequest) GetSubject() string         { return a.domain.UserID }
func (a *AuthRequest) GetNonce() string           { return a.domain.Nonce }
func (a *AuthRequest) GetAuthTime() time.Time     { return a.domain.AuthTime }
func (a *AuthRequest) Done() bool                 { return a.domain.IsDone }
func (a *AuthRequest) GetDeviceSessionID() string { return a.domain.DeviceSessionID }

func (a *AuthRequest) GetAudience() []string {
	return []string{a.domain.ClientID}
}

func (a *AuthRequest) GetResponseType() oidc.ResponseType {
	return oidc.ResponseType(a.domain.ResponseType)
}

func (a *AuthRequest) GetResponseMode() oidc.ResponseMode {
	return oidc.ResponseMode(a.domain.ResponseMode)
}

func (a *AuthRequest) GetCodeChallenge() *oidc.CodeChallenge {
	if a.domain.CodeChallenge == "" {
		return nil
	}
	return &oidc.CodeChallenge{
		Challenge: a.domain.CodeChallenge,
		Method:    oidc.CodeChallengeMethod(a.domain.CodeChallengeMethod),
	}
}
