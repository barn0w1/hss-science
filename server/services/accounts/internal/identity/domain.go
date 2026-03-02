package identity

import "time"

type User struct {
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

type FederatedIdentity struct {
	ID              string    `db:"id"`
	UserID          string    `db:"user_id"`
	Provider        string    `db:"provider"`
	ProviderSubject string    `db:"provider_subject"`
	CreatedAt       time.Time `db:"created_at"`
}
