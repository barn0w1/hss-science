package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain"
)

// AuthCodeRepo implements domain.AuthCodeRepository using PostgreSQL.
type AuthCodeRepo struct {
	db *sql.DB
}

func NewAuthCodeRepo(db *sql.DB) *AuthCodeRepo {
	return &AuthCodeRepo{db: db}
}

func (r *AuthCodeRepo) Create(ctx context.Context, code *domain.AuthCode) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO auth_codes (code, user_id, redirect_uri, client_state, used, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		code.Code, code.UserID, code.RedirectURI, code.ClientState, code.Used, code.ExpiresAt, code.CreatedAt,
	)
	return err
}

func (r *AuthCodeRepo) Consume(ctx context.Context, code string) (*domain.AuthCode, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	ac := &domain.AuthCode{}
	err = tx.QueryRowContext(ctx,
		`SELECT code, user_id, redirect_uri, client_state, used, expires_at, created_at
		 FROM auth_codes WHERE code = $1 FOR UPDATE`,
		code,
	).Scan(&ac.Code, &ac.UserID, &ac.RedirectURI, &ac.ClientState, &ac.Used, &ac.ExpiresAt, &ac.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if ac.Used {
		return nil, domain.ErrAlreadyUsed
	}
	if time.Now().After(ac.ExpiresAt) {
		return nil, domain.ErrExpired
	}

	_, err = tx.ExecContext(ctx, `UPDATE auth_codes SET used = TRUE WHERE code = $1`, code)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return ac, nil
}
