package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/config"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/repository"
	"golang.org/x/oauth2"
)

// Discord Endpoint Constants
const (
	discordAuthURL  = "https://discord.com/api/oauth2/authorize"
	discordTokenURL = "https://discord.com/api/oauth2/token"
	discordUserURL  = "https://discord.com/api/users/@me"
)

type discordProvider struct {
	oauthConfig *oauth2.Config
}

// NewDiscordProvider creates a new OAuthProvider for Discord.
func NewDiscordProvider(cfg *config.Config) repository.OAuthProvider {
	return &discordProvider{
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.DiscordClientID,
			ClientSecret: cfg.DiscordClientSecret,
			RedirectURL:  cfg.DiscordRedirectURL,
			Scopes:       []string{"identify"}, // Request basic user info
			Endpoint: oauth2.Endpoint{
				AuthURL:  discordAuthURL,
				TokenURL: discordTokenURL,
			},
		},
	}
}

// discordUserResponse maps the JSON response from Discord API.
type discordUserResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"` // nullable but handled as empty string if null
	// Discriminator is deprecated but might be useful if strictly needed
}

func (p *discordProvider) GetAuthURL(redirectURL, state string) string {
	// Use AuthCodeOption to dynamically override the Redirect URL
	// This allows support for both localhost and production domains
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("redirect_uri", redirectURL),
	}
	return p.oauthConfig.AuthCodeURL(state, opts...)
}

// GetUserInfo exchanges the auth code for a token and retrieves user info.
func (p *discordProvider) GetUserInfo(ctx context.Context, code string) (*repository.OAuthUserInfo, error) {
	// 1. Exchange Code for Token
	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// 2. Fetch User Info using the token
	// oauth2 library's Client automatically adds "Authorization: Bearer <token>" header
	client := p.oauthConfig.Client(ctx, token)
	resp, err := client.Get(discordUserURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info from discord: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("discord api returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	// 3. Parse Response
	var dUser discordUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&dUser); err != nil {
		return nil, fmt.Errorf("failed to decode discord user response: %w", err)
	}

	// 4. Construct Avatar URL
	// Format: https://cdn.discordapp.com/avatars/{user_id}/{avatar_hash}.png
	var avatarURL string
	if dUser.Avatar != "" {
		avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", dUser.ID, dUser.Avatar)
	} else {
		// Default avatar if none set (logic varies, but empty is safe for now)
		// Or construct default avatar based on discriminator logic if highly detailed
		avatarURL = ""
	}

	return &repository.OAuthUserInfo{
		DiscordID: dUser.ID,
		Name:      dUser.Username,
		AvatarURL: avatarURL,
	}, nil
}
