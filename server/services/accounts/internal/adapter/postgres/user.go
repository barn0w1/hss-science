package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain"
)

// UserRepo implements domain.UserRepository using PostgreSQL.
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) UpsertByIdentity(ctx context.Context, provider string, info *domain.ProviderUserInfo, accessToken, refreshToken string) (*domain.User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Try to find an existing identity.
	var userID string
	err = tx.QueryRowContext(ctx,
		`SELECT user_id FROM user_identities WHERE provider = $1 AND provider_id = $2`,
		provider, info.ProviderID,
	).Scan(&userID)

	now := time.Now()

	if err == sql.ErrNoRows {
		// Create a new user.
		err = tx.QueryRowContext(ctx,
			`INSERT INTO users (display_name, avatar_url, created_at, updated_at)
			 VALUES ($1, $2, $3, $3)
			 RETURNING id`,
			info.DisplayName, info.AvatarURL, now,
		).Scan(&userID)
		if err != nil {
			return nil, err
		}

		// Create the identity link.
		_, err = tx.ExecContext(ctx,
			`INSERT INTO user_identities (user_id, provider, provider_id, email, display_name, avatar_url, access_token, refresh_token, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
			userID, provider, info.ProviderID, info.Email, info.DisplayName, info.AvatarURL, accessToken, refreshToken, now,
		)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		// Update existing user profile and identity tokens.
		_, err = tx.ExecContext(ctx,
			`UPDATE users SET display_name = $1, avatar_url = $2, updated_at = $3 WHERE id = $4`,
			info.DisplayName, info.AvatarURL, now, userID,
		)
		if err != nil {
			return nil, err
		}

		_, err = tx.ExecContext(ctx,
			`UPDATE user_identities SET email = $1, display_name = $2, avatar_url = $3, access_token = $4, refresh_token = $5, updated_at = $6
			 WHERE provider = $7 AND provider_id = $8`,
			info.Email, info.DisplayName, info.AvatarURL, accessToken, refreshToken, now, provider, info.ProviderID,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &domain.User{
		ID:          userID,
		DisplayName: info.DisplayName,
		AvatarURL:   info.AvatarURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, display_name, avatar_url, created_at, updated_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	return u, err
}
