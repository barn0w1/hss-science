package model

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
