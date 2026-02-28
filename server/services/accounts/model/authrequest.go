package model

import "time"

type AuthRequest struct {
	ID                  string    `db:"id"`
	ClientID            string    `db:"client_id"`
	RedirectURI         string    `db:"redirect_uri"`
	State               string    `db:"state"`
	Nonce               string    `db:"nonce"`
	Scopes              []string  `db:"scopes"`
	ResponseType        string    `db:"response_type"`
	ResponseMode        string    `db:"response_mode"`
	CodeChallenge       string    `db:"code_challenge"`
	CodeChallengeMethod string    `db:"code_challenge_method"`
	Prompt              []string  `db:"prompt"`
	MaxAge              *int64    `db:"max_age"`
	LoginHint           string    `db:"login_hint"`
	UserID              string    `db:"user_id"`
	AuthTime            time.Time `db:"auth_time"`
	AMR                 []string  `db:"amr"`
	IsDone              bool      `db:"is_done"`
	Code                string    `db:"code"`
	CreatedAt           time.Time `db:"created_at"`
}
