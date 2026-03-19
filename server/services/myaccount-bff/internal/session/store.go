package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix   = "session:"
	lockPrefix  = "refresh_lock:"
	statePrefix = "oidc_state:"
	lockTTL     = 10 * time.Second
	stateTTL    = 10 * time.Minute
)

var ErrNotFound = errors.New("session: not found")

type Store struct {
	rdb     *redis.Client
	idleTTL time.Duration
	hardTTL time.Duration
}

func NewStore(rdb *redis.Client, idleTTL, hardTTL time.Duration) *Store {
	return &Store{rdb: rdb, idleTTL: idleTTL, hardTTL: hardTTL}
}

func (s *Store) Save(ctx context.Context, sid string, sess *Session) error {
	data, err := json.Marshal(sess) //nolint:gosec // tokens stored server-side in Redis, never sent to client
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return s.rdb.Set(ctx, keyPrefix+sid, data, s.idleTTL).Err()
}

func (s *Store) Load(ctx context.Context, sid string) (*Session, error) {
	data, err := s.rdb.GetEx(ctx, keyPrefix+sid, s.idleTTL).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load session: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	if !sess.CreatedAt.IsZero() && time.Now().UTC().After(sess.CreatedAt.Add(s.hardTTL)) {
		_ = s.Delete(ctx, sid)
		return nil, ErrNotFound
	}
	return &sess, nil
}

func (s *Store) Delete(ctx context.Context, sid string) error {
	return s.rdb.Del(ctx, keyPrefix+sid).Err()
}

func (s *Store) AcquireRefreshLock(ctx context.Context, sid string) (bool, error) {
	_, err := s.rdb.SetArgs(ctx, lockPrefix+sid, "1", redis.SetArgs{
		Mode: "NX",
		TTL:  lockTTL,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("acquire refresh lock: %w", err)
	}
	return true, nil
}

func (s *Store) ReleaseRefreshLock(ctx context.Context, sid string) error {
	return s.rdb.Del(ctx, lockPrefix+sid).Err()
}

func (s *Store) SaveState(ctx context.Context, state, verifier string) error {
	data, err := json.Marshal(map[string]string{"verifier": verifier})
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return s.rdb.Set(ctx, statePrefix+state, data, stateTTL).Err()
}

var getDelScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v ~= false then redis.call('DEL', KEYS[1]) end
return v
`)

func (s *Store) LoadAndDeleteState(ctx context.Context, state string) (string, error) {
	result, err := getDelScript.Run(ctx, s.rdb, []string{statePrefix + state}).Text()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("load and delete state: %w", err)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		return "", fmt.Errorf("unmarshal state: %w", err)
	}
	v, ok := payload["verifier"]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}
