package storage

import (
	"time"

	"github.com/lib/pq"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// AuthRequest represents a pending OIDC authorization request stored in PostgreSQL.
// It implements the op.AuthRequest interface.
type AuthRequest struct {
	ID                  string         `db:"id"`
	ClientID            string         `db:"client_id"`
	RedirectURI         string         `db:"redirect_uri"`
	State               string         `db:"state"`
	Nonce               string         `db:"nonce"`
	Scopes              pq.StringArray `db:"scopes"`
	ResponseTypeStr     string         `db:"response_type"`
	ResponseModeStr     string         `db:"response_mode"`
	CodeChallengeVal    string         `db:"code_challenge"`
	CodeChallengeMethod string         `db:"code_challenge_method"`
	Prompt              pq.StringArray `db:"prompt"`
	LoginHint           string         `db:"login_hint"`
	MaxAgeSeconds       *int           `db:"max_age_seconds"`
	UserID              *string        `db:"user_id"`
	IsDone              bool           `db:"done"`
	AuthTime            *time.Time     `db:"auth_time"`
	CreatedAt           time.Time      `db:"created_at"`
}

func (a *AuthRequest) GetID() string {
	return a.ID
}

func (a *AuthRequest) GetACR() string {
	return ""
}

func (a *AuthRequest) GetAMR() []string {
	if a.IsDone {
		return []string{"fed"}
	}
	return nil
}

func (a *AuthRequest) GetAudience() []string {
	return []string{a.ClientID}
}

func (a *AuthRequest) GetAuthTime() time.Time {
	if a.AuthTime != nil {
		return *a.AuthTime
	}
	return time.Time{}
}

func (a *AuthRequest) GetClientID() string {
	return a.ClientID
}

func (a *AuthRequest) GetCodeChallenge() *oidc.CodeChallenge {
	if a.CodeChallengeVal == "" {
		return nil
	}
	method := oidc.CodeChallengeMethodPlain
	if a.CodeChallengeMethod == "S256" {
		method = oidc.CodeChallengeMethodS256
	}
	return &oidc.CodeChallenge{
		Challenge: a.CodeChallengeVal,
		Method:    method,
	}
}

func (a *AuthRequest) GetNonce() string {
	return a.Nonce
}

func (a *AuthRequest) GetRedirectURI() string {
	return a.RedirectURI
}

func (a *AuthRequest) GetResponseType() oidc.ResponseType {
	return oidc.ResponseType(a.ResponseTypeStr)
}

func (a *AuthRequest) GetResponseMode() oidc.ResponseMode {
	return oidc.ResponseMode(a.ResponseModeStr)
}

func (a *AuthRequest) GetScopes() []string {
	return []string(a.Scopes)
}

func (a *AuthRequest) GetState() string {
	return a.State
}

func (a *AuthRequest) GetSubject() string {
	if a.UserID != nil {
		return *a.UserID
	}
	return ""
}

func (a *AuthRequest) Done() bool {
	return a.IsDone
}

// authRequestToInternal converts an incoming OIDC authorization request
// to our internal AuthRequest model for database persistence.
func authRequestToInternal(authReq *oidc.AuthRequest, userID string) *AuthRequest {
	req := &AuthRequest{
		ClientID:        authReq.ClientID,
		RedirectURI:     authReq.RedirectURI,
		State:           authReq.State,
		Nonce:           authReq.Nonce,
		Scopes:          pq.StringArray(authReq.Scopes),
		ResponseTypeStr: string(authReq.ResponseType),
		ResponseModeStr: string(authReq.ResponseMode),
		LoginHint:       authReq.LoginHint,
		CreatedAt:       time.Now(),
	}

	if authReq.CodeChallenge != "" {
		req.CodeChallengeVal = authReq.CodeChallenge
		req.CodeChallengeMethod = string(authReq.CodeChallengeMethod)
	}

	// Convert prompt
	prompts := make([]string, 0, len(authReq.Prompt))
	for _, p := range authReq.Prompt {
		switch p {
		case oidc.PromptNone, oidc.PromptLogin, oidc.PromptConsent, oidc.PromptSelectAccount:
			prompts = append(prompts, p)
		}
	}
	req.Prompt = pq.StringArray(prompts)

	// Convert max_age
	if authReq.MaxAge != nil {
		maxAge := int(*authReq.MaxAge)
		req.MaxAgeSeconds = &maxAge
	}

	if userID != "" {
		req.UserID = &userID
	}

	return req
}
