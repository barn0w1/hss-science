package repository

import (
	"context"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
)

// Repository Interface definition lives HERE (Dependency Inversion)
type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByEmail(ctx context.Context, email string) (*model.User, error)
}
