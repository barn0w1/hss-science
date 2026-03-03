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

	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
	"github.com/barn0w1/hss-science/server/services/accounts/testhelper"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgC, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("identity_test"),
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

func newID() string { return ulid.Make().String() }

func cleanTables(t *testing.T) {
	t.Helper()
	testhelper.CleanTables(t, testDB)
}

func TestGetByID_Found(t *testing.T) {
	cleanTables(t)
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	user := &identity.User{
		ID: newID(), Email: "alice@example.com", EmailVerified: true,
		Name: "Alice Smith", GivenName: "Alice", FamilyName: "Smith",
		Picture: "https://example.com/alice.jpg", CreatedAt: now,
	}
	fi := &identity.FederatedIdentity{
		ID: newID(), UserID: user.ID, Provider: "google", ProviderSubject: "goog-1",
		ProviderEmail: "alice@example.com", ProviderEmailVerified: true,
		ProviderDisplayName: "Alice Smith", ProviderGivenName: "Alice",
		ProviderFamilyName: "Smith", ProviderPictureURL: "https://example.com/alice.jpg",
		LastLoginAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateWithFederatedIdentity(ctx, user, fi); err != nil {
		t.Fatalf("CreateWithFederatedIdentity: %v", err)
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
	if got.GivenName != "Alice" {
		t.Errorf("expected Alice, got %s", got.GivenName)
	}
	if got.FamilyName != "Smith" {
		t.Errorf("expected Smith, got %s", got.FamilyName)
	}
	if got.Picture != "https://example.com/alice.jpg" {
		t.Errorf("expected pic URL, got %s", got.Picture)
	}
	if !got.EmailVerified {
		t.Error("expected EmailVerified to be true")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	cleanTables(t)
	repo := NewUserRepository(testDB)
	_, err := repo.GetByID(context.Background(), newID())
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFindByFederatedIdentity_Found(t *testing.T) {
	cleanTables(t)
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	user := &identity.User{
		ID: newID(), Email: "bob@example.com", Name: "Bob", CreatedAt: now,
	}
	fi := &identity.FederatedIdentity{
		ID: newID(), UserID: user.ID, Provider: "github", ProviderSubject: "gh-42",
		ProviderEmail: "bob@example.com",
		LastLoginAt:   now, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateWithFederatedIdentity(ctx, user, fi); err != nil {
		t.Fatalf("CreateWithFederatedIdentity: %v", err)
	}

	got, err := repo.FindByFederatedIdentity(ctx, "github", "gh-42")
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

func TestFindByFederatedIdentity_NotFound(t *testing.T) {
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

func TestUpdateFederatedIdentityClaims(t *testing.T) {
	cleanTables(t)
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	user := &identity.User{
		ID: newID(), Email: "carol@example.com", EmailVerified: true,
		Name: "Carol", CreatedAt: now,
	}
	fi := &identity.FederatedIdentity{
		ID: newID(), UserID: user.ID, Provider: "google", ProviderSubject: "goog-99",
		ProviderEmail: "carol@example.com", ProviderEmailVerified: true,
		ProviderDisplayName: "Carol", LastLoginAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateWithFederatedIdentity(ctx, user, fi); err != nil {
		t.Fatalf("CreateWithFederatedIdentity: %v", err)
	}

	newClaims := identity.FederatedClaims{
		Subject:       "goog-99",
		Email:         "carol.new@example.com",
		EmailVerified: true,
		Name:          "Carol Updated",
		GivenName:     "Carol",
		FamilyName:    "Updated",
		Picture:       "https://example.com/new.jpg",
	}
	loginAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := repo.UpdateFederatedIdentityClaims(ctx, "google", "goog-99", newClaims, loginAt); err != nil {
		t.Fatalf("UpdateFederatedIdentityClaims: %v", err)
	}

	// Verify FI was updated
	var provEmail, provName string
	err := testDB.QueryRowContext(ctx,
		`SELECT provider_email, provider_display_name FROM federated_identities WHERE provider = 'google' AND provider_subject = 'goog-99'`,
	).Scan(&provEmail, &provName)
	if err != nil {
		t.Fatalf("query FI: %v", err)
	}
	if provEmail != "carol.new@example.com" {
		t.Errorf("expected updated provider_email, got %s", provEmail)
	}
	if provName != "Carol Updated" {
		t.Errorf("expected updated provider_display_name, got %s", provName)
	}

	// Verify User was NOT modified
	gotUser, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if gotUser.Email != "carol@example.com" {
		t.Errorf("User.Email should be unchanged, got %s", gotUser.Email)
	}
	if gotUser.Name != "Carol" {
		t.Errorf("User.Name should be unchanged, got %s", gotUser.Name)
	}
}

func TestUniqueFederatedIdentity_Constraint(t *testing.T) {
	cleanTables(t)
	repo := NewUserRepository(testDB)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)
	user1 := &identity.User{
		ID: newID(), Email: "dave@example.com", Name: "Dave", CreatedAt: now,
	}
	fi1 := &identity.FederatedIdentity{
		ID: newID(), UserID: user1.ID, Provider: "google", ProviderSubject: "goog-dave",
		LastLoginAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateWithFederatedIdentity(ctx, user1, fi1); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// A second user attempting to claim the same (provider, provider_subject) should
	// fail due to UNIQUE(provider, provider_subject).
	user2 := &identity.User{
		ID: newID(), Email: "eve@example.com", Name: "Eve", CreatedAt: now,
	}
	fi2 := &identity.FederatedIdentity{
		ID: newID(), UserID: user2.ID, Provider: "google", ProviderSubject: "goog-dave",
		LastLoginAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreateWithFederatedIdentity(ctx, user2, fi2); err == nil {
		t.Fatal("expected UNIQUE(provider, provider_subject) constraint violation")
	}
}
