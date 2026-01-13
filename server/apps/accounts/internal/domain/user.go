// internal/domain/user.go
package domain

import "time"

// User represents the core user entity.
// Tags are added for sqlx automated mapping.
type User struct {
	ID        string    `db:"id"`
	Username  string    `db:"username"`
	AvatarURL string    `db:"avatar_url"`
	Role      string    `db:"role"`
	CreatedAt time.Time `db:"created_at"`
}
