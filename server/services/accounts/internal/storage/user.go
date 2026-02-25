package storage

import (
	"time"

	"github.com/zitadel/oidc/v3/pkg/oidc"
	"golang.org/x/text/language"
)

// User represents an internal user identity in PostgreSQL.
type User struct {
	ID            string    `db:"id"`
	Email         string    `db:"email"`
	EmailVerified bool      `db:"email_verified"`
	GivenName     string    `db:"given_name"`
	FamilyName    string    `db:"family_name"`
	Picture       string    `db:"picture"`
	Locale        string    `db:"locale"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

// UserProfile holds profile data from an upstream identity provider,
// used during user provisioning.
type UserProfile struct {
	Email         string
	EmailVerified bool
	GivenName     string
	FamilyName    string
	Picture       string
	Locale        string
}

// FederatedIdentity maps an external provider identity to an internal user.
type FederatedIdentity struct {
	ID          string    `db:"id"`
	UserID      string    `db:"user_id"`
	Provider    string    `db:"provider"`
	ExternalSub string    `db:"external_sub"`
	CreatedAt   time.Time `db:"created_at"`
}

// setUserinfo populates an OIDC UserInfo response based on requested scopes.
func setUserinfo(user *User, userinfo *oidc.UserInfo, scopes []string) {
	for _, scope := range scopes {
		switch scope {
		case oidc.ScopeOpenID:
			userinfo.Subject = user.ID
		case oidc.ScopeEmail:
			userinfo.Email = user.Email
			userinfo.EmailVerified = oidc.Bool(user.EmailVerified)
		case oidc.ScopeProfile:
			userinfo.GivenName = user.GivenName
			userinfo.FamilyName = user.FamilyName
			userinfo.Name = user.GivenName + " " + user.FamilyName
			userinfo.Picture = user.Picture
			userinfo.Locale = oidc.NewLocale(language.Make(user.Locale))
			userinfo.PreferredUsername = user.Email
		}
	}
}
