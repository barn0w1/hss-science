package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
)

type tokenDTO struct {
	TokenHash string     `db:"token_hash"`
	UserID    string     `db:"user_id"`
	ExpiresAt time.Time  `db:"expires_at"`
	CreatedAt time.Time  `db:"created_at"`
	RevokedAt *time.Time `db:"revoked_at"` // Nullable
	UserAgent string     `db:"user_agent"`
	IPAddress string     `db:"ip_address"`
}

func (d *tokenDTO) toDomain() *model.RefreshToken {
	return &model.RefreshToken{
		TokenHash: d.TokenHash,
		UserID:    d.UserID,
		ExpiresAt: d.ExpiresAt,
		CreatedAt: d.CreatedAt,
		RevokedAt: d.RevokedAt,
		UserAgent: d.UserAgent,
		IPAddress: d.IPAddress,
	}
}

func fromDomainToken(t *model.RefreshToken) *tokenDTO {
	return &tokenDTO{
		TokenHash: t.TokenHash,
		UserID:    t.UserID,
		ExpiresAt: t.ExpiresAt,
		CreatedAt: t.CreatedAt,
		RevokedAt: t.RevokedAt,
		UserAgent: t.UserAgent,
		IPAddress: t.IPAddress,
	}
}

const (
	querySaveToken = `
		INSERT INTO refresh_tokens (token_hash, user_id, expires_at, created_at, revoked_at, user_agent, ip_address)
		VALUES (:token_hash, :user_id, :expires_at, :created_at, :revoked_at, :user_agent, :ip_address)
	`
	queryGetToken = `
		SELECT * FROM refresh_tokens WHERE token_hash = $1
	`
	queryRevokeToken = `
		UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1
	`
	queryRevokeByUserID = `
		UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1
	`
	queryCleanupExpired = `
		DELETE FROM refresh_tokens WHERE expires_at < $1
	`
)

// Save stores a new refresh token.
func (r *TokenRepository) Save(ctx context.Context, token *model.RefreshToken) error {
	dto := fromDomainToken(token)
	_, err := r.db.NamedExecContext(ctx, querySaveToken, dto)
	if err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}
	return nil
}

// Get retrieves a refresh token by hash.
func (r *TokenRepository) Get(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var dto tokenDTO
	if err := r.db.GetContext(ctx, &dto, queryGetToken, tokenHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("token not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get token: %w", err)
	}
	return dto.toDomain(), nil
}

// Revoke marks a token as revoked.
func (r *TokenRepository) Revoke(ctx context.Context, tokenHash string) error {
	res, err := r.db.ExecContext(ctx, queryRevokeToken, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		// すでに存在しないか、ハッシュが合わない場合。
		// エラーにするかどうかは要件次第だが、冪等性を考えるとnilでも良い
		return errors.New("token not found or already revoked")
	}
	return nil
}

// RevokeByUserID marks all tokens for a user as revoked.
func (r *TokenRepository) RevokeByUserID(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, queryRevokeByUserID, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke tokens for user: %w", err)
	}
	return nil
}

// CleanupExpired deletes old tokens.
func (r *TokenRepository) CleanupExpired(ctx context.Context) error {
	// 現在時刻より前に期限が切れたものを削除
	_, err := r.db.ExecContext(ctx, queryCleanupExpired, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired tokens: %w", err)
	}
	return nil
}
