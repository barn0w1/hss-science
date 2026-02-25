package authn

import (
	"context"
	"net/http"
)

// Identity represents the verified claims from an external identity provider.
type Identity struct {
	Provider      string
	ExternalSub   string
	Email         string
	EmailVerified bool
	GivenName     string
	FamilyName    string
	Picture       string
	Locale        string
}

// AuthnProvider abstracts an upstream authentication method.
// Implementations handle redirect URL generation and callback processing.
type AuthnProvider interface {
	// Name returns the provider identifier (e.g. "google").
	Name() string

	// AuthURL returns the URL to redirect the user to for authentication.
	// The state parameter is opaquely forwarded to the callback.
	AuthURL(state string) string

	// HandleCallback processes the OAuth2/OIDC callback from the upstream provider.
	// It exchanges the authorization code, verifies the ID token, and returns the identity.
	HandleCallback(ctx context.Context, r *http.Request) (*Identity, error)
}
