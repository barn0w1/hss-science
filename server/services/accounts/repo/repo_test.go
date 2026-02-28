package repo

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/barn0w1/hss-science/server/services/accounts/model"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgC, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("accounts_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		panic("failed to start postgres: " + err.Error())
	}
	defer func() { _ = pgC.Terminate(ctx) }()

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic("failed to get connection string: " + err.Error())
	}

	testDB, err = sqlx.Connect("postgres", connStr)
	if err != nil {
		panic("failed to connect: " + err.Error())
	}
	defer func() { _ = testDB.Close() }()

	if err := runMigrations(testDB); err != nil {
		panic("failed to run migrations: " + err.Error())
	}

	os.Exit(m.Run())
}

func runMigrations(db *sqlx.DB) error {
	schema := `
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT NOT NULL,
    email_verified  BOOLEAN NOT NULL DEFAULT false,
    name            TEXT,
    given_name      TEXT,
    family_name     TEXT,
    picture         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE federated_identities (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider          TEXT NOT NULL,
    provider_subject  TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, provider_subject)
);
CREATE TABLE clients (
    id                          TEXT PRIMARY KEY,
    secret_hash                 TEXT NOT NULL DEFAULT '',
    redirect_uris               TEXT[] NOT NULL,
    post_logout_redirect_uris   TEXT[] NOT NULL DEFAULT '{}',
    application_type            TEXT NOT NULL DEFAULT 'web',
    auth_method                 TEXT NOT NULL DEFAULT 'client_secret_basic',
    response_types              TEXT[] NOT NULL,
    grant_types                 TEXT[] NOT NULL,
    access_token_type           TEXT NOT NULL DEFAULT 'jwt',
    id_token_lifetime_seconds   INTEGER NOT NULL DEFAULT 3600,
    clock_skew_seconds          INTEGER NOT NULL DEFAULT 0,
    id_token_userinfo_assertion BOOLEAN NOT NULL DEFAULT false,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE auth_requests (
    id                    UUID PRIMARY KEY,
    client_id             TEXT NOT NULL,
    redirect_uri          TEXT NOT NULL,
    state                 TEXT,
    nonce                 TEXT,
    scopes                TEXT[],
    response_type         TEXT NOT NULL,
    response_mode         TEXT,
    code_challenge        TEXT,
    code_challenge_method TEXT,
    prompt                TEXT[],
    max_age               INTEGER,
    login_hint            TEXT,
    user_id               UUID,
    auth_time             TIMESTAMPTZ,
    amr                   TEXT[],
    is_done               BOOLEAN NOT NULL DEFAULT false,
    code                  TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX auth_requests_code_idx ON auth_requests (code) WHERE code IS NOT NULL;
CREATE TABLE tokens (
    id               UUID PRIMARY KEY,
    client_id        TEXT NOT NULL,
    subject          TEXT NOT NULL,
    audience         TEXT[],
    scopes           TEXT[],
    expiration       TIMESTAMPTZ NOT NULL,
    refresh_token_id UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE refresh_tokens (
    id               UUID PRIMARY KEY,
    token            TEXT NOT NULL UNIQUE,
    client_id        TEXT NOT NULL,
    user_id          UUID NOT NULL REFERENCES users(id),
    audience         TEXT[],
    scopes           TEXT[],
    auth_time        TIMESTAMPTZ NOT NULL,
    amr              TEXT[],
    access_token_id  UUID,
    expiration       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);`
	_, err := db.Exec(schema)
	return err
}

func cleanTables(t *testing.T) {
	t.Helper()
	for _, table := range []string{"refresh_tokens", "tokens", "auth_requests", "federated_identities", "users", "clients"} {
		if _, err := testDB.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("failed to clean table %s: %v", table, err)
		}
	}
}

func TestUserRepository_CreateAndGetByID(t *testing.T) {
	cleanTables(t)
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	user := &model.User{
		ID:            uuid.New().String(),
		Email:         "alice@example.com",
		EmailVerified: true,
		Name:          "Alice Smith",
		GivenName:     "Alice",
		FamilyName:    "Smith",
		Picture:       "https://example.com/alice.jpg",
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("expected alice@example.com, got %s", got.Email)
	}
	if got.Name != "Alice Smith" {
		t.Errorf("expected Alice Smith, got %s", got.Name)
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	cleanTables(t)
	repo := NewUserRepository(testDB)
	_, err := repo.GetByID(context.Background(), uuid.New().String())
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestUserRepository_FindByFederatedIdentity(t *testing.T) {
	cleanTables(t)
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	user := &model.User{
		ID:    uuid.New().String(),
		Email: "bob@example.com",
		Name:  "Bob",
	}
	fi := &model.FederatedIdentity{
		ID:              uuid.New().String(),
		UserID:          user.ID,
		Provider:        "google",
		ProviderSubject: "goog-123",
	}
	if err := repo.CreateWithFederatedIdentity(ctx, user, fi); err != nil {
		t.Fatalf("CreateWithFederatedIdentity: %v", err)
	}

	got, err := repo.FindByFederatedIdentity(ctx, "google", "goog-123")
	if err != nil {
		t.Fatalf("FindByFederatedIdentity: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got.ID != user.ID {
		t.Errorf("expected ID %s, got %s", user.ID, got.ID)
	}
}

func TestUserRepository_FindByFederatedIdentity_NotFound(t *testing.T) {
	cleanTables(t)
	repo := NewUserRepository(testDB)
	got, err := repo.FindByFederatedIdentity(context.Background(), "google", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got user %v", got)
	}
}

func TestClientRepository_GetByID(t *testing.T) {
	cleanTables(t)
	ctx := context.Background()

	_, err := testDB.ExecContext(ctx,
		`INSERT INTO clients (id, secret_hash, redirect_uris, response_types, grant_types, access_token_type)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		"test-client", "$2a$10$hash",
		`{"https://app.example.com/callback"}`,
		`{"code"}`,
		`{"authorization_code","refresh_token"}`,
		"jwt",
	)
	if err != nil {
		t.Fatalf("insert client: %v", err)
	}

	repo := NewClientRepository(testDB)
	c, err := repo.GetByID(ctx, "test-client")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if c.ID != "test-client" {
		t.Errorf("expected test-client, got %s", c.ID)
	}
	if len(c.RedirectURIs) != 1 || c.RedirectURIs[0] != "https://app.example.com/callback" {
		t.Errorf("unexpected redirect URIs: %v", c.RedirectURIs)
	}
	if c.AccessTokenType != "jwt" {
		t.Errorf("expected jwt, got %s", c.AccessTokenType)
	}
}

func TestClientRepository_GetByID_NotFound(t *testing.T) {
	cleanTables(t)
	repo := NewClientRepository(testDB)
	_, err := repo.GetByID(context.Background(), "nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestAuthRequestRepository_CRUD(t *testing.T) {
	cleanTables(t)
	repo := NewAuthRequestRepository(testDB)
	ctx := context.Background()

	ar := &model.AuthRequest{
		ID:           uuid.New().String(),
		ClientID:     "test-client",
		RedirectURI:  "https://app.example.com/callback",
		State:        "state-1",
		Nonce:        "nonce-1",
		Scopes:       []string{"openid", "email"},
		ResponseType: "code",
	}
	if err := repo.Create(ctx, ar); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, ar.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ClientID != "test-client" {
		t.Errorf("expected test-client, got %s", got.ClientID)
	}
	if len(got.Scopes) != 2 {
		t.Errorf("expected 2 scopes, got %d", len(got.Scopes))
	}

	if err := repo.SaveCode(ctx, ar.ID, "code-123"); err != nil {
		t.Fatalf("SaveCode: %v", err)
	}

	byCode, err := repo.GetByCode(ctx, "code-123")
	if err != nil {
		t.Fatalf("GetByCode: %v", err)
	}
	if byCode.ID != ar.ID {
		t.Errorf("expected ID %s, got %s", ar.ID, byCode.ID)
	}

	userID := uuid.New().String()
	now := time.Now().UTC()
	if err := repo.CompleteLogin(ctx, ar.ID, userID, now, []string{"federated"}); err != nil {
		t.Fatalf("CompleteLogin: %v", err)
	}

	completed, err := repo.GetByID(ctx, ar.ID)
	if err != nil {
		t.Fatalf("GetByID after complete: %v", err)
	}
	if !completed.IsDone {
		t.Error("expected IsDone to be true")
	}
	if completed.UserID != userID {
		t.Errorf("expected user ID %s, got %s", userID, completed.UserID)
	}

	if err := repo.Delete(ctx, ar.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = repo.GetByID(ctx, ar.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after delete, got %v", err)
	}
}

func TestTokenRepository_CreateAccess(t *testing.T) {
	cleanTables(t)
	repo := NewTokenRepository(testDB)
	ctx := context.Background()

	exp := time.Now().UTC().Add(15 * time.Minute)
	id, err := repo.CreateAccess(ctx, "test-client", "user-1", []string{"test-client"}, []string{"openid"}, exp)
	if err != nil {
		t.Fatalf("CreateAccess: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty token ID")
	}

	tok, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if tok.ClientID != "test-client" {
		t.Errorf("expected client test-client, got %s", tok.ClientID)
	}
	if tok.Subject != "user-1" {
		t.Errorf("expected subject user-1, got %s", tok.Subject)
	}
}

func TestTokenRepository_CreateAccessAndRefresh(t *testing.T) {
	cleanTables(t)
	ctx := context.Background()

	userID := uuid.New().String()
	_, err := testDB.ExecContext(ctx,
		`INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "user@example.com")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewTokenRepository(testDB)
	accessExp := time.Now().UTC().Add(15 * time.Minute)
	refreshExp := time.Now().UTC().Add(7 * 24 * time.Hour)
	authTime := time.Now().UTC()

	accessID, refreshToken, err := repo.CreateAccessAndRefresh(ctx,
		"test-client", userID, []string{"test-client"}, []string{"openid", "email"},
		accessExp, refreshExp, authTime, []string{"federated"}, "",
	)
	if err != nil {
		t.Fatalf("CreateAccessAndRefresh: %v", err)
	}
	if accessID == "" || refreshToken == "" {
		t.Fatal("expected non-empty IDs")
	}

	rt, err := repo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		t.Fatalf("GetRefreshToken: %v", err)
	}
	if rt.ClientID != "test-client" {
		t.Errorf("expected client test-client, got %s", rt.ClientID)
	}
	if rt.UserID != userID {
		t.Errorf("expected user %s, got %s", userID, rt.UserID)
	}

	rtUserID, rtID, err := repo.GetRefreshInfo(ctx, refreshToken)
	if err != nil {
		t.Fatalf("GetRefreshInfo: %v", err)
	}
	if rtUserID != userID {
		t.Errorf("expected user %s, got %s", userID, rtUserID)
	}
	if rtID == "" {
		t.Error("expected non-empty refresh token ID")
	}

	accessID2, refreshToken2, err := repo.CreateAccessAndRefresh(ctx,
		"test-client", userID, []string{"test-client"}, []string{"openid"},
		accessExp, refreshExp, authTime, []string{"federated"}, refreshToken,
	)
	if err != nil {
		t.Fatalf("CreateAccessAndRefresh (rotation): %v", err)
	}
	if accessID2 == "" || refreshToken2 == "" {
		t.Fatal("expected non-empty IDs after rotation")
	}
	if refreshToken2 == refreshToken {
		t.Error("expected new refresh token after rotation")
	}

	_, err = repo.GetRefreshToken(ctx, refreshToken)
	if err != sql.ErrNoRows {
		t.Errorf("expected old refresh token to be deleted, got %v", err)
	}
}

func TestTokenRepository_Revoke(t *testing.T) {
	cleanTables(t)
	repo := NewTokenRepository(testDB)
	ctx := context.Background()

	exp := time.Now().UTC().Add(15 * time.Minute)
	id, err := repo.CreateAccess(ctx, "test-client", "user-1", []string{"test-client"}, []string{"openid"}, exp)
	if err != nil {
		t.Fatalf("CreateAccess: %v", err)
	}

	if err := repo.Revoke(ctx, id); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	_, err = repo.GetByID(ctx, id)
	if err != sql.ErrNoRows {
		t.Errorf("expected token to be revoked, got %v", err)
	}
}

func TestTokenRepository_DeleteByUserAndClient(t *testing.T) {
	cleanTables(t)
	ctx := context.Background()

	userID := uuid.New().String()
	_, err := testDB.ExecContext(ctx,
		`INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "user@example.com")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewTokenRepository(testDB)
	exp := time.Now().UTC().Add(15 * time.Minute)
	refreshExp := time.Now().UTC().Add(7 * 24 * time.Hour)
	authTime := time.Now().UTC()

	accessID, refreshToken, err := repo.CreateAccessAndRefresh(ctx,
		"test-client", userID, []string{"test-client"}, []string{"openid"},
		exp, refreshExp, authTime, []string{"federated"}, "",
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.DeleteByUserAndClient(ctx, userID, "test-client"); err != nil {
		t.Fatalf("DeleteByUserAndClient: %v", err)
	}

	_, err = repo.GetByID(ctx, accessID)
	if err != sql.ErrNoRows {
		t.Errorf("expected access token deleted, got %v", err)
	}
	_, err = repo.GetRefreshToken(ctx, refreshToken)
	if err != sql.ErrNoRows {
		t.Errorf("expected refresh token deleted, got %v", err)
	}
}
