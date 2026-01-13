package service

import (
	"context"

	"github.com/barn0w1/hss-science/server/apps/accounts/internal/domain"
	"github.com/barn0w1/hss-science/server/apps/accounts/internal/repository"
)

type AuthService struct {
	repo repository.Repository
}

func NewAuthService(repo repository.Repository) *AuthService {
	return &AuthService{repo: repo}
}

func (s *AuthService) GetUser(ctx context.Context, id string) (*domain.User, error) {
	// ここにキャッシュ確認を入れたり、ログを出したりする
	return s.repo.GetUser(ctx, id)
}
