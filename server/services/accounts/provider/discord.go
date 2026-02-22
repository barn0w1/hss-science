package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

const (
	discordAuthURL    = "https://discord.com/oauth2/authorize"
	discordTokenURL   = "https://discord.com/api/v10/oauth2/token"
	discordUserAPIURL = "https://discord.com/api/v10/users/@me"
)

// Discord implements OAuthProvider for Discord's OAuth2 flow.
type Discord struct {
	config *oauth2.Config
	client *http.Client
}

// DiscordConfig holds the configuration for the Discord OAuth provider.
type DiscordConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // The Accounts BFF callback URL
}

// NewDiscord creates a Discord OAuth provider.
func NewDiscord(cfg DiscordConfig) *Discord {
	return &Discord{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"identify", "email"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  discordAuthURL,
				TokenURL: discordTokenURL,
			},
		},
		client: http.DefaultClient,
	}
}

func (d *Discord) Name() string {
	return "discord"
}

func (d *Discord) AuthCodeURL(state string) string {
	return d.config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (d *Discord) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return d.config.Exchange(ctx, code)
}

func (d *Discord) FetchUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discordUserAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("discord: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discord: fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord: unexpected status %d: %s", resp.StatusCode, body)
	}

	var du discordUser
	if err := json.NewDecoder(resp.Body).Decode(&du); err != nil {
		return nil, fmt.Errorf("discord: decode response: %w", err)
	}

	return du.toUserInfo(), nil
}

// discordUser maps the Discord API /users/@me response.
type discordUser struct {
	ID            string  `json:"id"`
	Username      string  `json:"username"`
	GlobalName    *string `json:"global_name"`
	Email         *string `json:"email"`
	Avatar        *string `json:"avatar"`
	Discriminator string  `json:"discriminator"`
}

func (du *discordUser) toUserInfo() *UserInfo {
	displayName := du.Username
	if du.GlobalName != nil && *du.GlobalName != "" {
		displayName = *du.GlobalName
	}

	avatarURL := ""
	if du.Avatar != nil {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", du.ID, *du.Avatar)
	}

	return &UserInfo{
		ProviderUserID: du.ID,
		Username:       du.Username,
		Email:          du.Email,
		DisplayName:    displayName,
		AvatarURL:      avatarURL,
	}
}

// Compile-time interface check.
var _ OAuthProvider = (*Discord)(nil)
