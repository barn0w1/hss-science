package model

import (
	"time"

	"github.com/google/uuid"
)

type GlobalRole string

const (
	RoleSystemAdmin GlobalRole = "system_admin"
	RoleModerator   GlobalRole = "moderator"
	RoleUser        GlobalRole = "user"
)

// User represents a registered user identity in the system.
type User struct {
	ID        uuid.UUID
	DiscordID string // immutable

	Name      string
	AvatarURL string
	Role      GlobalRole

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewUser(discordID, name, avatarURL string) *User {
	return &User{
		ID:        uuid.New(),
		DiscordID: discordID,
		Name:      name,
		AvatarURL: avatarURL,
		Role:      RoleUser,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (u *User) UpdateProfile(name, avatarURL string) {
	if u.Name != name || u.AvatarURL != avatarURL {
		u.Name = name
		u.AvatarURL = avatarURL
		u.UpdatedAt = time.Now()
	}
}
