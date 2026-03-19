package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/barn0w1/hss-science/server/services/identity-service/internal/pkg/domerr"
	"github.com/barn0w1/hss-science/server/services/identity-service/testhelper"
)

func TestDeviceSessionRepository_Create(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	_, err := testDB.ExecContext(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "u@ex.com")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewDeviceSessionRepository(testDB)
	dsID := ulid.Make().String()
	ds, err := repo.FindOrCreate(ctx, dsID, userID, "Mozilla/5.0", "1.2.3.4", "Chrome on macOS")
	if err != nil {
		t.Fatalf("FindOrCreate: %v", err)
	}
	if ds.ID != dsID {
		t.Errorf("expected %s, got %s", dsID, ds.ID)
	}
	if ds.UserID != userID {
		t.Errorf("expected user %s, got %s", userID, ds.UserID)
	}
}

func TestDeviceSessionRepository_FindExisting(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	_, err := testDB.ExecContext(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "u@ex.com")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewDeviceSessionRepository(testDB)
	dsID := ulid.Make().String()
	if _, err := repo.FindOrCreate(ctx, dsID, userID, "OldUA", "1.1.1.1", "Old device"); err != nil {
		t.Fatalf("first FindOrCreate: %v", err)
	}

	ds, err := repo.FindOrCreate(ctx, dsID, userID, "NewUA", "2.2.2.2", "New device")
	if err != nil {
		t.Fatalf("second FindOrCreate: %v", err)
	}
	if ds.ID != dsID {
		t.Errorf("expected same ID %s, got %s", dsID, ds.ID)
	}
	if ds.UserAgent != "NewUA" {
		t.Errorf("expected UserAgent NewUA, got %s", ds.UserAgent)
	}
	if ds.IPAddress != "2.2.2.2" {
		t.Errorf("expected IP 2.2.2.2, got %s", ds.IPAddress)
	}
}

func TestDeviceSessionRepository_CrossUser(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	user1 := ulid.Make().String()
	user2 := ulid.Make().String()
	for _, row := range []struct{ id, email string }{{user1, "u1@ex.com"}, {user2, "u2@ex.com"}} {
		if _, err := testDB.ExecContext(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, row.id, row.email); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}

	repo := NewDeviceSessionRepository(testDB)
	dsID := ulid.Make().String()
	if _, err := repo.FindOrCreate(ctx, dsID, user1, "UA", "1.1.1.1", "Dev"); err != nil {
		t.Fatalf("first FindOrCreate: %v", err)
	}

	ds, err := repo.FindOrCreate(ctx, dsID, user2, "UA", "1.1.1.1", "Dev")
	if err != nil {
		t.Fatalf("cross-user FindOrCreate: %v", err)
	}
	if ds.ID == dsID {
		t.Error("expected a new session ID for different user")
	}
	if ds.UserID != user2 {
		t.Errorf("expected user2 %s, got %s", user2, ds.UserID)
	}
}

func TestDeviceSessionRepository_RevokedSession(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	if _, err := testDB.ExecContext(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "u@ex.com"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewDeviceSessionRepository(testDB)
	dsID := ulid.Make().String()
	if _, err := repo.FindOrCreate(ctx, dsID, userID, "UA", "1.1.1.1", "Dev"); err != nil {
		t.Fatalf("FindOrCreate: %v", err)
	}
	if err := repo.RevokeByID(ctx, dsID, userID); err != nil {
		t.Fatalf("RevokeByID: %v", err)
	}

	ds, err := repo.FindOrCreate(ctx, dsID, userID, "UA", "1.1.1.1", "Dev")
	if err != nil {
		t.Fatalf("FindOrCreate after revoke: %v", err)
	}
	if ds.ID == dsID {
		t.Error("expected new session ID after revoked session")
	}
}

func TestDeviceSessionRepository_RevokeByID(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	if _, err := testDB.ExecContext(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "u@ex.com"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewDeviceSessionRepository(testDB)
	dsID := ulid.Make().String()
	if _, err := repo.FindOrCreate(ctx, dsID, userID, "UA", "1.1.1.1", "Dev"); err != nil {
		t.Fatalf("FindOrCreate: %v", err)
	}

	// Insert a refresh token linked to the device session
	_, err := testDB.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, token_hash, client_id, user_id, audience, scopes, auth_time, amr, expiration, device_session_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		ulid.Make().String(), "hash-test", "c1", userID,
		`{"c1"}`, `{"openid"}`, time.Now().UTC(), `{"fed"}`,
		time.Now().UTC().Add(time.Hour), dsID,
	)
	if err != nil {
		t.Fatalf("insert refresh token: %v", err)
	}

	if err := repo.RevokeByID(ctx, dsID, userID); err != nil {
		t.Fatalf("RevokeByID: %v", err)
	}

	// Device session should be revoked
	var revokedAt *time.Time
	err = testDB.QueryRowxContext(ctx, `SELECT revoked_at FROM device_sessions WHERE id = $1`, dsID).Scan(&revokedAt)
	if err != nil {
		t.Fatalf("query revoked_at: %v", err)
	}
	if revokedAt == nil {
		t.Error("expected revoked_at to be set")
	}

	// Refresh tokens should be deleted
	var count int
	err = testDB.QueryRowxContext(ctx, `SELECT count(*) FROM refresh_tokens WHERE device_session_id = $1`, dsID).Scan(&count)
	if err != nil {
		t.Fatalf("count refresh tokens: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 refresh tokens, got %d", count)
	}
}

func TestDeviceSessionRepository_RevokeByID_WrongUser(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	if _, err := testDB.ExecContext(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "u@ex.com"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewDeviceSessionRepository(testDB)
	dsID := ulid.Make().String()
	if _, err := repo.FindOrCreate(ctx, dsID, userID, "UA", "1.1.1.1", "Dev"); err != nil {
		t.Fatalf("FindOrCreate: %v", err)
	}

	err := repo.RevokeByID(ctx, dsID, "wrong-user")
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeviceSessionRepository_ListActiveByUserID(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	if _, err := testDB.ExecContext(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "u@ex.com"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewDeviceSessionRepository(testDB)
	ds1ID := ulid.Make().String()
	ds2ID := ulid.Make().String()
	if _, err := repo.FindOrCreate(ctx, ds1ID, userID, "UA1", "1.1.1.1", "Dev1"); err != nil {
		t.Fatalf("FindOrCreate ds1: %v", err)
	}
	if _, err := repo.FindOrCreate(ctx, ds2ID, userID, "UA2", "2.2.2.2", "Dev2"); err != nil {
		t.Fatalf("FindOrCreate ds2: %v", err)
	}
	if err := repo.RevokeByID(ctx, ds1ID, userID); err != nil {
		t.Fatalf("RevokeByID: %v", err)
	}

	sessions, err := repo.ListActiveByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("ListActiveByUserID: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(sessions))
	}
	if sessions[0].ID != ds2ID {
		t.Errorf("expected ds2 %s, got %s", ds2ID, sessions[0].ID)
	}
}

func TestDeviceSessionRepository_DeleteRevokedBefore(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	if _, err := testDB.ExecContext(ctx, `INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "u@ex.com"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewDeviceSessionRepository(testDB)
	dsID := ulid.Make().String()
	if _, err := repo.FindOrCreate(ctx, dsID, userID, "UA", "1.1.1.1", "Dev"); err != nil {
		t.Fatalf("FindOrCreate: %v", err)
	}
	if err := repo.RevokeByID(ctx, dsID, userID); err != nil {
		t.Fatalf("RevokeByID: %v", err)
	}

	n, err := repo.DeleteRevokedBefore(ctx, time.Now().UTC().Add(time.Minute))
	if err != nil {
		t.Fatalf("DeleteRevokedBefore: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 deleted, got %d", n)
	}

	var count int
	_ = testDB.QueryRowxContext(ctx, `SELECT count(*) FROM device_sessions WHERE id = $1`, dsID).Scan(&count)
	if count != 0 {
		t.Error("expected device session to be deleted")
	}
}
