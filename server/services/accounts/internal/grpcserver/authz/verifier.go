package authz

import "context"

// Claims holds the verified JWT claims relevant for authorization.
type Claims struct {
	Subject  string
	Audience []string
}

// TokenVerifier verifies an access token and returns its claims.
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (*Claims, error)
}
