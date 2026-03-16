package identity

import (
	"context"
	"time"
)

type Repository interface {
	GetByID(ctx context.Context, id string) (*User, error)
	FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*User, error)
	CreateWithFederatedIdentity(ctx context.Context, user *User, fi *FederatedIdentity) error
	UpdateUserFromClaims(ctx context.Context, userID string, claims FederatedClaims, updatedAt time.Time) error
	UpdateFederatedIdentityClaims(
		ctx context.Context,
		provider, providerSubject string,
		claims FederatedClaims,
		lastLoginAt time.Time,
	) error

	ListFederatedIdentities(ctx context.Context, userID string) ([]*FederatedIdentity, error)
	DeleteFederatedIdentity(ctx context.Context, id, userID string) error
	UpdateLocalProfile(ctx context.Context, userID string, name, picture *string, updatedAt time.Time) error
}

type Service interface {
	GetUser(ctx context.Context, userID string) (*User, error)
	FindOrCreateByFederatedLogin(ctx context.Context, provider string, claims FederatedClaims) (*User, error)

	UpdateProfile(ctx context.Context, userID string, name, picture *string) (*User, error)
	ListLinkedProviders(ctx context.Context, userID string) ([]*FederatedIdentity, error)
	UnlinkProvider(ctx context.Context, userID, identityID string) error
}
