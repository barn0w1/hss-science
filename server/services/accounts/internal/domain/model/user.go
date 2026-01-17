package model

import (
	"time"
)

// Role defines user permissions
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// User represents a registered user in the system.
type User struct {
	ID        string // HSS内部でのUUID (他サービスからはこれを参照する)
	DiscordID string // Discord側のユニークID (Snowflake)
	Name      string // DiscordのUsername (表示用)
	AvatarURL string // DiscordのAvatar URL (表示用)
	Role      Role   // 権限
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewUser creates a user instance with default role.
func NewUser(discordID, name, avatarURL string) *User {
	return &User{
		// IDはRepository保存時に生成するか、ここでUUIDライブラリで生成する
		DiscordID: discordID,
		Name:      name,
		AvatarURL: avatarURL,
		Role:      RoleMember, // デフォルトはメンバー
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// UpdateProfile updates mutable fields from Discord info.
func (u *User) UpdateProfile(name, avatarURL string) {
	u.Name = name
	u.AvatarURL = avatarURL
	u.UpdatedAt = time.Now()
}
