package usecase

import (
    "context"
    "github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
    "github.com/barn0w1/hss-science/server/services/accounts/internal/domain/repository"
)

type AuthUsecase struct {
    repo repository.UserRepository
    // tokenGenerator etc...
}

func NewAuthUsecase(repo repository.UserRepository) *AuthUsecase {
    return &AuthUsecase{repo: repo}
}

func (u *AuthUsecase) SignUp(ctx context.Context, email, password string) (*model.User, error) {
    // 1. Check if email exists
    // 2. Hash password
    // 3. Create user model
    user := &model.User{Email: email, ...}
    
    if err := u.repo.Create(ctx, user); err != nil {
        return nil, err
    }
    return user, nil
}