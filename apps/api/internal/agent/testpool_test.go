package agent_test

// testpool_test.go — shared testcontainers Postgres bootstrap for this
// package's *_sqlc_test.go files. Previously lived in turn_test.go
// (RunTurn's own test file); RunTurn was retired in Slice 5d, but several
// unrelated sqlc-backed tests (agentstore/loop/refactor2) still need a real
// pool, so this helper survives on its own.

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

// seededStudentID is the fixed UUID from migration 0002_seed.sql — shared by
// several *_sqlc_test.go fixtures in this package.
var seededStudentID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

// newTurnTestPool spins up a throwaway Postgres, runs all migrations, and
// returns the pool. Container/pool torn down via t.Cleanup.
func newTurnTestPool(t *testing.T) *pgxpool.Pool {
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
