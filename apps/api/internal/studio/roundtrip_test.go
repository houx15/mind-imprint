package studio_test

// roundtrip_test.go — Slice 5b Task 7: seeds a demo Task+Project (migration
// 0018) and verifies studio.Load + studio.Project reconstruct the expected
// StudioProjection end-to-end against real Postgres (testcontainers). This is
// the external test package (studio_test) so it needs its own migration-
// applying pool helper — same pattern as store_test.newStoreTestPool /
// agent_test.newTurnTestPool (each external test package keeps its own copy;
// there is no shared exported helper to reuse across packages).

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/studio"
)

var demoProjectID = uuid.MustParse("00000000-0000-0000-0000-000000000101")

func TestSeededProjectProjection(t *testing.T) {
	if testing.Short() {
		t.Skip("requires postgres (testcontainers)")
	}
	pool := newMigratedPool(t) // applies all migrations incl. 0018 seed
	q := sqlc.New(pool)
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("skills.ByID(writing-project) not found")
	}

	d, err := studio.Load(context.Background(), q, demoProjectID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	proj, err := studio.Project(sk, cards.ByID, d)
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	if len(proj.Stations) != 7 {
		t.Fatalf("want 7 stations, got %d", len(proj.Stations))
	}
	wantStates := []string{"done", "done", "done", "done", "current", "locked", "locked"}
	for i, w := range wantStates {
		if proj.Stations[i].State != w {
			t.Errorf("S%d = %q, want %q", i, proj.Stations[i].State, w)
		}
	}
	if proj.ActiveStation != "S4" {
		t.Errorf("activeStation = %q, want S4", proj.ActiveStation)
	}
	if proj.Coach.Anchor != "论证图 · 治理决心主张" {
		t.Errorf("anchor = %q", proj.Coach.Anchor)
	}
	if len(proj.Coach.Messages) < 2 {
		t.Errorf("coach thread = %d msgs, want >=2", len(proj.Coach.Messages))
	}
	if len(proj.Coach.Equipment) == 0 {
		t.Errorf("equipment empty")
	}
	if len(proj.Onboarding.RubricRows) != 4 {
		t.Errorf("rubric rows = %d, want 4", len(proj.Onboarding.RubricRows))
	}
}

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
