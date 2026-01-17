package postgres

import (
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/repository"
	"github.com/jmoiron/sqlx"
)

// ensure interface implementation
var _ repository.UserRepository = (*UserRepository)(nil)
var _ repository.TokenRepository = (*TokenRepository)(nil)

// UserRepository implements domain/repository.UserRepository
type UserRepository struct {
	db *sqlx.DB
}

// TokenRepository implements domain/repository.TokenRepository
type TokenRepository struct {
	db *sqlx.DB
}

// NewUserRepository creates a new postgres user repository.
func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

// NewTokenRepository creates a new postgres token repository.
func NewTokenRepository(db *sqlx.DB) *TokenRepository {
	return &TokenRepository{db: db}
}
