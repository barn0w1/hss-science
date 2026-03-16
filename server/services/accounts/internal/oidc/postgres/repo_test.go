package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/oklog/ulid/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
	"github.com/barn0w1/hss-science/server/services/accounts/testhelper"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgC, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("oidc_repo_test"),
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

	if err := testhelper.RunMigrations(testDB); err != nil {
		panic("failed to run migrations: " + err.Error())
	}

	os.Exit(m.Run())
}

func TestClientRepository_GetByID(t *testing.T) {
	testhelper.CleanTables(t, testDB)
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
	testhelper.CleanTables(t, testDB)
	repo := NewClientRepository(testDB)
	_, err := repo.GetByID(context.Background(), "nonexistent")
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected domerr.ErrNotFound, got %v", err)
	}
}

func TestAuthRequestRepository_CRUD(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	repo := NewAuthRequestRepository(testDB)
	ctx := context.Background()

	ar := &oidc.AuthRequest{
		ID:           ulid.Make().String(),
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

	userID := ulid.Make().String()
	now := time.Now().UTC()
	if err := repo.CompleteLogin(ctx, ar.ID, userID, now, []string{"federated"}, ""); err != nil {
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
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected domerr.ErrNotFound after delete, got %v", err)
	}
}

func TestTokenRepository_CreateAccess(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	repo := NewTokenRepository(testDB)
	ctx := context.Background()

	tokenID := ulid.Make().String()
	exp := time.Now().UTC().Add(15 * time.Minute)
	access := &oidc.Token{
		ID:         tokenID,
		ClientID:   "test-client",
		Subject:    "user-1",
		Audience:   []string{"test-client"},
		Scopes:     []string{"openid"},
		Expiration: exp,
	}
	if err := repo.CreateAccess(ctx, access); err != nil {
		t.Fatalf("CreateAccess: %v", err)
	}

	tok, err := repo.GetByID(ctx, tokenID)
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
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	_, err := testDB.ExecContext(ctx,
		`INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "user@example.com")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewTokenRepository(testDB)
	accessExp := time.Now().UTC().Add(15 * time.Minute)
	refreshExp := time.Now().UTC().Add(7 * 24 * time.Hour)
	authTime := time.Now().UTC()

	accessID := ulid.Make().String()
	refreshID := ulid.Make().String()
	refreshTokenValue := ulid.Make().String()

	access := &oidc.Token{
		ID:             accessID,
		ClientID:       "test-client",
		Subject:        userID,
		Audience:       []string{"test-client"},
		Scopes:         []string{"openid", "email"},
		Expiration:     accessExp,
		RefreshTokenID: refreshID,
	}
	refresh := &oidc.RefreshToken{
		ID:            refreshID,
		Token:         refreshTokenValue,
		ClientID:      "test-client",
		UserID:        userID,
		Audience:      []string{"test-client"},
		Scopes:        []string{"openid", "email"},
		AuthTime:      authTime,
		AMR:           []string{"federated"},
		AccessTokenID: accessID,
		Expiration:    refreshExp,
	}

	if err := repo.CreateAccessAndRefresh(ctx, access, refresh, ""); err != nil {
		t.Fatalf("CreateAccessAndRefresh: %v", err)
	}

	rt, err := repo.GetRefreshToken(ctx, refreshTokenValue)
	if err != nil {
		t.Fatalf("GetRefreshToken: %v", err)
	}
	if rt.ClientID != "test-client" {
		t.Errorf("expected client test-client, got %s", rt.ClientID)
	}
	if rt.UserID != userID {
		t.Errorf("expected user %s, got %s", userID, rt.UserID)
	}

	rtUserID, rtID, err := repo.GetRefreshInfo(ctx, refreshTokenValue)
	if err != nil {
		t.Fatalf("GetRefreshInfo: %v", err)
	}
	if rtUserID != userID {
		t.Errorf("expected user %s, got %s", userID, rtUserID)
	}
	if rtID == "" {
		t.Error("expected non-empty refresh token ID")
	}

	// Rotation: create new tokens, delete old refresh
	accessID2 := ulid.Make().String()
	refreshID2 := ulid.Make().String()
	refreshTokenValue2 := ulid.Make().String()

	access2 := &oidc.Token{
		ID:             accessID2,
		ClientID:       "test-client",
		Subject:        userID,
		Audience:       []string{"test-client"},
		Scopes:         []string{"openid"},
		Expiration:     accessExp,
		RefreshTokenID: refreshID2,
	}
	refresh2 := &oidc.RefreshToken{
		ID:            refreshID2,
		Token:         refreshTokenValue2,
		ClientID:      "test-client",
		UserID:        userID,
		Audience:      []string{"test-client"},
		Scopes:        []string{"openid"},
		AuthTime:      authTime,
		AMR:           []string{"federated"},
		AccessTokenID: accessID2,
		Expiration:    refreshExp,
	}

	if err := repo.CreateAccessAndRefresh(ctx, access2, refresh2, refreshTokenValue); err != nil {
		t.Fatalf("CreateAccessAndRefresh (rotation): %v", err)
	}

	_, err = repo.GetRefreshToken(ctx, refreshTokenValue)
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected old refresh token to be deleted, got %v", err)
	}
}

func TestTokenRepository_Revoke(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	repo := NewTokenRepository(testDB)
	ctx := context.Background()

	tokenID := ulid.Make().String()
	exp := time.Now().UTC().Add(15 * time.Minute)
	access := &oidc.Token{
		ID:         tokenID,
		ClientID:   "test-client",
		Subject:    "user-1",
		Audience:   []string{"test-client"},
		Scopes:     []string{"openid"},
		Expiration: exp,
	}
	if err := repo.CreateAccess(ctx, access); err != nil {
		t.Fatalf("CreateAccess: %v", err)
	}

	if err := repo.Revoke(ctx, tokenID, "test-client"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	_, err := repo.GetByID(ctx, tokenID)
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected token to be revoked, got %v", err)
	}
}

func TestTokenRepository_DeleteByUserAndClient(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	_, err := testDB.ExecContext(ctx,
		`INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "user@example.com")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewTokenRepository(testDB)
	exp := time.Now().UTC().Add(15 * time.Minute)
	refreshExp := time.Now().UTC().Add(7 * 24 * time.Hour)
	authTime := time.Now().UTC()

	accessID := ulid.Make().String()
	refreshID := ulid.Make().String()
	refreshTokenValue := ulid.Make().String()

	access := &oidc.Token{
		ID:             accessID,
		ClientID:       "test-client",
		Subject:        userID,
		Audience:       []string{"test-client"},
		Scopes:         []string{"openid"},
		Expiration:     exp,
		RefreshTokenID: refreshID,
	}
	refresh := &oidc.RefreshToken{
		ID:            refreshID,
		Token:         refreshTokenValue,
		ClientID:      "test-client",
		UserID:        userID,
		Audience:      []string{"test-client"},
		Scopes:        []string{"openid"},
		AuthTime:      authTime,
		AMR:           []string{"fed"},
		AccessTokenID: accessID,
		Expiration:    refreshExp,
	}

	if err := repo.CreateAccessAndRefresh(ctx, access, refresh, ""); err != nil {
		t.Fatal(err)
	}

	if err := repo.DeleteByUserAndClient(ctx, userID, "test-client"); err != nil {
		t.Fatalf("DeleteByUserAndClient: %v", err)
	}

	_, err = repo.GetByID(ctx, accessID)
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected access token deleted, got %v", err)
	}
	_, err = repo.GetRefreshToken(ctx, refreshTokenValue)
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected refresh token deleted, got %v", err)
	}
}

func TestTokenRepository_CreateAccessAndRefresh_DoubleRotation(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	_, err := testDB.ExecContext(ctx,
		`INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "user@example.com")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewTokenRepository(testDB)
	accessExp := time.Now().UTC().Add(15 * time.Minute)
	refreshExp := time.Now().UTC().Add(7 * 24 * time.Hour)
	authTime := time.Now().UTC()

	tokenHash := "initialhash"
	accessID := ulid.Make().String()
	refreshID := ulid.Make().String()

	access := &oidc.Token{
		ID:             accessID,
		ClientID:       "test-client",
		Subject:        userID,
		Audience:       []string{"test-client"},
		Scopes:         []string{"openid"},
		Expiration:     accessExp,
		RefreshTokenID: refreshID,
	}
	refresh := &oidc.RefreshToken{
		ID:            refreshID,
		Token:         tokenHash,
		ClientID:      "test-client",
		UserID:        userID,
		Audience:      []string{"test-client"},
		Scopes:        []string{"openid"},
		AuthTime:      authTime,
		AMR:           []string{"fed"},
		AccessTokenID: accessID,
		Expiration:    refreshExp,
	}

	if err := repo.CreateAccessAndRefresh(ctx, access, refresh, ""); err != nil {
		t.Fatalf("initial CreateAccessAndRefresh: %v", err)
	}

	// First rotation — should succeed, consuming tokenHash.
	access2 := &oidc.Token{
		ID:             ulid.Make().String(),
		ClientID:       "test-client",
		Subject:        userID,
		Audience:       []string{"test-client"},
		Scopes:         []string{"openid"},
		Expiration:     accessExp,
		RefreshTokenID: ulid.Make().String(),
	}
	access2.RefreshTokenID = ulid.Make().String()
	refresh2 := &oidc.RefreshToken{
		ID:            access2.RefreshTokenID,
		Token:         "secondhash",
		ClientID:      "test-client",
		UserID:        userID,
		Audience:      []string{"test-client"},
		Scopes:        []string{"openid"},
		AuthTime:      authTime,
		AMR:           []string{"fed"},
		AccessTokenID: access2.ID,
		Expiration:    refreshExp,
	}
	if err := repo.CreateAccessAndRefresh(ctx, access2, refresh2, tokenHash); err != nil {
		t.Fatalf("first rotation: %v", err)
	}

	// Second rotation with the same original tokenHash — must fail (already consumed).
	access3 := &oidc.Token{
		ID:             ulid.Make().String(),
		ClientID:       "test-client",
		Subject:        userID,
		Audience:       []string{"test-client"},
		Scopes:         []string{"openid"},
		Expiration:     accessExp,
		RefreshTokenID: ulid.Make().String(),
	}
	refresh3 := &oidc.RefreshToken{
		ID:            access3.RefreshTokenID,
		Token:         "thirdhash",
		ClientID:      "test-client",
		UserID:        userID,
		Audience:      []string{"test-client"},
		Scopes:        []string{"openid"},
		AuthTime:      authTime,
		AMR:           []string{"fed"},
		AccessTokenID: access3.ID,
		Expiration:    refreshExp,
	}
	err = repo.CreateAccessAndRefresh(ctx, access3, refresh3, tokenHash)
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected ErrNotFound for double rotation, got %v", err)
	}
}

func TestTokenRepository_Revoke_WrongClientID(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	repo := NewTokenRepository(testDB)
	ctx := context.Background()

	tokenID := ulid.Make().String()
	exp := time.Now().UTC().Add(15 * time.Minute)
	access := &oidc.Token{
		ID:         tokenID,
		ClientID:   "test-client",
		Subject:    "user-1",
		Audience:   []string{"test-client"},
		Scopes:     []string{"openid"},
		Expiration: exp,
	}
	if err := repo.CreateAccess(ctx, access); err != nil {
		t.Fatalf("CreateAccess: %v", err)
	}

	err := repo.Revoke(ctx, tokenID, "other-client")
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected ErrNotFound for wrong clientID, got %v", err)
	}

	// Token must still exist.
	tok, err := repo.GetByID(ctx, tokenID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if tok.ID != tokenID {
		t.Errorf("expected token %s to survive wrong-client revoke", tokenID)
	}
}

func TestTokenRepository_DeleteExpired(t *testing.T) {
	testhelper.CleanTables(t, testDB)
	ctx := context.Background()

	userID := ulid.Make().String()
	_, err := testDB.ExecContext(ctx,
		`INSERT INTO users (id, email) VALUES ($1, $2)`, userID, "user@example.com")
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewTokenRepository(testDB)
	past := time.Now().UTC().Add(-1 * time.Hour)
	future := time.Now().UTC().Add(time.Hour)
	authTime := time.Now().UTC()

	// Expired access token.
	expiredAccessID := ulid.Make().String()
	if err := repo.CreateAccess(ctx, &oidc.Token{
		ID:         expiredAccessID,
		ClientID:   "test-client",
		Subject:    userID,
		Audience:   []string{"test-client"},
		Scopes:     []string{"openid"},
		Expiration: past,
	}); err != nil {
		t.Fatalf("CreateAccess (expired): %v", err)
	}

	// Active access token.
	activeAccessID := ulid.Make().String()
	if err := repo.CreateAccess(ctx, &oidc.Token{
		ID:         activeAccessID,
		ClientID:   "test-client",
		Subject:    userID,
		Audience:   []string{"test-client"},
		Scopes:     []string{"openid"},
		Expiration: future,
	}); err != nil {
		t.Fatalf("CreateAccess (active): %v", err)
	}

	// Expired refresh token.
	expiredRefreshID := ulid.Make().String()
	if err := repo.createRefreshForTest(ctx, expiredRefreshID, userID, past, authTime); err != nil {
		t.Fatalf("create expired refresh: %v", err)
	}

	// Active refresh token.
	activeRefreshID := ulid.Make().String()
	if err := repo.createRefreshForTest(ctx, activeRefreshID, userID, future, authTime); err != nil {
		t.Fatalf("create active refresh: %v", err)
	}

	access, refresh, err := repo.DeleteExpired(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if access != 1 {
		t.Errorf("expected 1 expired access token deleted, got %d", access)
	}
	if refresh != 1 {
		t.Errorf("expected 1 expired refresh token deleted, got %d", refresh)
	}

	// Active tokens must remain.
	if _, err := repo.GetByID(ctx, activeAccessID); err != nil {
		t.Errorf("expected active access token to remain: %v", err)
	}
}

// createRefreshForTest inserts a refresh token with arbitrary expiration directly
// for testing purposes, bypassing the service layer's access-token pairing requirement.
func (r *TokenRepository) createRefreshForTest(ctx context.Context, id, userID string, expiration, authTime time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, token_hash, client_id, user_id, audience, scopes, auth_time, amr, expiration)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, "hash-"+id, "test-client", userID,
		`{"test-client"}`, `{"openid"}`,
		authTime, `{"fed"}`, expiration,
	)
	return err
}
