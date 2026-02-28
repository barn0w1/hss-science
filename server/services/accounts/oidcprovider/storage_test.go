package oidcprovider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/crypto/bcrypt"

	"github.com/barn0w1/hss-science/server/services/accounts/model"
	"github.com/barn0w1/hss-science/server/services/accounts/repo"
)

var storageTestDB *sqlx.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgC, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("storage_test"),
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

	storageTestDB, err = sqlx.Connect("postgres", connStr)
	if err != nil {
		panic("failed to connect: " + err.Error())
	}
	defer func() { _ = storageTestDB.Close() }()

	if err := runMigrations(storageTestDB); err != nil {
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
		if _, err := storageTestDB.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("failed to clean table %s: %v", table, err)
		}
	}
}

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	sk := NewSigningKey(key)
	pk := NewPublicKey(key)
	return NewStorage(
		storageTestDB,
		repo.NewUserRepository(storageTestDB),
		repo.NewClientRepository(storageTestDB),
		repo.NewAuthRequestRepository(storageTestDB),
		repo.NewTokenRepository(storageTestDB),
		sk, pk,
	)
}

func seedTestClient(t *testing.T, clientID, secret string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	_, err = storageTestDB.Exec(
		`INSERT INTO clients (id, secret_hash, redirect_uris, response_types, grant_types, access_token_type)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		clientID, string(hash),
		`{"https://app.example.com/callback"}`,
		`{"code"}`,
		`{"authorization_code","refresh_token"}`,
		"jwt",
	)
	if err != nil {
		t.Fatalf("seed client: %v", err)
	}
}

func seedTestUser(t *testing.T) *model.User {
	t.Helper()
	user := &model.User{
		ID:            uuid.New().String(),
		Email:         "test@example.com",
		EmailVerified: true,
		Name:          "Test User",
		GivenName:     "Test",
		FamilyName:    "User",
		Picture:       "https://example.com/pic.jpg",
	}
	_, err := storageTestDB.Exec(
		`INSERT INTO users (id, email, email_verified, name, given_name, family_name, picture) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		user.ID, user.Email, user.EmailVerified, user.Name, user.GivenName, user.FamilyName, user.Picture,
	)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func TestStorage_CreateAuthRequest(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()

	authReq := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "https://app.example.com/callback",
		State:        "state-1",
		Nonce:        "nonce-1",
		Scopes:       oidc.SpaceDelimitedArray{"openid", "email"},
		ResponseType: oidc.ResponseTypeCode,
	}

	ar, err := s.CreateAuthRequest(ctx, authReq, "")
	if err != nil {
		t.Fatalf("CreateAuthRequest: %v", err)
	}
	if ar.GetID() == "" {
		t.Fatal("expected non-empty auth request ID")
	}
	if ar.GetClientID() != "test-client" {
		t.Errorf("expected test-client, got %s", ar.GetClientID())
	}
}

func TestStorage_AuthRequestByID(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()

	authReq := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "https://app.example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	created, err := s.CreateAuthRequest(ctx, authReq, "")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.AuthRequestByID(ctx, created.GetID())
	if err != nil {
		t.Fatalf("AuthRequestByID: %v", err)
	}
	if got.GetID() != created.GetID() {
		t.Errorf("expected ID %s, got %s", created.GetID(), got.GetID())
	}
}

func TestStorage_AuthRequestByID_NotFound(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	_, err := s.AuthRequestByID(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error for non-existent auth request")
	}
}

func TestStorage_SaveAuthCode_AuthRequestByCode(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()

	authReq := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "https://app.example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	created, err := s.CreateAuthRequest(ctx, authReq, "")
	if err != nil {
		t.Fatal(err)
	}

	code := uuid.New().String()
	if err := s.SaveAuthCode(ctx, created.GetID(), code); err != nil {
		t.Fatalf("SaveAuthCode: %v", err)
	}

	byCode, err := s.AuthRequestByCode(ctx, code)
	if err != nil {
		t.Fatalf("AuthRequestByCode: %v", err)
	}
	if byCode.GetID() != created.GetID() {
		t.Errorf("expected ID %s, got %s", created.GetID(), byCode.GetID())
	}
}

func TestStorage_DeleteAuthRequest(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()

	authReq := &oidc.AuthRequest{
		ClientID:     "test-client",
		RedirectURI:  "https://app.example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}
	created, err := s.CreateAuthRequest(ctx, authReq, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteAuthRequest(ctx, created.GetID()); err != nil {
		t.Fatalf("DeleteAuthRequest: %v", err)
	}

	_, err = s.AuthRequestByID(ctx, created.GetID())
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestStorage_GetClientByClientID(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()
	seedTestClient(t, "my-client", "secret123")

	client, err := s.GetClientByClientID(ctx, "my-client")
	if err != nil {
		t.Fatalf("GetClientByClientID: %v", err)
	}
	if client.GetID() != "my-client" {
		t.Errorf("expected my-client, got %s", client.GetID())
	}
}

func TestStorage_GetClientByClientID_NotFound(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	_, err := s.GetClientByClientID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent client")
	}
}

func TestStorage_AuthorizeClientIDSecret(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()
	seedTestClient(t, "my-client", "secret123")

	if err := s.AuthorizeClientIDSecret(ctx, "my-client", "secret123"); err != nil {
		t.Fatalf("AuthorizeClientIDSecret: %v", err)
	}

	if err := s.AuthorizeClientIDSecret(ctx, "my-client", "wrong-secret"); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestStorage_CreateAccessToken(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()

	ar := &AuthRequest{model: &model.AuthRequest{
		ClientID: "test-client",
		UserID:   uuid.New().String(),
		Scopes:   []string{"openid"},
	}}

	tokenID, exp, err := s.CreateAccessToken(ctx, ar)
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}
	if tokenID == "" {
		t.Fatal("expected non-empty token ID")
	}
	if exp.Before(time.Now()) {
		t.Error("expected expiration in the future")
	}
}

func TestStorage_CreateAccessAndRefreshTokens(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{model: &model.AuthRequest{
		ClientID: "test-client",
		UserID:   user.ID,
		Scopes:   []string{"openid", "offline_access"},
		AuthTime: time.Now().UTC(),
		AMR:      []string{"federated"},
	}}

	accessID, refreshToken, exp, err := s.CreateAccessAndRefreshTokens(ctx, ar, "")
	if err != nil {
		t.Fatalf("CreateAccessAndRefreshTokens: %v", err)
	}
	if accessID == "" || refreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if exp.Before(time.Now()) {
		t.Error("expected expiration in the future")
	}
}

func TestStorage_TokenRequestByRefreshToken(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{model: &model.AuthRequest{
		ClientID: "test-client",
		UserID:   user.ID,
		Scopes:   []string{"openid"},
		AuthTime: time.Now().UTC(),
		AMR:      []string{"federated"},
	}}

	_, refreshToken, _, err := s.CreateAccessAndRefreshTokens(ctx, ar, "")
	if err != nil {
		t.Fatal(err)
	}

	rtr, err := s.TokenRequestByRefreshToken(ctx, refreshToken)
	if err != nil {
		t.Fatalf("TokenRequestByRefreshToken: %v", err)
	}
	if rtr.GetSubject() != user.ID {
		t.Errorf("expected subject %s, got %s", user.ID, rtr.GetSubject())
	}
}

func TestStorage_TokenRequestByRefreshToken_NotFound(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	_, err := s.TokenRequestByRefreshToken(context.Background(), "nonexistent")
	if !errors.Is(err, op.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
	}
}

func TestStorage_RevokeToken(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()

	ar := &AuthRequest{model: &model.AuthRequest{
		ClientID: "test-client",
		UserID:   uuid.New().String(),
		Scopes:   []string{"openid"},
	}}
	tokenID, _, err := s.CreateAccessToken(ctx, ar)
	if err != nil {
		t.Fatal(err)
	}

	if oidcErr := s.RevokeToken(ctx, tokenID, "user-1", "test-client"); oidcErr != nil {
		t.Fatalf("RevokeToken: %v", oidcErr)
	}
}

func TestStorage_TerminateSession(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{model: &model.AuthRequest{
		ClientID: "test-client",
		UserID:   user.ID,
		Scopes:   []string{"openid"},
		AuthTime: time.Now().UTC(),
		AMR:      []string{"federated"},
	}}

	_, _, _, err := s.CreateAccessAndRefreshTokens(ctx, ar, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := s.TerminateSession(ctx, user.ID, "test-client"); err != nil {
		t.Fatalf("TerminateSession: %v", err)
	}
}

func TestStorage_SetUserinfoFromScopes(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()
	user := seedTestUser(t)

	userinfo := &oidc.UserInfo{}
	err := s.SetUserinfoFromScopes(ctx, userinfo, user.ID, "test-client", []string{oidc.ScopeOpenID, oidc.ScopeEmail, oidc.ScopeProfile})
	if err != nil {
		t.Fatalf("SetUserinfoFromScopes: %v", err)
	}

	if userinfo.Subject != user.ID {
		t.Errorf("expected subject %s, got %s", user.ID, userinfo.Subject)
	}
	if userinfo.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", userinfo.Email)
	}
	if userinfo.Name != "Test User" {
		t.Errorf("expected name Test User, got %s", userinfo.Name)
	}
	if userinfo.GivenName != "Test" {
		t.Errorf("expected given name Test, got %s", userinfo.GivenName)
	}
}

func TestStorage_SetUserinfoFromScopes_NotFound(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	userinfo := &oidc.UserInfo{}
	err := s.SetUserinfoFromScopes(context.Background(), userinfo, uuid.New().String(), "test-client", []string{oidc.ScopeOpenID})
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestStorage_SetIntrospectionFromToken(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{model: &model.AuthRequest{
		ClientID: "test-client",
		UserID:   user.ID,
		Scopes:   []string{oidc.ScopeOpenID, oidc.ScopeEmail, oidc.ScopeProfile},
	}}
	tokenID, _, err := s.CreateAccessToken(ctx, ar)
	if err != nil {
		t.Fatal(err)
	}

	introspection := &oidc.IntrospectionResponse{}
	err = s.SetIntrospectionFromToken(ctx, introspection, tokenID, user.ID, "test-client")
	if err != nil {
		t.Fatalf("SetIntrospectionFromToken: %v", err)
	}
	if !introspection.Active {
		t.Error("expected active=true")
	}
	if introspection.Subject != user.ID {
		t.Errorf("expected subject %s, got %s", user.ID, introspection.Subject)
	}
}

func TestStorage_SetIntrospectionFromToken_NotFound(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	introspection := &oidc.IntrospectionResponse{}
	err := s.SetIntrospectionFromToken(context.Background(), introspection, uuid.New().String(), "user-1", "test-client")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if introspection.Active {
		t.Error("expected active=false for non-existent token")
	}
}

func TestStorage_SigningKey(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	key, err := s.SigningKey(context.Background())
	if err != nil {
		t.Fatalf("SigningKey: %v", err)
	}
	if key.ID() == "" {
		t.Error("expected non-empty key ID")
	}
}

func TestStorage_KeySet(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	keys, err := s.KeySet(context.Background())
	if err != nil {
		t.Fatalf("KeySet: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}
}

func TestStorage_Health(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	if err := s.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestStorage_GetPrivateClaimsFromScopes(t *testing.T) {
	s := newTestStorage(t)
	claims, err := s.GetPrivateClaimsFromScopes(context.Background(), "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 0 {
		t.Errorf("expected empty claims map, got %v", claims)
	}
}

func TestStorage_GetKeyByIDAndClientID(t *testing.T) {
	s := newTestStorage(t)
	_, err := s.GetKeyByIDAndClientID(context.Background(), "kid", "client")
	if err == nil {
		t.Fatal("expected error for unsupported jwt profile grant")
	}
}

func TestStorage_ValidateJWTProfileScopes(t *testing.T) {
	s := newTestStorage(t)
	_, err := s.ValidateJWTProfileScopes(context.Background(), "user", []string{"openid"})
	if err == nil {
		t.Fatal("expected error for unsupported jwt profile grant")
	}
}

func TestStorage_GetRefreshTokenInfo(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{model: &model.AuthRequest{
		ClientID: "test-client",
		UserID:   user.ID,
		Scopes:   []string{"openid"},
		AuthTime: time.Now().UTC(),
		AMR:      []string{"federated"},
	}}

	_, refreshToken, _, err := s.CreateAccessAndRefreshTokens(ctx, ar, "")
	if err != nil {
		t.Fatal(err)
	}

	userID, tokenID, err := s.GetRefreshTokenInfo(ctx, "test-client", refreshToken)
	if err != nil {
		t.Fatalf("GetRefreshTokenInfo: %v", err)
	}
	if userID != user.ID {
		t.Errorf("expected user %s, got %s", user.ID, userID)
	}
	if tokenID == "" {
		t.Error("expected non-empty token ID")
	}
}

func TestStorage_GetRefreshTokenInfo_NotFound(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	_, _, err := s.GetRefreshTokenInfo(context.Background(), "client", "nonexistent")
	if !errors.Is(err, op.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
	}
}

func TestStorage_ClientCredentials(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()
	seedTestClient(t, "cc-client", "cc-secret")

	client, err := s.ClientCredentials(ctx, "cc-client", "cc-secret")
	if err != nil {
		t.Fatalf("ClientCredentials: %v", err)
	}
	if client.GetID() != "cc-client" {
		t.Errorf("expected cc-client, got %s", client.GetID())
	}

	_, err = s.ClientCredentials(ctx, "cc-client", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong credentials")
	}
}

func TestStorage_ClientCredentialsTokenRequest(t *testing.T) {
	s := newTestStorage(t)
	req, err := s.ClientCredentialsTokenRequest(context.Background(), "cc-client", []string{"openid"})
	if err != nil {
		t.Fatalf("ClientCredentialsTokenRequest: %v", err)
	}
	if req.GetSubject() != "cc-client" {
		t.Errorf("expected subject cc-client, got %s", req.GetSubject())
	}
}

func TestStorage_SetUserinfoFromToken(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{model: &model.AuthRequest{
		ClientID: "test-client",
		UserID:   user.ID,
		Scopes:   []string{oidc.ScopeOpenID, oidc.ScopeEmail},
	}}
	tokenID, _, err := s.CreateAccessToken(ctx, ar)
	if err != nil {
		t.Fatal(err)
	}

	userinfo := &oidc.UserInfo{}
	err = s.SetUserinfoFromToken(ctx, userinfo, tokenID, user.ID, "test-client")
	if err != nil {
		t.Fatalf("SetUserinfoFromToken: %v", err)
	}
	if userinfo.Subject != user.ID {
		t.Errorf("expected subject %s, got %s", user.ID, userinfo.Subject)
	}
	if userinfo.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", userinfo.Email)
	}
}

func TestStorage_SetUserinfoFromToken_TokenNotFound(t *testing.T) {
	cleanTables(t)
	s := newTestStorage(t)
	userinfo := &oidc.UserInfo{}
	err := s.SetUserinfoFromToken(context.Background(), userinfo, uuid.New().String(), "user-1", "test-client")
	if err == nil {
		t.Fatal("expected error for non-existent token")
	}
}
