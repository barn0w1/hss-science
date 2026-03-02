package oidc

import (
	"context"
	"time"
)

type AuthRequestRepository interface {
	Create(ctx context.Context, ar *AuthRequest) error
	GetByID(ctx context.Context, id string) (*AuthRequest, error)
	GetByCode(ctx context.Context, code string) (*AuthRequest, error)
	SaveCode(ctx context.Context, id, code string) error
	CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
	Delete(ctx context.Context, id string) error
}

type ClientRepository interface {
	GetByID(ctx context.Context, clientID string) (*Client, error)
}

type TokenRepository interface {
	CreateAccess(ctx context.Context, access *Token) error
	CreateAccessAndRefresh(ctx context.Context, access *Token, refresh *RefreshToken, currentRefreshToken string) error
	GetByID(ctx context.Context, tokenID string) (*Token, error)
	GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error)
	GetRefreshInfo(ctx context.Context, token string) (userID, tokenID string, err error)
	DeleteByUserAndClient(ctx context.Context, userID, clientID string) error
	Revoke(ctx context.Context, tokenID string) error
	RevokeRefreshToken(ctx context.Context, token string) error
}

type AuthRequestService interface {
	Create(ctx context.Context, ar *AuthRequest) error
	GetByID(ctx context.Context, id string) (*AuthRequest, error)
	GetByCode(ctx context.Context, code string) (*AuthRequest, error)
	SaveCode(ctx context.Context, id, code string) error
	CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
	Delete(ctx context.Context, id string) error
}

type ClientService interface {
	GetByID(ctx context.Context, clientID string) (*Client, error)
	AuthorizeSecret(ctx context.Context, clientID, clientSecret string) error
	ClientCredentials(ctx context.Context, clientID, clientSecret string) (*Client, error)
}

type TokenService interface {
	CreateAccess(ctx context.Context, clientID, subject string, audience, scopes []string, expiration time.Time) (tokenID string, err error)
	CreateAccessAndRefresh(ctx context.Context, clientID, subject string, audience, scopes []string, accessExpiration, refreshExpiration, authTime time.Time, amr []string, currentRefreshToken string) (accessID, refreshToken string, err error)
	GetByID(ctx context.Context, tokenID string) (*Token, error)
	GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error)
	GetRefreshInfo(ctx context.Context, token string) (userID, tokenID string, err error)
	DeleteByUserAndClient(ctx context.Context, userID, clientID string) error
	Revoke(ctx context.Context, tokenID string) error
	RevokeRefreshToken(ctx context.Context, token string) error
}
