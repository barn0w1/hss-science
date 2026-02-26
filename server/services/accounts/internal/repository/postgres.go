package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/storage"
)

// Compile-time interface check.
var _ AccountRepository = (*PostgresAccountRepository)(nil)

// PostgresAccountRepository implements AccountRepository using PostgreSQL.
type PostgresAccountRepository struct {
	db *sqlx.DB
}

// NewPostgresAccountRepository creates a new repository backed by the given database.
func NewPostgresAccountRepository(db *sqlx.DB) *PostgresAccountRepository {
	return &PostgresAccountRepository{db: db}
}

func (r *PostgresAccountRepository) GetUser(ctx context.Context, userID string) (*storage.User, error) {
	var user storage.User
	err := r.db.GetContext(ctx, &user,
		`SELECT id, email, email_verified, given_name, family_name, picture, locale, created_at, updated_at
		 FROM users WHERE id = $1`, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %s", userID)
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &user, nil
}

// allowedUpdateFields restricts which columns can be updated via UpdateUser.
var allowedUpdateFields = map[string]bool{
	"given_name":  true,
	"family_name": true,
	"picture":     true,
	"locale":      true,
}

func (r *PostgresAccountRepository) UpdateUser(ctx context.Context, userID string, fields map[string]any) (*storage.User, error) {
	if len(fields) == 0 {
		return r.GetUser(ctx, userID)
	}

	setClauses := make([]string, 0, len(fields)+1)
	args := make([]any, 0, len(fields)+2)
	i := 1
	for col, val := range fields {
		if !allowedUpdateFields[col] {
			return nil, fmt.Errorf("field %q is not updatable", col)
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, userID)

	query := fmt.Sprintf(
		`UPDATE users SET %s WHERE id = $%d
		 RETURNING id, email, email_verified, given_name, family_name, picture, locale, created_at, updated_at`,
		strings.Join(setClauses, ", "), i)

	var user storage.User
	err := r.db.QueryRowxContext(ctx, query, args...).StructScan(&user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found: %s", userID)
		}
		return nil, fmt.Errorf("update user: %w", err)
	}
	return &user, nil
}

func (r *PostgresAccountRepository) ListFederatedIdentities(ctx context.Context, userID string) ([]storage.FederatedIdentity, error) {
	var identities []storage.FederatedIdentity
	err := r.db.SelectContext(ctx, &identities,
		`SELECT id, user_id, provider, external_sub, created_at
		 FROM federated_identities WHERE user_id = $1
		 ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list federated identities: %w", err)
	}
	return identities, nil
}

func (r *PostgresAccountRepository) CountFederatedIdentities(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM federated_identities WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count federated identities: %w", err)
	}
	return count, nil
}

func (r *PostgresAccountRepository) DeleteFederatedIdentity(ctx context.Context, userID, identityID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM federated_identities WHERE id = $1 AND user_id = $2`,
		identityID, userID)
	if err != nil {
		return fmt.Errorf("delete federated identity: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("federated identity not found: %s", identityID)
	}
	return nil
}

func (r *PostgresAccountRepository) ListRefreshTokens(ctx context.Context, userID string) ([]storage.RefreshToken, error) {
	var tokens []storage.RefreshToken
	err := r.db.SelectContext(ctx, &tokens,
		`SELECT id, token, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expires_at, created_at
		 FROM refresh_tokens WHERE user_id = $1 AND expires_at > now()
		 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list refresh tokens: %w", err)
	}
	return tokens, nil
}

func (r *PostgresAccountRepository) DeleteRefreshToken(ctx context.Context, userID, tokenID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get the refresh token to find the associated access token.
	var accessTokenID *string
	err = tx.QueryRowContext(ctx,
		`SELECT access_token_id FROM refresh_tokens WHERE id = $1 AND user_id = $2`,
		tokenID, userID).Scan(&accessTokenID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("session not found: %s", tokenID)
		}
		return fmt.Errorf("get refresh token: %w", err)
	}

	// Delete refresh token.
	if _, err := tx.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE id = $1`, tokenID); err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}

	// Delete associated access token if present.
	if accessTokenID != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM access_tokens WHERE id = $1`, *accessTokenID); err != nil {
			return fmt.Errorf("delete access token: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *PostgresAccountRepository) DeleteUser(ctx context.Context, userID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}
	return nil
}
