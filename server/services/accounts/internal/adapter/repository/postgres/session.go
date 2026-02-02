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

type sessionDTO struct {
	ID        uuid.UUID      `db:"id"`
	UserID    uuid.UUID      `db:"user_id"`
	ExpiresAt time.Time      `db:"expires_at"`
	CreatedAt time.Time      `db:"created_at"`
	RevokedAt *time.Time     `db:"revoked_at"`
	UserAgent sql.NullString `db:"user_agent"`
	IPAddress sql.NullString `db:"ip_address"`
}

func (d *sessionDTO) toDomain() *model.Session {
	userAgent := ""
	if d.UserAgent.Valid {
		userAgent = d.UserAgent.String
	}
	ipAddress := ""
	if d.IPAddress.Valid {
		ipAddress = d.IPAddress.String
	}

	return &model.Session{
		ID:        d.ID,
		UserID:    d.UserID,
		ExpiresAt: d.ExpiresAt,
		CreatedAt: d.CreatedAt,
		RevokedAt: d.RevokedAt,
		UserAgent: userAgent,
		IPAddress: ipAddress,
	}
}

func fromDomainSession(s *model.Session) *sessionDTO {
	userAgent := sql.NullString{Valid: false}
	if s.UserAgent != "" {
		userAgent = sql.NullString{String: s.UserAgent, Valid: true}
	}
	ipAddress := sql.NullString{Valid: false}
	if s.IPAddress != "" {
		ipAddress = sql.NullString{String: s.IPAddress, Valid: true}
	}

	return &sessionDTO{
		ID:        s.ID,
		UserID:    s.UserID,
		ExpiresAt: s.ExpiresAt,
		CreatedAt: s.CreatedAt,
		RevokedAt: s.RevokedAt,
		UserAgent: userAgent,
		IPAddress: ipAddress,
	}
}

const (
	sessionColumns = "id, user_id, expires_at, created_at, revoked_at, user_agent, ip_address"

	queryCreateSession = `
		INSERT INTO sessions (id, user_id, expires_at, created_at, revoked_at, user_agent, ip_address)
		VALUES (:id, :user_id, :expires_at, :created_at, :revoked_at, :user_agent, :ip_address)
	`
	queryGetSessionByID = `
		SELECT ` + sessionColumns + ` FROM sessions WHERE id = $1
	`
	queryDeleteSession = `
		DELETE FROM sessions WHERE id = $1
	`
	queryDeleteSessionsByUserID = `
		DELETE FROM sessions WHERE user_id = $1
	`
	queryRevokeSession = `
		UPDATE sessions SET revoked_at = $2 WHERE id = $1
	`
	queryCleanupExpiredSessions = `
		DELETE FROM sessions WHERE expires_at < $1
	`
)

// Create inserts a new session.
func (r *SessionRepository) Create(ctx context.Context, session *model.Session) error {
	dto := fromDomainSession(session)
	_, err := r.db.NamedExecContext(ctx, queryCreateSession, dto)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// GetByID retrieves a session by ID.
func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Session, error) {
	var dto sessionDTO
	if err := r.db.GetContext(ctx, &dto, queryGetSessionByID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get session by id: %w", err)
	}
	return dto.toDomain(), nil
}

// Delete removes a session by ID.
func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, queryDeleteSession, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// Revoke marks a session as revoked.
func (r *SessionRepository) Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, queryRevokeSession, id, revokedAt)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}
	return nil
}

// DeleteByUserID removes all sessions for a user.
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, queryDeleteSessionsByUserID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete sessions by user id: %w", err)
	}
	return nil
}

// CleanupExpired removes expired sessions.
func (r *SessionRepository) CleanupExpired(ctx context.Context, cutoff time.Time) error {
	_, err := r.db.ExecContext(ctx, queryCleanupExpiredSessions, cutoff)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	return nil
}
