package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"

	oidcdom "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc"
	"github.com/barn0w1/hss-science/server/services/identity-service/internal/pkg/domerr"
)

var _ oidcdom.DeviceSessionRepository = (*DeviceSessionRepository)(nil)

type DeviceSessionRepository struct {
	db *sqlx.DB
}

func NewDeviceSessionRepository(db *sqlx.DB) *DeviceSessionRepository {
	return &DeviceSessionRepository{db: db}
}

func (r *DeviceSessionRepository) FindOrCreate(
	ctx context.Context, id, userID, userAgent, ipAddress, deviceName string,
) (*oidcdom.DeviceSession, error) {
	var ds oidcdom.DeviceSession
	var revokedAt sql.NullTime
	err := r.db.QueryRowxContext(ctx,
		`SELECT id, user_id, user_agent, ip_address, device_name, created_at, last_used_at, revoked_at
		 FROM device_sessions WHERE id = $1`, id,
	).Scan(&ds.ID, &ds.UserID, &ds.UserAgent, &ds.IPAddress, &ds.DeviceName,
		&ds.CreatedAt, &ds.LastUsedAt, &revokedAt)

	if err == nil {
		if ds.UserID != userID || revokedAt.Valid {
			return r.create(ctx, ulid.Make().String(), userID, userAgent, ipAddress, deviceName)
		}
		_, err = r.db.ExecContext(ctx,
			`UPDATE device_sessions SET user_agent = $1, ip_address = $2, last_used_at = now() WHERE id = $3`,
			userAgent, ipAddress, ds.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("update device session: %w", err)
		}
		ds.UserAgent = userAgent
		ds.IPAddress = ipAddress
		ds.LastUsedAt = time.Now().UTC()
		return &ds, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("lookup device session: %w", err)
	}
	return r.create(ctx, id, userID, userAgent, ipAddress, deviceName)
}

func (r *DeviceSessionRepository) create(
	ctx context.Context, id, userID, userAgent, ipAddress, deviceName string,
) (*oidcdom.DeviceSession, error) {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO device_sessions (id, user_id, user_agent, ip_address, device_name, created_at, last_used_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		id, userID, userAgent, ipAddress, deviceName, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create device session: %w", err)
	}
	return &oidcdom.DeviceSession{
		ID: id, UserID: userID, UserAgent: userAgent,
		IPAddress: ipAddress, DeviceName: deviceName,
		CreatedAt: now, LastUsedAt: now,
	}, nil
}

func (r *DeviceSessionRepository) RevokeByID(ctx context.Context, id, userID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx,
		`UPDATE device_sessions SET revoked_at = now()
		 WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`, id, userID)
	if err != nil {
		return fmt.Errorf("revoke device session: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("device session %s: %w", id, domerr.ErrNotFound)
	}

	if _, err = tx.ExecContext(ctx,
		`DELETE FROM refresh_tokens WHERE device_session_id = $1`, id,
	); err != nil {
		return fmt.Errorf("delete refresh tokens for device session: %w", err)
	}

	return tx.Commit()
}

func (r *DeviceSessionRepository) ListActiveByUserID(
	ctx context.Context, userID string,
) (sessions []*oidcdom.DeviceSession, err error) {
	rows, err := r.db.QueryxContext(ctx,
		`SELECT id, user_id, user_agent, ip_address, device_name, created_at, last_used_at, revoked_at
		 FROM device_sessions
		 WHERE user_id = $1 AND revoked_at IS NULL
		 ORDER BY last_used_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	for rows.Next() {
		var ds oidcdom.DeviceSession
		var revokedAt sql.NullTime
		if err := rows.Scan(&ds.ID, &ds.UserID, &ds.UserAgent, &ds.IPAddress, &ds.DeviceName,
			&ds.CreatedAt, &ds.LastUsedAt, &revokedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, &ds)
	}
	return sessions, rows.Err()
}

func (r *DeviceSessionRepository) DeleteRevokedBefore(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM device_sessions
		 WHERE revoked_at IS NOT NULL
		   AND revoked_at < $1
		   AND NOT EXISTS (
		       SELECT 1 FROM refresh_tokens
		       WHERE device_session_id = device_sessions.id
		         AND expiration > now()
		   )`, before)
	if err != nil {
		return 0, fmt.Errorf("delete revoked device sessions: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}
