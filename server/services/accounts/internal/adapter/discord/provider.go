package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain"
	"golang.org/x/oauth2"
)

const (
	discordAPIBase = "https://discord.com/api/v10"
	providerName   = "discord"
)

// Provider implements domain.OAuthProvider for Discord.
type Provider struct {
	cfg *oauth2.Config
}

// NewProvider creates a new Discord OAuth provider.
func NewProvider(clientID, clientSecret, redirectURL string) *Provider {
	return &Provider{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"identify", "email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://discord.com/api/oauth2/authorize",
				TokenURL: "https://discord.com/api/oauth2/token",
			},
		},
	}
}

func (p *Provider) Name() string { return providerName }

func (p *Provider) AuthURL(state string) string {
	return p.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *Provider) Exchange(ctx context.Context, code string) (*domain.ProviderUserInfo, string, string, error) {
	token, err := p.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, "", "", fmt.Errorf("discord token exchange: %w", err)
	}

	userInfo, err := fetchDiscordUser(ctx, token.AccessToken)
	if err != nil {
		return nil, "", "", err
	}

	return userInfo, token.AccessToken, token.RefreshToken, nil
}

type discordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	GlobalName    string `json:"global_name"`
	Avatar        string `json:"avatar"`
	Email         string `json:"email"`
}

func fetchDiscordUser(ctx context.Context, accessToken string) (*domain.ProviderUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discordAPIBase+"/users/@me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discord user fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord API error (status %d): %s", resp.StatusCode, body)
	}

	var u discordUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("discord user decode: %w", err)
	}

	displayName := u.GlobalName
	if displayName == "" {
		displayName = u.Username
	}

	avatarURL := ""
	if u.Avatar != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", u.ID, u.Avatar)
	}

	return &domain.ProviderUserInfo{
		ProviderID:  u.ID,
		Email:       u.Email,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
	}, nil
}
