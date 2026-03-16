package session

import "time"

type Session struct {
	UserID          string    `json:"user_id"`
	AccessToken     string    `json:"access_token"`
	RefreshToken    string    `json:"refresh_token"`
	IDToken         string    `json:"id_token"`
	TokenExpiry     time.Time `json:"token_expiry"`
	DeviceSessionID string    `json:"device_session_id"`
	CreatedAt       time.Time `json:"created_at"`
	LastActiveAt    time.Time `json:"last_active_at"`
}
