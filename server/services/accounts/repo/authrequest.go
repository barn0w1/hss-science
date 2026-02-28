package repo

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/barn0w1/hss-science/server/services/accounts/model"
)

type AuthRequestRepository struct {
	db *sqlx.DB
}

func NewAuthRequestRepository(db *sqlx.DB) *AuthRequestRepository {
	return &AuthRequestRepository{db: db}
}

func (r *AuthRequestRepository) Create(ctx context.Context, ar *model.AuthRequest) error {
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

const activeFilter = `created_at > now() - interval '30 minutes'`

func (r *AuthRequestRepository) GetByID(ctx context.Context, id string) (*model.AuthRequest, error) {
	return r.scanOne(ctx,
		`SELECT id, client_id, redirect_uri, state, nonce, scopes, response_type, response_mode,
		        code_challenge, code_challenge_method, prompt, max_age, login_hint,
		        user_id, auth_time, amr, is_done, code, created_at
		 FROM auth_requests WHERE id = $1 AND `+activeFilter, id)
}

func (r *AuthRequestRepository) GetByCode(ctx context.Context, code string) (*model.AuthRequest, error) {
	return r.scanOne(ctx,
		`SELECT id, client_id, redirect_uri, state, nonce, scopes, response_type, response_mode,
		        code_challenge, code_challenge_method, prompt, max_age, login_hint,
		        user_id, auth_time, amr, is_done, code, created_at
		 FROM auth_requests WHERE code = $1 AND `+activeFilter, code)
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

func (r *AuthRequestRepository) scanOne(ctx context.Context, query string, args ...any) (*model.AuthRequest, error) {
	row := r.db.QueryRowxContext(ctx, query, args...)
	var ar model.AuthRequest
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
