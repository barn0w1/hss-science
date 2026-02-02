package postgres

import (
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/repository"
	"github.com/jmoiron/sqlx"
)

// ensure interface implementation
var _ repository.UserRepository = (*UserRepository)(nil)
var _ repository.SessionRepository = (*SessionRepository)(nil)
var _ repository.AuthCodeRepository = (*AuthCodeRepository)(nil)

// UserRepository implements domain/repository.UserRepository
type UserRepository struct {
	db *sqlx.DB
}

// SessionRepository implements domain/repository.SessionRepository
type SessionRepository struct {
	db *sqlx.DB
}

// AuthCodeRepository implements domain/repository.AuthCodeRepository
type AuthCodeRepository struct {
	db *sqlx.DB
}

// NewUserRepository creates a new postgres user repository.
func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

// NewSessionRepository creates a new postgres session repository.
func NewSessionRepository(db *sqlx.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// NewAuthCodeRepository creates a new postgres auth code repository.
func NewAuthCodeRepository(db *sqlx.DB) *AuthCodeRepository {
	return &AuthCodeRepository{db: db}
}
