package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

var _ Service = (*identityService)(nil)

type identityService struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &identityService{repo: repo}
}

func (s *identityService) GetUser(ctx context.Context, userID string) (*User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("identity.GetUser(%s): %w", userID, err)
	}
	return user, nil
}

func (s *identityService) FindOrCreateByFederatedLogin(
	ctx context.Context,
	provider string,
	claims FederatedClaims,
) (*User, error) {
	existing, err := s.repo.FindByFederatedIdentity(ctx, provider, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("identity.FindOrCreate: lookup: %w", err)
	}

	if existing != nil {
		now := time.Now().UTC()
		if err := s.repo.UpdateFederatedIdentityClaims(ctx, provider, claims.Subject, claims, now); err != nil {
			return nil, fmt.Errorf("identity.FindOrCreate: update claims: %w", err)
		}
		return existing, nil
	}

	now := time.Now().UTC()
	user := &User{
		ID:            newID(),
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Picture:       claims.Picture,
		CreatedAt:     now,
	}
	fi := &FederatedIdentity{
		ID:                    newID(),
		UserID:                user.ID,
		Provider:              provider,
		ProviderSubject:       claims.Subject,
		ProviderEmail:         claims.Email,
		ProviderEmailVerified: claims.EmailVerified,
		ProviderDisplayName:   claims.Name,
		ProviderGivenName:     claims.GivenName,
		ProviderFamilyName:    claims.FamilyName,
		ProviderPictureURL:    claims.Picture,
		LastLoginAt:           now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := s.repo.CreateWithFederatedIdentity(ctx, user, fi); err != nil {
		return nil, fmt.Errorf("identity.FindOrCreate: create: %w", err)
	}
	return user, nil
}

func newID() string {
	return ulid.Make().String()
}
