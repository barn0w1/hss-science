package domain

import "time"

type User struct {
	ID        string
	Username  string
	AvatarURL string
	Role      string
	CreatedAt time.Time
}
