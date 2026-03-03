package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

var _ identity.Repository = (*UserRepository)(nil)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

type userRow struct {
	ID            string    `db:"id"`
	Email         string    `db:"email"`
	EmailVerified bool      `db:"email_verified"`
	Name          string    `db:"name"`
	GivenName     string    `db:"given_name"`
	FamilyName    string    `db:"family_name"`
	Picture       string    `db:"picture"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

func toUser(row userRow) *identity.User {
	return &identity.User{
		ID:            row.ID,
		Email:         row.Email,
		EmailVerified: row.EmailVerified,
		Name:          row.Name,
		GivenName:     row.GivenName,
		FamilyName:    row.FamilyName,
		Picture:       row.Picture,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*identity.User, error) {
	var row userRow
	err := r.db.QueryRowxContext(ctx,
		`SELECT id, email, email_verified, name, given_name, family_name, picture, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).StructScan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domerr.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toUser(row), nil
}

func (r *UserRepository) FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*identity.User, error) {
	var row userRow
	err := r.db.QueryRowxContext(ctx,
		`SELECT u.id, u.email, u.email_verified, u.name, u.given_name, u.family_name, u.picture, u.created_at, u.updated_at
		 FROM users u
		 JOIN federated_identities fi ON fi.user_id = u.id
		 WHERE fi.provider = $1 AND fi.provider_subject = $2`,
		provider, providerSubject,
	).StructScan(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toUser(row), nil
}

func (r *UserRepository) CreateWithFederatedIdentity(
	ctx context.Context,
	user *identity.User,
	fi *identity.FederatedIdentity,
) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO users (id, email, email_verified, name, given_name, family_name, picture, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		user.ID, user.Email, user.EmailVerified, user.Name,
		user.GivenName, user.FamilyName, user.Picture, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO federated_identities
		    (id, user_id, provider, provider_subject,
		     provider_email, provider_email_verified,
		     provider_display_name, provider_given_name, provider_family_name, provider_picture_url,
		     last_login_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		fi.ID, fi.UserID, fi.Provider, fi.ProviderSubject,
		fi.ProviderEmail, fi.ProviderEmailVerified,
		fi.ProviderDisplayName, fi.ProviderGivenName, fi.ProviderFamilyName, fi.ProviderPictureURL,
		fi.LastLoginAt, fi.CreatedAt, fi.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *UserRepository) UpdateUserFromClaims(ctx context.Context, userID string, claims identity.FederatedClaims, updatedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users
		 SET email          = $1,
		     email_verified = $2,
		     name           = $3,
		     given_name     = $4,
		     family_name    = $5,
		     picture        = $6,
		     updated_at     = $7
		 WHERE id = $8`,
		claims.Email, claims.EmailVerified, claims.Name,
		claims.GivenName, claims.FamilyName, claims.Picture,
		updatedAt, userID,
	)
	return err
}

func (r *UserRepository) UpdateFederatedIdentityClaims(
	ctx context.Context,
	provider, providerSubject string,
	claims identity.FederatedClaims,
	lastLoginAt time.Time,
) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE federated_identities
		 SET provider_email          = $1,
		     provider_email_verified = $2,
		     provider_display_name   = $3,
		     provider_given_name     = $4,
		     provider_family_name    = $5,
		     provider_picture_url    = $6,
		     last_login_at           = $7,
		     updated_at              = now()
		 WHERE provider = $8 AND provider_subject = $9`,
		claims.Email, claims.EmailVerified, claims.Name,
		claims.GivenName, claims.FamilyName, claims.Picture,
		lastLoginAt, provider, providerSubject,
	)
	return err
}
