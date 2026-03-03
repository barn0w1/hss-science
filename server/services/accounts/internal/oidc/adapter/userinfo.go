package adapter

import (
	"context"
	"time"
)

type UserClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	GivenName     string
	FamilyName    string
	Picture       string
	UpdatedAt     time.Time
}

type UserClaimsSource interface {
	UserClaims(ctx context.Context, userID string) (*UserClaims, error)
}
