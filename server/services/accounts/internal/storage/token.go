package storage

import (
	"time"

	"github.com/lib/pq"
)

// Token represents an access token stored in PostgreSQL.
type Token struct {
	ID        string         `db:"id"`
	ClientID  string         `db:"client_id"`
	Subject   string         `db:"subject"`
	Audience  pq.StringArray `db:"audience"`
	Scopes    pq.StringArray `db:"scopes"`
	ExpiresAt time.Time      `db:"expires_at"`
	CreatedAt time.Time      `db:"created_at"`
}

// RefreshToken represents a refresh token stored in PostgreSQL.
type RefreshToken struct {
	ID            string         `db:"id"`
	TokenValue    string         `db:"token"`
	ClientID      string         `db:"client_id"`
	UserID        string         `db:"user_id"`
	Audience      pq.StringArray `db:"audience"`
	Scopes        pq.StringArray `db:"scopes"`
	AuthTime      time.Time      `db:"auth_time"`
	AMR           pq.StringArray `db:"amr"`
	AccessTokenID *string        `db:"access_token_id"`
	ExpiresAt     time.Time      `db:"expires_at"`
	CreatedAt     time.Time      `db:"created_at"`
}

// RefreshTokenRequest wraps RefreshToken to implement the op.RefreshTokenRequest interface.
type RefreshTokenRequest struct {
	*RefreshToken
}

func (r *RefreshTokenRequest) GetAMR() []string {
	return []string(r.AMR)
}

func (r *RefreshTokenRequest) GetAudience() []string {
	return []string(r.Audience)
}

func (r *RefreshTokenRequest) GetAuthTime() time.Time {
	return r.AuthTime
}

func (r *RefreshTokenRequest) GetClientID() string {
	return r.ClientID
}

func (r *RefreshTokenRequest) GetScopes() []string {
	return []string(r.Scopes)
}

func (r *RefreshTokenRequest) GetSubject() string {
	return r.UserID
}

func (r *RefreshTokenRequest) SetCurrentScopes(scopes []string) {
	r.Scopes = pq.StringArray(scopes)
}
