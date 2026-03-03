package authn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
)

type githubClaimsProvider struct {
	httpClient *http.Client
}

func (g *githubClaimsProvider) FetchClaims(ctx context.Context, token *oauth2.Token) (*identity.FederatedClaims, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	token.SetAuthHeader(req)
	resp, err := g.httpClient.Do(req) //nolint:gosec // URL is a hardcoded constant
	if err != nil {
		return nil, fmt.Errorf("github user API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github user API returned %d: %s", resp.StatusCode, body)
	}
	var ghUser struct {
		ID        int64  `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
		return nil, fmt.Errorf("decoding github user: %w", err)
	}
	return &identity.FederatedClaims{
		Subject: strconv.FormatInt(ghUser.ID, 10),
		Email:   ghUser.Email,
		Name:    ghUser.Name,
		Picture: ghUser.AvatarURL,
	}, nil
}

func newGitHubProvider(clientID, clientSecret, callbackURL string) *Provider {
	oauth2Cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  callbackURL,
		Endpoint:     github.Endpoint,
		Scopes:       []string{"read:user", "user:email"},
	}

	return &Provider{
		Name:         "github",
		DisplayName:  "Sign in with GitHub",
		OAuth2Config: oauth2Cfg,
		Claims: &githubClaimsProvider{
			httpClient: &http.Client{Timeout: 10 * time.Second},
		},
	}
}
