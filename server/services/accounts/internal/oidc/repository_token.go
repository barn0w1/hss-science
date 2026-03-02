package oidc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type TokenRepository struct {
	db *sqlx.DB
}

func NewTokenRepository(db *sqlx.DB) *TokenRepository {
	return &TokenRepository{db: db}
}

func (r *TokenRepository) CreateAccess(ctx context.Context, clientID, subject string, audience, scopes []string, expiration time.Time) (string, error) {
	id := uuid.New().String()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tokens (id, client_id, subject, audience, scopes, expiration)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		id, clientID, subject, pq.Array(audience), pq.Array(scopes), expiration,
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *TokenRepository) CreateAccessAndRefresh(
	ctx context.Context,
	clientID, subject string,
	audience, scopes []string,
	accessExpiration time.Time,
	refreshExpiration time.Time,
	authTime time.Time,
	amr []string,
	currentRefreshToken string,
) (accessTokenID, refreshToken string, err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback() }()

	if currentRefreshToken != "" {
		_, err = tx.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token = $1`, currentRefreshToken)
		if err != nil {
			return "", "", err
		}
	}

	refreshID := uuid.New().String()
	newRefreshToken := uuid.New().String()
	accessID := uuid.New().String()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO tokens (id, client_id, subject, audience, scopes, expiration, refresh_token_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		accessID, clientID, subject, pq.Array(audience), pq.Array(scopes), accessExpiration, refreshID,
	)
	if err != nil {
		return "", "", err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, token, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expiration)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		refreshID, newRefreshToken, clientID, subject,
		pq.Array(audience), pq.Array(scopes), authTime, pq.Array(amr), accessID, refreshExpiration,
	)
	if err != nil {
		return "", "", err
	}

	if err = tx.Commit(); err != nil {
		return "", "", err
	}
	return accessID, newRefreshToken, nil
}

func (r *TokenRepository) GetByID(ctx context.Context, tokenID string) (*Token, error) {
	row := r.db.QueryRowxContext(ctx,
		`SELECT id, client_id, subject, audience, scopes, expiration, refresh_token_id, created_at
		 FROM tokens WHERE id = $1 AND expiration > now()`, tokenID)
	return r.scanToken(row)
}

func (r *TokenRepository) GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error) {
	row := r.db.QueryRowxContext(ctx,
		`SELECT id, token, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expiration, created_at
		 FROM refresh_tokens WHERE token = $1 AND expiration > now()`, token)
	return r.scanRefreshToken(row)
}

func (r *TokenRepository) GetRefreshInfo(ctx context.Context, token string) (userID, tokenID string, err error) {
	err = r.db.QueryRowxContext(ctx,
		`SELECT user_id, id FROM refresh_tokens WHERE token = $1 AND expiration > now()`, token,
	).Scan(&userID, &tokenID)
	return
}

func (r *TokenRepository) DeleteByUserAndClient(ctx context.Context, userID, clientID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1 AND client_id = $2`, userID, clientID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM tokens WHERE subject = $1 AND client_id = $2`, userID, clientID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *TokenRepository) Revoke(ctx context.Context, tokenID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tokens WHERE id = $1`, tokenID)
	return err
}

func (r *TokenRepository) RevokeRefreshToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token = $1`, token)
	return err
}

func (r *TokenRepository) scanToken(row *sqlx.Row) (*Token, error) {
	var t Token
	var audience, scopes pq.StringArray
	var refreshTokenID *string
	err := row.Scan(&t.ID, &t.ClientID, &t.Subject, &audience, &scopes, &t.Expiration, &refreshTokenID, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	t.Audience = audience
	t.Scopes = scopes
	if refreshTokenID != nil {
		t.RefreshTokenID = *refreshTokenID
	}
	return &t, nil
}

func (r *TokenRepository) scanRefreshToken(row *sqlx.Row) (*RefreshToken, error) {
	var rt RefreshToken
	var audience, scopes, amr pq.StringArray
	var accessTokenID *string
	err := row.Scan(&rt.ID, &rt.Token, &rt.ClientID, &rt.UserID, &audience, &scopes,
		&rt.AuthTime, &amr, &accessTokenID, &rt.Expiration, &rt.CreatedAt)
	if err != nil {
		return nil, err
	}
	rt.Audience = audience
	rt.Scopes = scopes
	rt.AMR = amr
	if accessTokenID != nil {
		rt.AccessTokenID = *accessTokenID
	}
	return &rt, nil
}
