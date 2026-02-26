package handler

import (
	"time"

	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

// ProfileResponse is the REST representation of a user profile.
type ProfileResponse struct {
	UserID        string `json:"user_id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// LinkedAccountResponse is the REST representation of a linked account.
type LinkedAccountResponse struct {
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	ExternalSub string `json:"external_sub"`
	LinkedAt    string `json:"linked_at"`
}

// SessionResponse is the REST representation of an active session.
type SessionResponse struct {
	SessionID string   `json:"session_id"`
	ClientID  string   `json:"client_id"`
	Scopes    []string `json:"scopes"`
	AuthTime  string   `json:"auth_time"`
	ExpiresAt string   `json:"expires_at"`
	CreatedAt string   `json:"created_at"`
}

func profileToREST(p *pb.Profile) *ProfileResponse {
	return &ProfileResponse{
		UserID:        p.UserId,
		Email:         p.Email,
		EmailVerified: p.EmailVerified,
		GivenName:     p.GivenName,
		FamilyName:    p.FamilyName,
		Picture:       p.Picture,
		Locale:        p.Locale,
		CreatedAt:     p.CreatedAt.AsTime().Format(time.RFC3339),
		UpdatedAt:     p.UpdatedAt.AsTime().Format(time.RFC3339),
	}
}

func linkedAccountToREST(la *pb.LinkedAccount) *LinkedAccountResponse {
	return &LinkedAccountResponse{
		ID:          la.Id,
		Provider:    la.Provider,
		ExternalSub: la.ExternalSub,
		LinkedAt:    la.LinkedAt.AsTime().Format(time.RFC3339),
	}
}

func sessionToREST(s *pb.Session) *SessionResponse {
	return &SessionResponse{
		SessionID: s.SessionId,
		ClientID:  s.ClientId,
		Scopes:    s.Scopes,
		AuthTime:  s.AuthTime.AsTime().Format(time.RFC3339),
		ExpiresAt: s.ExpiresAt.AsTime().Format(time.RFC3339),
		CreatedAt: s.CreatedAt.AsTime().Format(time.RFC3339),
	}
}
