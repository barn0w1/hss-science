package oidcprovider

import (
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/model"
)

type RefreshTokenRequest struct {
	model  *model.RefreshToken
	scopes []string
}

func NewRefreshTokenRequest(rt *model.RefreshToken) *RefreshTokenRequest {
	return &RefreshTokenRequest{
		model:  rt,
		scopes: rt.Scopes,
	}
}

func (r *RefreshTokenRequest) GetAMR() []string            { return r.model.AMR }
func (r *RefreshTokenRequest) GetAudience() []string       { return r.model.Audience }
func (r *RefreshTokenRequest) GetAuthTime() time.Time      { return r.model.AuthTime }
func (r *RefreshTokenRequest) GetClientID() string         { return r.model.ClientID }
func (r *RefreshTokenRequest) GetScopes() []string         { return r.scopes }
func (r *RefreshTokenRequest) GetSubject() string          { return r.model.UserID }
func (r *RefreshTokenRequest) SetCurrentScopes(s []string) { r.scopes = s }
