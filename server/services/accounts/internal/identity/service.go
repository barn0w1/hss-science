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
	applyLocalOverrides(user)
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
		if err := s.repo.UpdateUserFromClaims(ctx, existing.ID, claims, now); err != nil {
			return nil, fmt.Errorf("identity.FindOrCreate: update user: %w", err)
		}
		if err := s.repo.UpdateFederatedIdentityClaims(ctx, provider, claims.Subject, claims, now); err != nil {
			return nil, fmt.Errorf("identity.FindOrCreate: update claims: %w", err)
		}
		existing.Email = claims.Email
		existing.EmailVerified = claims.EmailVerified
		existing.Name = claims.Name
		existing.GivenName = claims.GivenName
		existing.FamilyName = claims.FamilyName
		existing.Picture = claims.Picture
		existing.UpdatedAt = now
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
		UpdatedAt:     now,
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

func (s *identityService) UpdateProfile(
	ctx context.Context, userID string, name, picture *string,
) (*User, error) {
	now := time.Now().UTC()
	if err := s.repo.UpdateLocalProfile(ctx, userID, name, picture, now); err != nil {
		return nil, fmt.Errorf("identity.UpdateProfile: %w", err)
	}
	return s.GetUser(ctx, userID)
}

func (s *identityService) ListLinkedProviders(
	ctx context.Context, userID string,
) ([]*FederatedIdentity, error) {
	fis, err := s.repo.ListFederatedIdentities(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("identity.ListLinkedProviders: %w", err)
	}
	return fis, nil
}

func (s *identityService) UnlinkProvider(
	ctx context.Context, userID, identityID string,
) error {
	if err := s.repo.DeleteFederatedIdentity(ctx, identityID, userID); err != nil {
		return fmt.Errorf("identity.UnlinkProvider: %w", err)
	}
	return nil
}

func applyLocalOverrides(u *User) {
	if u.LocalName != nil && *u.LocalName != "" {
		u.Name = *u.LocalName
	}
	if u.LocalPicture != nil && *u.LocalPicture != "" {
		u.Picture = *u.LocalPicture
	}
}

func newID() string {
	return ulid.Make().String()
}
