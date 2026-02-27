package model

import "time"

type FederatedIdentity struct {
	ID              string    `db:"id"`
	UserID          string    `db:"user_id"`
	Provider        string    `db:"provider"`
	ProviderSubject string    `db:"provider_subject"`
	CreatedAt       time.Time `db:"created_at"`
}
