package studio_test

// roundtrip_test.go carries the shared migrated-Postgres pool helper
// (newMigratedPool) and the seeded demo project id (demoProjectID, migration
// 0018) used by other external-package tests in this file's package
// (e.g. load_events_test.go's TestLoadProjectEvents). The original
// TestSeededProjectProjection here exercised studio.Project/StudioProjection
// end-to-end; that station-projection subtree was removed as dead code
// (zero non-test callers), so the test went with it — this file now keeps
// only the pool/seed plumbing other tests still depend on.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"mindimprint/api/internal/store"
)

var demoProjectID = uuid.MustParse("00000000-0000-0000-0000-000000000101")

// newMigratedPool spins up a throwaway Postgres, runs all migrations
// (including 0018's demo-project seed), and returns a connected pool. Same
// shape as store_test.newStoreTestPool / agent_test.newTurnTestPool.
func newMigratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("mindimprint"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}
	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := store.RunMigrations(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}
