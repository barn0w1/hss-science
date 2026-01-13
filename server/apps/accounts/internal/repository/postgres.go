package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/barn0w1/hss-science/server/apps/accounts/internal/domain"
	"github.com/jmoiron/sqlx"
)

type postgresRepo struct {
	db *sqlx.DB
}

// NewPostgresRepo returns a repository instance backed by sqlx.
func NewPostgresRepo(db *sqlx.DB) Repository {
	return &postgresRepo{db: db}
}

const getUserQuery = `
SELECT id, username, avatar_url, role, created_at
FROM users
WHERE id = $1
LIMIT 1;
`

func (r *postgresRepo) GetUser(ctx context.Context, id string) (*domain.User, error) {
	var u domain.User
	// sqlx.Get handles QueryRow + Scan automatically using struct tags
	if err := r.db.GetContext(ctx, &u, getUserQuery, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return nil when not found, as per common Go repository pattern
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &u, nil
}

const createUserQuery = `
INSERT INTO users (id, username, avatar_url, role, created_at)
VALUES (:id, :username, :avatar_url, :role, :created_at);
`

func (r *postgresRepo) CreateUser(ctx context.Context, user *domain.User) error {
	if _, err := r.db.NamedExecContext(ctx, createUserQuery, user); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}
