package repository

import (
	"context"
	"database/sql"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/repository"
	// pgx driver
)

type postgresUserRepo struct {
	db *sql.DB
}

// Ensure implementation
var _ repository.UserRepository = (*postgresUserRepo)(nil)

func NewPostgresUserRepo(db *sql.DB) *postgresUserRepo {
	return &postgresUserRepo{db: db}
}

func (r *postgresUserRepo) Create(ctx context.Context, user *model.User) error {
	// SQL query execution
	return nil
}
