package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/repository"
	"github.com/google/uuid"
)

type authCodeDTO struct {
	Code        string     `db:"code"`
	UserID      uuid.UUID  `db:"user_id"`
	Audience    string     `db:"audience"`
	RedirectURI string     `db:"redirect_uri"`
	ExpiresAt   time.Time  `db:"expires_at"`
	ConsumedAt  *time.Time `db:"consumed_at"`
	CreatedAt   time.Time  `db:"created_at"`
}

func (d *authCodeDTO) toDomain() *model.AuthCode {
	return &model.AuthCode{
		Code:        d.Code,
		UserID:      d.UserID,
		Audience:    d.Audience,
		RedirectURI: d.RedirectURI,
		CreatedAt:   d.CreatedAt,
		ExpiresAt:   d.ExpiresAt,
		ConsumedAt:  d.ConsumedAt,
	}
}

func fromDomainAuthCode(c *model.AuthCode) *authCodeDTO {
	return &authCodeDTO{
		Code:        c.Code,
		UserID:      c.UserID,
		Audience:    c.Audience,
		RedirectURI: c.RedirectURI,
		CreatedAt:   c.CreatedAt,
		ExpiresAt:   c.ExpiresAt,
		ConsumedAt:  c.ConsumedAt,
	}
}

const (
	authCodeColumns = "code, user_id, audience, redirect_uri, expires_at, consumed_at, created_at"

	queryCreateAuthCode = `
		INSERT INTO auth_codes (code, user_id, audience, redirect_uri, expires_at, consumed_at, created_at)
		VALUES (:code, :user_id, :audience, :redirect_uri, :expires_at, :consumed_at, :created_at)
	`
	queryGetAuthCodeByCode = `
		SELECT ` + authCodeColumns + ` FROM auth_codes WHERE code = $1
	`
	queryConsumeAuthCode = `
		UPDATE auth_codes SET consumed_at = $2 WHERE code = $1 AND consumed_at IS NULL
	`
	queryDeleteAuthCode = `
		DELETE FROM auth_codes WHERE code = $1
	`
	queryCleanupExpiredAuthCodes = `
		DELETE FROM auth_codes WHERE expires_at < $1
	`
)

// Create inserts a new auth code.
func (r *AuthCodeRepository) Create(ctx context.Context, code *model.AuthCode) error {
	dto := fromDomainAuthCode(code)
	_, err := r.db.NamedExecContext(ctx, queryCreateAuthCode, dto)
	if err != nil {
		return fmt.Errorf("failed to create auth code: %w", err)
	}
	return nil
}

// GetByCode retrieves an auth code by its code value.
func (r *AuthCodeRepository) GetByCode(ctx context.Context, code string) (*model.AuthCode, error) {
	var dto authCodeDTO
	if err := r.db.GetContext(ctx, &dto, queryGetAuthCodeByCode, code); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get auth code: %w", err)
	}
	return dto.toDomain(), nil
}

// Consume marks an auth code as used.
func (r *AuthCodeRepository) Consume(ctx context.Context, code string, consumedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, queryConsumeAuthCode, code, consumedAt)
	if err != nil {
		return fmt.Errorf("failed to consume auth code: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read consume result: %w", err)
	}
	if rows == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// Delete removes an auth code.
func (r *AuthCodeRepository) Delete(ctx context.Context, code string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteAuthCode, code)
	if err != nil {
		return fmt.Errorf("failed to delete auth code: %w", err)
	}
	return nil
}

// CleanupExpired removes expired auth codes.
func (r *AuthCodeRepository) CleanupExpired(ctx context.Context, cutoff time.Time) error {
	_, err := r.db.ExecContext(ctx, queryCleanupExpiredAuthCodes, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired auth codes: %w", err)
	}
	return nil
}
