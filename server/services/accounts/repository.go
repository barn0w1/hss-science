package accounts

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// PGRepository performs database operations for the Accounts service.
type PGRepository struct {
	db *sqlx.DB
}

// NewPGRepository creates a new PostgreSQL-backed repository.
func NewPGRepository(db *sqlx.DB) *PGRepository {
	return &PGRepository{db: db}
}

// UpsertUser performs JIT provisioning: inserts a new user or updates
// profile fields if the google_id already exists. Returns the internal user ID.
func (r *PGRepository) UpsertUser(ctx context.Context, googleID, email, name, picture string) (string, error) {
	const query = `
		INSERT INTO users (id, google_id, email, name, picture)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (google_id)
		DO UPDATE SET email = EXCLUDED.email,
		              name = EXCLUDED.name,
		              picture = EXCLUDED.picture,
		              updated_at = NOW()
		RETURNING id`

	var userID string
	err := r.db.QueryRowContext(ctx, query, newUserID(), googleID, email, name, picture).Scan(&userID)
	if err != nil {
		return "", err
	}
	return userID, nil
}

// CreateSession inserts a new session row for multi-device support.
// Returns the session ID.
func (r *PGRepository) CreateSession(ctx context.Context, userID, deviceIP, deviceUA string, expiresAt time.Time) (string, error) {
	const query = `
		INSERT INTO sessions (id, user_id, device_ip, device_ua, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`

	var sessionID string
	err := r.db.QueryRowContext(ctx, query, newSessionID(), userID, deviceIP, deviceUA, expiresAt).Scan(&sessionID)
	if err != nil {
		return "", err
	}
	return sessionID, nil
}

func newUserID() string    { return "usr_" + uuid.NewString() }
func newSessionID() string { return "sess_" + uuid.NewString() }
