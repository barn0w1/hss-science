package testhelper

import (
	"fmt"
	"io/fs"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/barn0w1/hss-science/server/services/accounts/migrations"
)

func RunMigrations(db *sqlx.DB) error {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read migration dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := fs.ReadFile(migrations.FS, entry.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("exec %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func CleanTables(t testing.TB, db *sqlx.DB) {
	t.Helper()
	for _, table := range []string{"refresh_tokens", "tokens", "auth_requests", "federated_identities", "users", "clients"} {
		if _, err := db.Exec("DELETE FROM " + table); err != nil {
			t.Fatalf("failed to clean table %s: %v", table, err)
		}
	}
}
