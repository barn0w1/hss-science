package oidc

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"
)

var _ TokenService = (*tokenService)(nil)

type tokenService struct {
	repo TokenRepository
}

func NewTokenService(repo TokenRepository) TokenService {
	return &tokenService{repo: repo}
}

func newID() string {
	return ulid.Make().String()
}

func (s *tokenService) CreateAccess(ctx context.Context, clientID, subject string, audience, scopes []string, expiration time.Time) (string, error) {
	id := newID()
	access := &Token{
		ID:         id,
		ClientID:   clientID,
		Subject:    subject,
		Audience:   audience,
		Scopes:     scopes,
		Expiration: expiration,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.repo.CreateAccess(ctx, access); err != nil {
		return "", err
	}
	return id, nil
}

func (s *tokenService) CreateAccessAndRefresh(
	ctx context.Context, clientID, subject string,
	audience, scopes []string,
	accessExpiration, refreshExpiration, authTime time.Time,
	amr []string, currentRefreshToken string,
) (accessID, refreshToken string, err error) {
	now := time.Now().UTC()
	accessID = newID()
	refreshID := newID()
	refreshTokenValue := newID()

	access := &Token{
		ID:             accessID,
		ClientID:       clientID,
		Subject:        subject,
		Audience:       audience,
		Scopes:         scopes,
		Expiration:     accessExpiration,
		RefreshTokenID: refreshID,
		CreatedAt:      now,
	}
	refresh := &RefreshToken{
		ID:            refreshID,
		Token:         refreshTokenValue,
		ClientID:      clientID,
		UserID:        subject,
		Audience:      audience,
		Scopes:        scopes,
		AuthTime:      authTime,
		AMR:           amr,
		AccessTokenID: accessID,
		Expiration:    refreshExpiration,
		CreatedAt:     now,
	}
	if err := s.repo.CreateAccessAndRefresh(ctx, access, refresh, currentRefreshToken); err != nil {
		return "", "", err
	}
	return accessID, refreshTokenValue, nil
}

func (s *tokenService) GetByID(ctx context.Context, tokenID string) (*Token, error) {
	return s.repo.GetByID(ctx, tokenID)
}

func (s *tokenService) GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error) {
	return s.repo.GetRefreshToken(ctx, token)
}

func (s *tokenService) GetRefreshInfo(ctx context.Context, token string) (userID, tokenID string, err error) {
	return s.repo.GetRefreshInfo(ctx, token)
}

func (s *tokenService) DeleteByUserAndClient(ctx context.Context, userID, clientID string) error {
	return s.repo.DeleteByUserAndClient(ctx, userID, clientID)
}

func (s *tokenService) Revoke(ctx context.Context, tokenID string) error {
	return s.repo.Revoke(ctx, tokenID)
}

func (s *tokenService) RevokeRefreshToken(ctx context.Context, token string) error {
	return s.repo.RevokeRefreshToken(ctx, token)
}
