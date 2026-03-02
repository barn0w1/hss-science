package identity

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, u *User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, email_verified, name, given_name, family_name, picture)
         VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		u.ID, u.Email, u.EmailVerified, u.Name, u.GivenName, u.FamilyName, u.Picture,
	)
	return err
}

func (r *Repository) GetByID(ctx context.Context, id string) (*User, error) {
	var u User
	err := r.db.QueryRowxContext(ctx,
		`SELECT id, email, email_verified, name, given_name, family_name, picture, created_at, updated_at
         FROM users WHERE id = $1`, id,
	).StructScan(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*User, error) {
	var u User
	err := r.db.QueryRowxContext(ctx,
		`SELECT u.id, u.email, u.email_verified, u.name, u.given_name, u.family_name, u.picture, u.created_at, u.updated_at
         FROM users u
         JOIN federated_identities fi ON fi.user_id = u.id
         WHERE fi.provider = $1 AND fi.provider_subject = $2`,
		provider, providerSubject,
	).StructScan(&u)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *Repository) CreateWithFederatedIdentity(ctx context.Context, u *User, fi *FederatedIdentity) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO users (id, email, email_verified, name, given_name, family_name, picture)
         VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		u.ID, u.Email, u.EmailVerified, u.Name, u.GivenName, u.FamilyName, u.Picture,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO federated_identities (id, user_id, provider, provider_subject)
         VALUES ($1, $2, $3, $4)`,
		fi.ID, fi.UserID, fi.Provider, fi.ProviderSubject,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}
