package store_test

import (
	"context"
	"testing"

	"mindimprint/api/internal/store"
)

// TestRunMigrationsCreatesRiverTables verifies RunMigrations applies river's
// own schema (river_job et al.) after the goose pass, so every caller that
// migrates a fresh database gets river's tables for free.
func TestRunMigrationsCreatesRiverTables(t *testing.T) {
	ctx := context.Background()
	pool := newStoreTestPool(t)

	if err := store.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var exists bool
	if err := pool.QueryRow(ctx,
		`SELECT to_regclass('public.river_job') IS NOT NULL`,
	).Scan(&exists); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !exists {
		t.Fatalf("river_job table not created")
	}
}
