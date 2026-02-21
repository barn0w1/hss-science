package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain"
)

// OAuthStateRepo implements domain.OAuthStateRepository using PostgreSQL.
type OAuthStateRepo struct {
	db *sql.DB
}

func NewOAuthStateRepo(db *sql.DB) *OAuthStateRepo {
	return &OAuthStateRepo{db: db}
}

func (r *OAuthStateRepo) Create(ctx context.Context, state *domain.OAuthState) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO oauth_states (state, provider, redirect_uri, client_state, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		state.State, state.Provider, state.RedirectURI, state.ClientState, state.ExpiresAt, state.CreatedAt,
	)
	return err
}

func (r *OAuthStateRepo) Consume(ctx context.Context, state string) (*domain.OAuthState, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	s := &domain.OAuthState{}
	err = tx.QueryRowContext(ctx,
		`SELECT state, provider, redirect_uri, client_state, expires_at, created_at
		 FROM oauth_states WHERE state = $1 FOR UPDATE`,
		state,
	).Scan(&s.State, &s.Provider, &s.RedirectURI, &s.ClientState, &s.ExpiresAt, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if time.Now().After(s.ExpiresAt) {
		// Clean up expired state.
		tx.ExecContext(ctx, `DELETE FROM oauth_states WHERE state = $1`, state)
		tx.Commit()
		return nil, domain.ErrExpired
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM oauth_states WHERE state = $1`, state)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s, nil
}
