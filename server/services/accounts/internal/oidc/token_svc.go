package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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

func newRefreshTokenValue() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashRefreshToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
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
) (accessID, rawRefreshToken string, err error) {
	now := time.Now().UTC()
	accessID = newID()
	refreshID := newID()

	rawRefreshToken, err = newRefreshTokenValue()
	if err != nil {
		return "", "", err
	}

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
		Token:         hashRefreshToken(rawRefreshToken),
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

	currentHash := ""
	if currentRefreshToken != "" {
		currentHash = hashRefreshToken(currentRefreshToken)
	}

	if err := s.repo.CreateAccessAndRefresh(ctx, access, refresh, currentHash); err != nil {
		return "", "", err
	}
	return accessID, rawRefreshToken, nil
}

func (s *tokenService) GetByID(ctx context.Context, tokenID string) (*Token, error) {
	return s.repo.GetByID(ctx, tokenID)
}

func (s *tokenService) GetRefreshToken(ctx context.Context, rawToken string) (*RefreshToken, error) {
	return s.repo.GetRefreshToken(ctx, hashRefreshToken(rawToken))
}

func (s *tokenService) GetRefreshInfo(ctx context.Context, rawToken string) (userID, tokenID string, err error) {
	return s.repo.GetRefreshInfo(ctx, hashRefreshToken(rawToken))
}

func (s *tokenService) DeleteByUserAndClient(ctx context.Context, userID, clientID string) error {
	return s.repo.DeleteByUserAndClient(ctx, userID, clientID)
}

func (s *tokenService) Revoke(ctx context.Context, tokenID, clientID string) error {
	return s.repo.Revoke(ctx, tokenID, clientID)
}

func (s *tokenService) RevokeRefreshToken(ctx context.Context, rawToken, clientID string) error {
	return s.repo.RevokeRefreshToken(ctx, hashRefreshToken(rawToken), clientID)
}

func (s *tokenService) DeleteExpired(ctx context.Context, before time.Time) (int64, int64, error) {
	return s.repo.DeleteExpired(ctx, before)
}
