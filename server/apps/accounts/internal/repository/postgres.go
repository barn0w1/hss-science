package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/barn0w1/hss-science/server/apps/accounts/internal/domain"
)

type postgresRepo struct {
	db *sql.DB
}

func NewPostgresRepo(db *sql.DB) Repository {
	return &postgresRepo{db: db}
}

const getUserQuery = `
SELECT id, username, avatar_url, role, created_at
FROM users
WHERE id = $1
LIMIT 1;
`

func (r *postgresRepo) GetUser(ctx context.Context, id string) (*domain.User, error) {
	row := r.db.QueryRowContext(ctx, getUserQuery, id)

	var u domain.User
	// Scanで手動マッピング。泥臭いが、これが一番速いし確実。
	err := row.Scan(
		&u.ID,
		&u.Username,
		&u.AvatarURL,
		&u.Role,
		&u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // または独自のエラー
	}
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}

	return &u, nil
}
