package accounts

import "embed"

// MigrationsFS provides access to the embedded SQL migration files.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
