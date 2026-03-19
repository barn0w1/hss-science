package adapter

import (
	"time"

	oidcdom "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc"
)

type RefreshTokenRequest struct {
	domain *oidcdom.RefreshToken
	scopes []string
}

func NewRefreshTokenRequest(rt *oidcdom.RefreshToken) *RefreshTokenRequest {
	return &RefreshTokenRequest{
		domain: rt,
		scopes: rt.Scopes,
	}
}

func (r *RefreshTokenRequest) GetAMR() []string            { return r.domain.AMR }
func (r *RefreshTokenRequest) GetAudience() []string       { return r.domain.Audience }
func (r *RefreshTokenRequest) GetAuthTime() time.Time      { return r.domain.AuthTime }
func (r *RefreshTokenRequest) GetClientID() string         { return r.domain.ClientID }
func (r *RefreshTokenRequest) GetScopes() []string         { return r.scopes }
func (r *RefreshTokenRequest) GetSubject() string          { return r.domain.UserID }
func (r *RefreshTokenRequest) SetCurrentScopes(s []string) { r.scopes = s }
