package adapter

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
	"github.com/oklog/ulid/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/crypto/bcrypt"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
	identitypg "github.com/barn0w1/hss-science/server/services/accounts/internal/identity/postgres"
	oidcdom "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
	oidcpg "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc/postgres"
	"github.com/barn0w1/hss-science/server/services/accounts/testhelper"
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

	if err := testhelper.RunMigrations(storageTestDB); err != nil {
		panic("failed to run migrations: " + err.Error())
	}

	os.Exit(m.Run())
}

func newTestAdapter(t *testing.T) *StorageAdapter {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	sk := NewSigningKey(key)
	pks := NewPublicKeySet(key, nil)

	authReqRepo := oidcpg.NewAuthRequestRepository(storageTestDB)
	clientRepo := oidcpg.NewClientRepository(storageTestDB)
	tokenRepo := oidcpg.NewTokenRepository(storageTestDB)
	authReqSvc := oidcdom.NewAuthRequestService(authReqRepo, 30*time.Minute)
	clientSvc := oidcdom.NewClientService(clientRepo)
	tokenSvc := oidcdom.NewTokenService(tokenRepo)
	identitySvc := identity.NewService(identitypg.NewUserRepository(storageTestDB))

	return NewStorageAdapter(&testUserClaimsBridge{svc: identitySvc}, authReqSvc, clientSvc, tokenSvc,
		sk, pks, 15*time.Minute, 7*24*time.Hour, storageTestDB.PingContext)
}

type testUserClaimsBridge struct {
	svc identity.Service
}

func (b *testUserClaimsBridge) UserClaims(ctx context.Context, userID string) (*UserClaims, error) {
	user, err := b.svc.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &UserClaims{
		Subject:       user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		Name:          user.Name,
		GivenName:     user.GivenName,
		FamilyName:    user.FamilyName,
		Picture:       user.Picture,
		UpdatedAt:     user.UpdatedAt,
	}, nil
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

type testUser struct {
	ID string
}

func seedTestUser(t *testing.T) *testUser {
	t.Helper()
	user := &testUser{
		ID: ulid.Make().String(),
	}
	_, err := storageTestDB.Exec(
		`INSERT INTO users (id, email, email_verified, name, given_name, family_name, picture) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		user.ID, "test@example.com", true, "Test User", "Test", "User", "https://example.com/pic.jpg",
	)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func TestStorage_CreateAuthRequest(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	_, err := s.AuthRequestByID(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error for non-existent auth request")
	}
}

func TestStorage_SaveAuthCode_AuthRequestByCode(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	_, err := s.GetClientByClientID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent client")
	}
}

func TestStorage_AuthorizeClientIDSecret(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	ctx := context.Background()

	ar := &AuthRequest{domain: &oidcdom.AuthRequest{
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{domain: &oidcdom.AuthRequest{
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{domain: &oidcdom.AuthRequest{
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	_, err := s.TokenRequestByRefreshToken(context.Background(), "nonexistent")
	if !errors.Is(err, op.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
	}
}

func TestStorage_RevokeToken(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	ctx := context.Background()

	ar := &AuthRequest{domain: &oidcdom.AuthRequest{
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{domain: &oidcdom.AuthRequest{
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	userinfo := &oidc.UserInfo{}
	err := s.SetUserinfoFromScopes(context.Background(), userinfo, uuid.New().String(), "test-client", []string{oidc.ScopeOpenID})
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

func TestStorage_SetIntrospectionFromToken(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{domain: &oidcdom.AuthRequest{
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	key, err := s.SigningKey(context.Background())
	if err != nil {
		t.Fatalf("SigningKey: %v", err)
	}
	if key.ID() == "" {
		t.Error("expected non-empty key ID")
	}
}

func TestStorage_KeySet(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	keys, err := s.KeySet(context.Background())
	if err != nil {
		t.Fatalf("KeySet: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}
}

func TestStorage_Health(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	if err := s.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
}

func TestStorage_GetPrivateClaimsFromScopes(t *testing.T) {
	s := newTestAdapter(t)
	claims, err := s.GetPrivateClaimsFromScopes(context.Background(), "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(claims) != 0 {
		t.Errorf("expected empty claims map, got %v", claims)
	}
}

func TestStorage_GetKeyByIDAndClientID(t *testing.T) {
	s := newTestAdapter(t)
	_, err := s.GetKeyByIDAndClientID(context.Background(), "kid", "client")
	if err == nil {
		t.Fatal("expected error for unsupported jwt profile grant")
	}
}

func TestStorage_ValidateJWTProfileScopes(t *testing.T) {
	s := newTestAdapter(t)
	_, err := s.ValidateJWTProfileScopes(context.Background(), "user", []string{"openid"})
	if err == nil {
		t.Fatal("expected error for unsupported jwt profile grant")
	}
}

func TestStorage_GetRefreshTokenInfo(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{domain: &oidcdom.AuthRequest{
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	_, _, err := s.GetRefreshTokenInfo(context.Background(), "client", "nonexistent")
	if !errors.Is(err, op.ErrInvalidRefreshToken) {
		t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
	}
}

func TestStorage_ClientCredentials(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
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
	s := newTestAdapter(t)
	req, err := s.ClientCredentialsTokenRequest(context.Background(), "cc-client", []string{"openid"})
	if err != nil {
		t.Fatalf("ClientCredentialsTokenRequest: %v", err)
	}
	if req.GetSubject() != "cc-client" {
		t.Errorf("expected subject cc-client, got %s", req.GetSubject())
	}
}

func TestStorage_SetUserinfoFromToken(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	ctx := context.Background()
	user := seedTestUser(t)

	ar := &AuthRequest{domain: &oidcdom.AuthRequest{
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
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	userinfo := &oidc.UserInfo{}
	err := s.SetUserinfoFromToken(context.Background(), userinfo, uuid.New().String(), "user-1", "test-client")
	if err == nil {
		t.Fatal("expected error for non-existent token")
	}
}

func TestStorage_CreateAuthRequest_PKCERequired(t *testing.T) {
	testhelper.CleanTables(t, storageTestDB)
	s := newTestAdapter(t)
	ctx := context.Background()

	_, err := storageTestDB.Exec(
		`INSERT INTO clients (id, secret_hash, redirect_uris, response_types, grant_types, access_token_type, auth_method)
		 VALUES ($1, $2, $3, $4, $5, $6, 'none')`,
		"public-client", "",
		`{"https://app.example.com/callback"}`,
		`{"code"}`,
		`{"authorization_code"}`,
		"jwt",
	)
	if err != nil {
		t.Fatalf("seed public client: %v", err)
	}

	authReq := &oidc.AuthRequest{
		ClientID:     "public-client",
		RedirectURI:  "https://app.example.com/callback",
		Scopes:       oidc.SpaceDelimitedArray{"openid"},
		ResponseType: oidc.ResponseTypeCode,
	}

	_, err = s.CreateAuthRequest(ctx, authReq, "")
	if err == nil {
		t.Fatal("expected error for public client without PKCE")
	}
	if !errors.As(err, new(*oidc.Error)) {
		t.Fatalf("expected oidc.Error, got %T: %v", err, err)
	}
}
