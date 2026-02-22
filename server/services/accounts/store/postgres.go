package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/barn0w1/hss-science/server/services/accounts/domain"
)

// querier abstracts pgxpool.Pool and pgx.Tx for shared query functions.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// UserStore implements domain.UserRepository using PostgreSQL.
type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

func (s *UserStore) UpsertByProvider(ctx context.Context, account *domain.ExternalAccount, displayName, avatarURL string) (*domain.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Check if an external account with this provider+provider_user_id exists.
	var existingUserID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT user_id FROM external_accounts
		 WHERE provider = $1 AND provider_user_id = $2`,
		account.Provider, account.ProviderUserID,
	).Scan(&existingUserID)

	var userID uuid.UUID

	if errors.Is(err, pgx.ErrNoRows) {
		// New user: create user row first.
		err = tx.QueryRow(ctx,
			`INSERT INTO users (display_name, avatar_url)
			 VALUES ($1, $2)
			 RETURNING id`,
			displayName, avatarURL,
		).Scan(&userID)
		if err != nil {
			return nil, err
		}

		// Create the external account link.
		_, err = tx.Exec(ctx,
			`INSERT INTO external_accounts
			 (user_id, provider, provider_user_id, provider_username, provider_email,
			  access_token, refresh_token, token_expiry)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			userID, account.Provider, account.ProviderUserID,
			account.ProviderUsername, account.ProviderEmail,
			account.AccessToken, account.RefreshToken, account.TokenExpiry,
		)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		// Existing user: update profile and tokens.
		userID = existingUserID

		_, err = tx.Exec(ctx,
			`UPDATE users SET display_name = $1, avatar_url = $2, updated_at = NOW()
			 WHERE id = $3`,
			displayName, avatarURL, userID,
		)
		if err != nil {
			return nil, err
		}

		_, err = tx.Exec(ctx,
			`UPDATE external_accounts
			 SET provider_username = $1, provider_email = $2,
			     access_token = $3, refresh_token = $4, token_expiry = $5,
			     updated_at = NOW()
			 WHERE provider = $6 AND provider_user_id = $7`,
			account.ProviderUsername, account.ProviderEmail,
			account.AccessToken, account.RefreshToken, account.TokenExpiry,
			account.Provider, account.ProviderUserID,
		)
		if err != nil {
			return nil, err
		}
	}

	// Fetch the full user row.
	user, err := getUserByID(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserStore) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return getUserByID(ctx, s.pool, id)
}

func getUserByID(ctx context.Context, q querier, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	err := q.QueryRow(ctx,
		`SELECT id, display_name, avatar_url, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.DisplayName, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// StateStore implements domain.StateRepository using PostgreSQL.
type StateStore struct {
	pool *pgxpool.Pool
}

func NewStateStore(pool *pgxpool.Pool) *StateStore {
	return &StateStore{pool: pool}
}

func (s *StateStore) Create(ctx context.Context, state *domain.OAuthState) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO oauth_states (state, provider, redirect_uri, client_state, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		state.State, state.Provider, state.RedirectURI, state.ClientState, state.ExpiresAt,
	)
	return err
}

func (s *StateStore) Consume(ctx context.Context, stateValue string) (*domain.OAuthState, error) {
	var st domain.OAuthState
	err := s.pool.QueryRow(ctx,
		`DELETE FROM oauth_states
		 WHERE state = $1 AND expires_at > NOW()
		 RETURNING state, provider, redirect_uri, client_state, expires_at, created_at`,
		stateValue,
	).Scan(&st.State, &st.Provider, &st.RedirectURI, &st.ClientState, &st.ExpiresAt, &st.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrStateNotFound
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *StateStore) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM oauth_states WHERE expires_at <= NOW()`,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// AuthCodeStore implements domain.AuthCodeRepository using PostgreSQL.
type AuthCodeStore struct {
	pool *pgxpool.Pool
}

func NewAuthCodeStore(pool *pgxpool.Pool) *AuthCodeStore {
	return &AuthCodeStore{pool: pool}
}

func (s *AuthCodeStore) Create(ctx context.Context, code *domain.AuthCode) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO auth_codes (code, user_id, expires_at)
		 VALUES ($1, $2, $3)`,
		code.Code, code.UserID, code.ExpiresAt,
	)
	return err
}

func (s *AuthCodeStore) Consume(ctx context.Context, codeValue string) (*domain.AuthCode, error) {
	var ac domain.AuthCode
	err := s.pool.QueryRow(ctx,
		`UPDATE auth_codes
		 SET used = TRUE
		 WHERE code = $1 AND used = FALSE AND expires_at > NOW()
		 RETURNING code, user_id, used, expires_at, created_at`,
		codeValue,
	).Scan(&ac.Code, &ac.UserID, &ac.Used, &ac.ExpiresAt, &ac.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrAuthCodeNotFound
	}
	if err != nil {
		return nil, err
	}
	return &ac, nil
}

func (s *AuthCodeStore) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM auth_codes WHERE expires_at <= NOW()`,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Compile-time interface checks.
var (
	_ domain.UserRepository     = (*UserStore)(nil)
	_ domain.StateRepository    = (*StateStore)(nil)
	_ domain.AuthCodeRepository = (*AuthCodeStore)(nil)
)
