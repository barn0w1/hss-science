package identity

import "time"

type User struct {
	ID            string
	Email         string
	EmailVerified bool
	Name          string
	GivenName     string
	FamilyName    string
	Picture       string
	CreatedAt     time.Time
}

type FederatedIdentity struct {
	ID              string
	UserID          string
	Provider        string
	ProviderSubject string

	ProviderEmail         string
	ProviderEmailVerified bool
	ProviderDisplayName   string
	ProviderGivenName     string
	ProviderFamilyName    string
	ProviderPictureURL    string

	LastLoginAt time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type FederatedClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	GivenName     string
	FamilyName    string
	Picture       string
}
