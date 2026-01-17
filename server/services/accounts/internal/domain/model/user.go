package model

import (
	"time"

	"github.com/google/uuid"
)

// GlobalRole defines system-wide permissions.
// Service-specific permissions (e.g. Drive read/write) should be handled in respective services
// or mapped from these global roles.
type GlobalRole string

const (
	RoleSystemAdmin GlobalRole = "system_admin" // System Administrator
	RoleModerator   GlobalRole = "moderator"    // Community Moderator / Manager
	RoleUser        GlobalRole = "user"         // General Member
)

// User represents a registered user in the system (Identity).
type User struct {
	ID        string     // HSS internal UUID
	DiscordID string     // Unique Discord ID
	Name      string     // Discord Username
	AvatarURL string     // Discord Avatar URL
	Role      GlobalRole // System-wide permissions
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewUser creates a user instance with a generated UUID.
func NewUser(discordID, name, avatarURL string) *User {
	newID := uuid.New().String()

	return &User{
		ID:        newID,
		DiscordID: discordID,
		Name:      name,
		AvatarURL: avatarURL,
		Role:      RoleUser, // Default is general user
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// UpdateProfile updates mutable fields from Discord info.
func (u *User) UpdateProfile(name, avatarURL string) {
	if u.Name != name || u.AvatarURL != avatarURL {
		u.Name = name
		u.AvatarURL = avatarURL
		u.UpdatedAt = time.Now()
	}
}
