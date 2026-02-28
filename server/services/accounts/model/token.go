package model

import "time"

type Token struct {
	ID             string    `db:"id"`
	ClientID       string    `db:"client_id"`
	Subject        string    `db:"subject"`
	Audience       []string  `db:"audience"`
	Scopes         []string  `db:"scopes"`
	Expiration     time.Time `db:"expiration"`
	RefreshTokenID string    `db:"refresh_token_id"`
	CreatedAt      time.Time `db:"created_at"`
}

type RefreshToken struct {
	ID            string    `db:"id"`
	Token         string    `db:"token"`
	ClientID      string    `db:"client_id"`
	UserID        string    `db:"user_id"`
	Audience      []string  `db:"audience"`
	Scopes        []string  `db:"scopes"`
	AuthTime      time.Time `db:"auth_time"`
	AMR           []string  `db:"amr"`
	AccessTokenID string    `db:"access_token_id"`
	Expiration    time.Time `db:"expiration"`
	CreatedAt     time.Time `db:"created_at"`
}
