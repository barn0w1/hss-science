package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewStore(rdb, 2*time.Hour, 7*24*time.Hour), mr
}

func sampleSession() *Session {
	now := time.Now().UTC().Truncate(time.Second)
	return &Session{
		UserID:          "user-1",
		AccessToken:     "at-abc",
		RefreshToken:    "rt-xyz",
		IDToken:         "idt",
		TokenExpiry:     now.Add(15 * time.Minute),
		DeviceSessionID: "dsid-1",
		CreatedAt:       now,
		LastActiveAt:    now,
	}
}

func TestStore_SaveLoad(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	sess := sampleSession()

	if err := store.Save(ctx, "sid-1", sess); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Load(ctx, "sid-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.UserID != sess.UserID {
		t.Errorf("UserID: got %s, want %s", got.UserID, sess.UserID)
	}
	if got.AccessToken != sess.AccessToken {
		t.Errorf("AccessToken mismatch")
	}
	if got.DeviceSessionID != sess.DeviceSessionID {
		t.Errorf("DeviceSessionID mismatch")
	}
}

func TestStore_Load_NotFound(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.Load(context.Background(), "nosuchkey")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_Load_HardTTLExpired(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	sess := sampleSession()
	sess.CreatedAt = time.Now().UTC().Add(-8 * 24 * time.Hour)

	if err := store.Save(ctx, "sid-old", sess); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, err := store.Load(ctx, "sid-old")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after hard TTL, got %v", err)
	}
}

func TestStore_Delete(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "sid-del", sampleSession()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(ctx, "sid-del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := store.Load(ctx, "sid-del")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestStore_AcquireRefreshLock(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	ok, err := store.AcquireRefreshLock(ctx, "sid-lock")
	if err != nil {
		t.Fatalf("AcquireRefreshLock: %v", err)
	}
	if !ok {
		t.Fatal("expected lock acquisition to succeed")
	}

	ok2, err := store.AcquireRefreshLock(ctx, "sid-lock")
	if err != nil {
		t.Fatalf("second AcquireRefreshLock: %v", err)
	}
	if ok2 {
		t.Fatal("expected second lock acquisition to fail")
	}

	if err := store.ReleaseRefreshLock(ctx, "sid-lock"); err != nil {
		t.Fatalf("ReleaseRefreshLock: %v", err)
	}

	mr.FastForward(lockTTL + time.Second)

	ok3, err := store.AcquireRefreshLock(ctx, "sid-lock")
	if err != nil {
		t.Fatalf("AcquireRefreshLock after release: %v", err)
	}
	if !ok3 {
		t.Fatal("expected lock acquisition after release to succeed")
	}
}

func TestStore_SaveState_LoadAndDeleteState(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if err := store.SaveState(ctx, "state-1", "verifier-abc"); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	verifier, err := store.LoadAndDeleteState(ctx, "state-1")
	if err != nil {
		t.Fatalf("LoadAndDeleteState: %v", err)
	}
	if verifier != "verifier-abc" {
		t.Errorf("expected verifier-abc, got %s", verifier)
	}

	_, err = store.LoadAndDeleteState(ctx, "state-1")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestStore_LoadAndDeleteState_NotFound(t *testing.T) {
	store, _ := newTestStore(t)
	_, err := store.LoadAndDeleteState(context.Background(), "nosuchstate")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
