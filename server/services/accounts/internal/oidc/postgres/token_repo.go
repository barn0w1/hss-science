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

var _ oidc.TokenRepository = (*TokenRepository)(nil)

type TokenRepository struct {
	db *sqlx.DB
}

func NewTokenRepository(db *sqlx.DB) *TokenRepository {
	return &TokenRepository{db: db}
}

func (r *TokenRepository) CreateAccess(ctx context.Context, access *oidc.Token) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tokens (id, client_id, subject, audience, scopes, expiration, refresh_token_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		access.ID, access.ClientID, access.Subject,
		pq.Array(access.Audience), pq.Array(access.Scopes),
		access.Expiration, nilIfEmpty(access.RefreshTokenID),
	)
	return err
}

func (r *TokenRepository) CreateAccessAndRefresh(ctx context.Context, access *oidc.Token, refresh *oidc.RefreshToken, currentRefreshToken string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if currentRefreshToken != "" {
		var oldAccessTokenID sql.NullString
		err = tx.QueryRowContext(ctx,
			`SELECT access_token_id FROM refresh_tokens WHERE token_hash = $1`,
			currentRefreshToken,
		).Scan(&oldAccessTokenID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("lookup old refresh token: %w", err)
		}

		if oldAccessTokenID.Valid && oldAccessTokenID.String != "" {
			if _, err = tx.ExecContext(ctx,
				`DELETE FROM tokens WHERE id = $1`, oldAccessTokenID.String,
			); err != nil {
				return fmt.Errorf("revoke old access token: %w", err)
			}
		}

		result, err := tx.ExecContext(ctx,
			`DELETE FROM refresh_tokens WHERE token_hash = $1 AND expiration > now()`, currentRefreshToken,
		)
		if err != nil {
			return fmt.Errorf("delete old refresh token: %w", err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("rows affected: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("refresh token already used or expired: %w", domerr.ErrNotFound)
		}
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO tokens (id, client_id, subject, audience, scopes, expiration, refresh_token_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		access.ID, access.ClientID, access.Subject,
		pq.Array(access.Audience), pq.Array(access.Scopes),
		access.Expiration, access.RefreshTokenID,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, token_hash, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expiration)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		refresh.ID, refresh.Token, refresh.ClientID, refresh.UserID,
		pq.Array(refresh.Audience), pq.Array(refresh.Scopes),
		refresh.AuthTime, pq.Array(refresh.AMR),
		refresh.AccessTokenID, refresh.Expiration,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *TokenRepository) GetByID(ctx context.Context, tokenID string) (*oidc.Token, error) {
	row := r.db.QueryRowxContext(ctx,
		`SELECT id, client_id, subject, audience, scopes, expiration, refresh_token_id, created_at
		 FROM tokens WHERE id = $1 AND expiration > now()`, tokenID)
	return r.scanToken(row)
}

func (r *TokenRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*oidc.RefreshToken, error) {
	row := r.db.QueryRowxContext(ctx,
		`SELECT id, token_hash, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expiration, created_at
		 FROM refresh_tokens WHERE token_hash = $1 AND expiration > now()`, tokenHash)
	return r.scanRefreshToken(row)
}

func (r *TokenRepository) GetRefreshInfo(ctx context.Context, tokenHash string) (userID, tokenID string, err error) {
	err = r.db.QueryRowxContext(ctx,
		`SELECT user_id, id FROM refresh_tokens WHERE token_hash = $1 AND expiration > now()`, tokenHash,
	).Scan(&userID, &tokenID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", fmt.Errorf("refresh token: %w", domerr.ErrNotFound)
		}
		return "", "", err
	}
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

func (r *TokenRepository) Revoke(ctx context.Context, tokenID, clientID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM tokens WHERE id = $1 AND client_id = $2`, tokenID, clientID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("token %s: %w", tokenID, domerr.ErrNotFound)
	}
	return nil
}

func (r *TokenRepository) RevokeRefreshToken(ctx context.Context, tokenHash, clientID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM refresh_tokens WHERE token_hash = $1 AND client_id = $2`, tokenHash, clientID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("refresh token: %w", domerr.ErrNotFound)
	}
	return nil
}

func (r *TokenRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, int64, error) {
	res1, err := r.db.ExecContext(ctx, `DELETE FROM tokens WHERE expiration < $1`, before)
	if err != nil {
		return 0, 0, fmt.Errorf("delete expired access tokens: %w", err)
	}
	accessDeleted, _ := res1.RowsAffected()

	res2, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE expiration < $1`, before)
	if err != nil {
		return accessDeleted, 0, fmt.Errorf("delete expired refresh tokens: %w", err)
	}
	refreshDeleted, _ := res2.RowsAffected()

	return accessDeleted, refreshDeleted, nil
}

func (r *TokenRepository) scanToken(row *sqlx.Row) (*oidc.Token, error) {
	var t oidc.Token
	var audience, scopes pq.StringArray
	var refreshTokenID *string
	err := row.Scan(&t.ID, &t.ClientID, &t.Subject, &audience, &scopes, &t.Expiration, &refreshTokenID, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("token: %w", domerr.ErrNotFound)
		}
		return nil, err
	}
	t.Audience = audience
	t.Scopes = scopes
	if refreshTokenID != nil {
		t.RefreshTokenID = *refreshTokenID
	}
	return &t, nil
}

func (r *TokenRepository) scanRefreshToken(row *sqlx.Row) (*oidc.RefreshToken, error) {
	var rt oidc.RefreshToken
	var audience, scopes, amr pq.StringArray
	var accessTokenID *string
	err := row.Scan(&rt.ID, &rt.Token, &rt.ClientID, &rt.UserID, &audience, &scopes,
		&rt.AuthTime, &amr, &accessTokenID, &rt.Expiration, &rt.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("refresh token: %w", domerr.ErrNotFound)
		}
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

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
