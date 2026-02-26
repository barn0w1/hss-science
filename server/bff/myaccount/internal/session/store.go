package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionData holds the data stored in Redis for a user session.
type SessionData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token"`
	UserID       string    `json:"user_id"`
	Email        string    `json:"email"`
	GivenName    string    `json:"given_name"`
	FamilyName   string    `json:"family_name"`
	Picture      string    `json:"picture"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Store manages sessions in Redis.
type Store struct {
	rdb    *redis.Client
	prefix string
	maxAge time.Duration
}

// NewStore creates a new Redis-backed session store.
func NewStore(rdb *redis.Client, maxAge time.Duration) *Store {
	return &Store{
		rdb:    rdb,
		prefix: "myaccount:session:",
		maxAge: maxAge,
	}
}

// Create stores a new session and returns an opaque session ID.
func (s *Store) Create(ctx context.Context, data *SessionData) (string, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}

	if err := s.rdb.Set(ctx, s.prefix+sessionID, encoded, s.maxAge).Err(); err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}

	return sessionID, nil
}

// Get retrieves session data by session ID.
func (s *Store) Get(ctx context.Context, sessionID string) (*SessionData, error) {
	val, err := s.rdb.Get(ctx, s.prefix+sessionID).Result()
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	var data SessionData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &data, nil
}

// Delete removes a session from Redis.
func (s *Store) Delete(ctx context.Context, sessionID string) error {
	return s.rdb.Del(ctx, s.prefix+sessionID).Err()
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
