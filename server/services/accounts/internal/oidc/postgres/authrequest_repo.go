package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

var _ oidc.AuthRequestRepository = (*AuthRequestRepository)(nil)

type AuthRequestRepository struct {
	db *sqlx.DB
}

func NewAuthRequestRepository(db *sqlx.DB) *AuthRequestRepository {
	return &AuthRequestRepository{db: db}
}

func (r *AuthRequestRepository) Create(ctx context.Context, ar *oidc.AuthRequest) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO auth_requests
		 (id, client_id, redirect_uri, state, nonce, scopes, response_type, response_mode,
		  code_challenge, code_challenge_method, prompt, max_age, login_hint)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		ar.ID, ar.ClientID, ar.RedirectURI, ar.State, ar.Nonce,
		pq.Array(ar.Scopes), ar.ResponseType, ar.ResponseMode,
		ar.CodeChallenge, ar.CodeChallengeMethod,
		pq.Array(ar.Prompt), ar.MaxAge, ar.LoginHint,
	)
	return err
}

func (r *AuthRequestRepository) GetByID(ctx context.Context, id string) (*oidc.AuthRequest, error) {
	return r.scanOne(ctx,
		`SELECT id, client_id, redirect_uri, state, nonce, scopes, response_type, response_mode,
		        code_challenge, code_challenge_method, prompt, max_age, login_hint,
		        user_id, auth_time, amr, is_done, code, created_at
		 FROM auth_requests WHERE id = $1`, id)
}

func (r *AuthRequestRepository) GetByCode(ctx context.Context, code string) (*oidc.AuthRequest, error) {
	return r.scanOne(ctx,
		`SELECT id, client_id, redirect_uri, state, nonce, scopes, response_type, response_mode,
		        code_challenge, code_challenge_method, prompt, max_age, login_hint,
		        user_id, auth_time, amr, is_done, code, created_at
		 FROM auth_requests WHERE code = $1`, code)
}

func (r *AuthRequestRepository) SaveCode(ctx context.Context, id, code string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE auth_requests SET code = $1 WHERE id = $2`, code, id)
	return err
}

func (r *AuthRequestRepository) CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE auth_requests SET user_id = $1, auth_time = $2, amr = $3, is_done = true WHERE id = $4`,
		userID, authTime, pq.Array(amr), id)
	return err
}

func (r *AuthRequestRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM auth_requests WHERE id = $1`, id)
	return err
}

func (r *AuthRequestRepository) DeleteExpiredBefore(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM auth_requests WHERE created_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *AuthRequestRepository) scanOne(ctx context.Context, query string, args ...any) (*oidc.AuthRequest, error) {
	row := r.db.QueryRowxContext(ctx, query, args...)
	var ar oidc.AuthRequest
	var scopes, prompt, amr pq.StringArray
	var userID *string
	var authTime *time.Time
	var code *string
	err := row.Scan(
		&ar.ID, &ar.ClientID, &ar.RedirectURI, &ar.State, &ar.Nonce,
		&scopes, &ar.ResponseType, &ar.ResponseMode,
		&ar.CodeChallenge, &ar.CodeChallengeMethod,
		&prompt, &ar.MaxAge, &ar.LoginHint,
		&userID, &authTime, &amr, &ar.IsDone, &code, &ar.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("auth request: %w", domerr.ErrNotFound)
		}
		return nil, err
	}
	ar.Scopes = scopes
	ar.Prompt = prompt
	ar.AMR = amr
	if userID != nil {
		ar.UserID = *userID
	}
	if authTime != nil {
		ar.AuthTime = *authTime
	}
	if code != nil {
		ar.Code = *code
	}
	return &ar, nil
}
