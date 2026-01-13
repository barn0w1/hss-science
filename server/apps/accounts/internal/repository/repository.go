package repository

import (
	"context"

	"github.com/barn0w1/hss-science/server/apps/accounts/internal/domain"
)

type Repository interface {
	GetUser(ctx context.Context, id string) (*domain.User, error)
	CreateUser(ctx context.Context, user *domain.User) error
}
