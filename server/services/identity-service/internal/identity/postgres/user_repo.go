package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/barn0w1/hss-science/server/services/identity-service/internal/identity"
	"github.com/barn0w1/hss-science/server/services/identity-service/internal/pkg/domerr"
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
	LocalName     *string   `db:"local_name"`
	LocalPicture  *string   `db:"local_picture"`
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
		LocalName:     row.LocalName,
		LocalPicture:  row.LocalPicture,
	}
}

type fiRow struct {
	ID                    string    `db:"id"`
	UserID                string    `db:"user_id"`
	Provider              string    `db:"provider"`
	ProviderSubject       string    `db:"provider_subject"`
	ProviderEmail         string    `db:"provider_email"`
	ProviderEmailVerified bool      `db:"provider_email_verified"`
	ProviderDisplayName   string    `db:"provider_display_name"`
	ProviderGivenName     string    `db:"provider_given_name"`
	ProviderFamilyName    string    `db:"provider_family_name"`
	ProviderPictureURL    string    `db:"provider_picture_url"`
	LastLoginAt           time.Time `db:"last_login_at"`
	CreatedAt             time.Time `db:"created_at"`
	UpdatedAt             time.Time `db:"updated_at"`
}

func toFederatedIdentity(row fiRow) *identity.FederatedIdentity {
	return &identity.FederatedIdentity{
		ID:                    row.ID,
		UserID:                row.UserID,
		Provider:              row.Provider,
		ProviderSubject:       row.ProviderSubject,
		ProviderEmail:         row.ProviderEmail,
		ProviderEmailVerified: row.ProviderEmailVerified,
		ProviderDisplayName:   row.ProviderDisplayName,
		ProviderGivenName:     row.ProviderGivenName,
		ProviderFamilyName:    row.ProviderFamilyName,
		ProviderPictureURL:    row.ProviderPictureURL,
		LastLoginAt:           row.LastLoginAt,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*identity.User, error) {
	var row userRow
	err := r.db.QueryRowxContext(ctx,
		`SELECT id, email, email_verified, name, given_name, family_name, picture,
		        created_at, updated_at, local_name, local_picture
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
		`SELECT u.id, u.email, u.email_verified, u.name, u.given_name, u.family_name, u.picture,
		        u.created_at, u.updated_at, u.local_name, u.local_picture
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
		     name           = CASE WHEN local_name    IS NULL THEN $3 ELSE name    END,
		     given_name     = $4,
		     family_name    = $5,
		     picture        = CASE WHEN local_picture IS NULL THEN $6 ELSE picture END,
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

func (r *UserRepository) ListFederatedIdentities(ctx context.Context, userID string) ([]*identity.FederatedIdentity, error) {
	var rows []fiRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, user_id, provider, provider_subject,
		        provider_email, provider_email_verified,
		        provider_display_name, provider_given_name, provider_family_name,
		        provider_picture_url, last_login_at, created_at, updated_at
		 FROM federated_identities
		 WHERE user_id = $1
		 ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	result := make([]*identity.FederatedIdentity, len(rows))
	for i, row := range rows {
		result[i] = toFederatedIdentity(row)
	}
	return result, nil
}

func (r *UserRepository) DeleteFederatedIdentity(ctx context.Context, id, userID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRowxContext(ctx,
		`SELECT COUNT(*) FROM federated_identities WHERE user_id = $1 FOR UPDATE`, userID,
	).Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return domerr.ErrFailedPrecondition
	}

	res, err := tx.ExecContext(ctx,
		`DELETE FROM federated_identities WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domerr.ErrNotFound
	}

	return tx.Commit()
}

func (r *UserRepository) UpdateLocalProfile(
	ctx context.Context,
	userID string,
	name, picture *string,
	updatedAt time.Time,
) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users
		 SET local_name    = CASE WHEN $2 THEN $3 ELSE local_name    END,
		     local_picture = CASE WHEN $4 THEN $5 ELSE local_picture END,
		     updated_at    = $6
		 WHERE id = $1`,
		userID,
		name != nil, nullableString(name),
		picture != nil, nullableString(picture),
		updatedAt,
	)
	return err
}

func nullableString(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
