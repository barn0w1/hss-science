package oidcprovider

import (
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/barn0w1/hss-science/server/services/accounts/model"
)

type AuthRequest struct {
	model *model.AuthRequest
}

func NewAuthRequest(m *model.AuthRequest) *AuthRequest {
	return &AuthRequest{model: m}
}

func (a *AuthRequest) GetID() string          { return a.model.ID }
func (a *AuthRequest) GetACR() string         { return "" }
func (a *AuthRequest) GetAMR() []string       { return a.model.AMR }
func (a *AuthRequest) GetClientID() string    { return a.model.ClientID }
func (a *AuthRequest) GetRedirectURI() string { return a.model.RedirectURI }
func (a *AuthRequest) GetScopes() []string    { return a.model.Scopes }
func (a *AuthRequest) GetState() string       { return a.model.State }
func (a *AuthRequest) GetSubject() string     { return a.model.UserID }
func (a *AuthRequest) GetNonce() string       { return a.model.Nonce }
func (a *AuthRequest) GetAuthTime() time.Time { return a.model.AuthTime }
func (a *AuthRequest) Done() bool             { return a.model.IsDone }

func (a *AuthRequest) GetAudience() []string {
	return []string{a.model.ClientID}
}

func (a *AuthRequest) GetResponseType() oidc.ResponseType {
	return oidc.ResponseType(a.model.ResponseType)
}

func (a *AuthRequest) GetResponseMode() oidc.ResponseMode {
	return oidc.ResponseMode(a.model.ResponseMode)
}

func (a *AuthRequest) GetCodeChallenge() *oidc.CodeChallenge {
	if a.model.CodeChallenge == "" {
		return nil
	}
	return &oidc.CodeChallenge{
		Challenge: a.model.CodeChallenge,
		Method:    oidc.CodeChallengeMethod(a.model.CodeChallengeMethod),
	}
}
