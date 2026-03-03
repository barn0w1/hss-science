package oidc

import (
	"context"
	"fmt"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

var _ AuthRequestService = (*authRequestService)(nil)

type authRequestService struct {
	repo           AuthRequestRepository
	authRequestTTL time.Duration
}

func NewAuthRequestService(repo AuthRequestRepository, authRequestTTL time.Duration) AuthRequestService {
	return &authRequestService{repo: repo, authRequestTTL: authRequestTTL}
}

func (s *authRequestService) Create(ctx context.Context, ar *AuthRequest) error {
	return s.repo.Create(ctx, ar)
}

func (s *authRequestService) GetByID(ctx context.Context, id string) (*AuthRequest, error) {
	ar, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(ar.CreatedAt.Add(s.authRequestTTL)) {
		return nil, fmt.Errorf("auth request expired: %w", domerr.ErrNotFound)
	}
	return ar, nil
}

func (s *authRequestService) GetByCode(ctx context.Context, code string) (*AuthRequest, error) {
	ar, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if time.Now().UTC().After(ar.CreatedAt.Add(s.authRequestTTL)) {
		return nil, fmt.Errorf("auth request expired: %w", domerr.ErrNotFound)
	}
	return ar, nil
}

func (s *authRequestService) SaveCode(ctx context.Context, id, code string) error {
	return s.repo.SaveCode(ctx, id, code)
}

func (s *authRequestService) CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error {
	return s.repo.CompleteLogin(ctx, id, userID, authTime, amr)
}

func (s *authRequestService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *authRequestService) DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error) {
	return s.repo.DeleteExpiredBefore(ctx, before)
}
