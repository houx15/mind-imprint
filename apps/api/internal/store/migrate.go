package store

import (
	"context"
	"embed"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// RunMigrations applies all pending goose migrations to the pool's database.
// It opens a short-lived database/sql handle over the same pgx config (goose
// needs database/sql), runs "up", then closes it. The pgxpool is untouched.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return err
	}

	// Apply river's own schema (river_job et al.) so every caller that runs
	// migrations gets the job-queue tables. Idempotent via river's versioning.
	return RunRiverMigrations(ctx, pool)
}
