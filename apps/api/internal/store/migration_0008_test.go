package store

// Version-targeted test for migration 0008 (0008_eval_trigger.sql).
//
// Goal: prove that the self-healing UPDATE in 0008 demotes all-but-the-most-recent
// in-flight eval per task before building the partial unique index, so the migration
// succeeds even when pre-existing data has ≥2 queued/running rows for one task.
//
// The test lives in package store (not store_test) so it can access the unexported
// migrationFS embed that goose needs.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// newBareTestPool starts a throwaway Postgres container and returns a connected pool
// WITHOUT running any migrations. The container is terminated via t.Cleanup.
func newBareTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("mindimprint"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestMigration0008_DemotesInflightDuplicates exercises the self-healing path:
//
//  1. Apply migrations 1-7 (no trigger/trigger_milestone columns exist yet).
//  2. Seed two queued evaluations for the same task — simulating the pre-guard state
//     that could exist on a live database.
//  3. Apply migration 8.
//  4. Assert exactly one in-flight row remains (older one demoted to 'failed').
//  5. Assert the partial unique index was created.
//  6. Assert a second in-flight insert for the same task now fails with SQLSTATE 23505.
//
// RED scenario: without the demotion UPDATE, step 3 would abort — goose.UpToContext
// returns an error because CREATE UNIQUE INDEX finds duplicate (task_id) values among
// the WHERE-filtered rows and the test fails at t.Fatalf("goose up to 8").
func TestMigration0008_DemotesInflightDuplicates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()

	pool := newBareTestPool(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })

	// Configure goose (global state; safe because -p 1 runs packages sequentially).
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose set dialect: %v", err)
	}

	// ── Step 1: apply migrations 1-7 ──────────────────────────────────────────
	// At v7 the evaluations table has no trigger/trigger_milestone columns.
	if err := goose.UpToContext(ctx, db, "migrations", 7); err != nil {
		t.Fatalf("goose up to 7: %v", err)
	}

	// ── Step 2: seed duplicate in-flight evaluations ───────────────────────────
	// Migrations 1-7 include 0002_seed.sql which inserts the demo student.
	const seededStudentID = "00000000-0000-0000-0000-000000000003"

	var taskID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO tasks (user_id, title) VALUES ($1, 'migration-0008-test') RETURNING id`,
		seededStudentID,
	).Scan(&taskID); err != nil {
		t.Fatalf("insert task: %v", err)
	}

	// Insert the older evaluation first (explicit timestamp to guarantee ordering).
	if _, err := pool.Exec(ctx,
		`INSERT INTO evaluations (task_id, scores, narrative, model, tier, status, created_at)
		 VALUES ($1, '[]', '', 'deepseek-reasoner', 'flagship', 'queued', now() - interval '1 minute')`,
		taskID); err != nil {
		t.Fatalf("insert older eval: %v", err)
	}

	// Insert the newer evaluation (uses now() default).
	if _, err := pool.Exec(ctx,
		`INSERT INTO evaluations (task_id, scores, narrative, model, tier, status)
		 VALUES ($1, '[]', '', 'deepseek-reasoner', 'flagship', 'queued')`,
		taskID); err != nil {
		t.Fatalf("insert newer eval: %v", err)
	}

	// Confirm pre-migration duplicate state.
	var before int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM evaluations WHERE status IN ('queued','running') AND task_id = $1`,
		taskID).Scan(&before); err != nil {
		t.Fatalf("pre-migration count: %v", err)
	}
	if before != 2 {
		t.Fatalf("pre-migration in-flight count = %d, want 2", before)
	}

	// ── Step 3: apply migration 8 ─────────────────────────────────────────────
	// The self-healing UPDATE must demote the older duplicate before the index builds.
	if err := goose.UpToContext(ctx, db, "migrations", 8); err != nil {
		t.Fatalf("goose up to 8: %v", err)
	}

	// ── Step 4: exactly one in-flight row survives ─────────────────────────────
	var after int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM evaluations WHERE status IN ('queued','running') AND task_id = $1`,
		taskID).Scan(&after); err != nil {
		t.Fatalf("post-migration in-flight count: %v", err)
	}
	if after != 1 {
		t.Fatalf("post-migration in-flight count = %d, want 1", after)
	}

	// The demoted row must be 'failed' with the expected reason.
	var failedCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM evaluations
		 WHERE status = 'failed' AND error = '被单评估在途约束取代' AND task_id = $1`,
		taskID).Scan(&failedCount); err != nil {
		t.Fatalf("failed count: %v", err)
	}
	if failedCount != 1 {
		t.Fatalf("demoted-to-failed count = %d, want 1", failedCount)
	}

	// ── Step 5: unique index was created ──────────────────────────────────────
	var indexExists bool
	if err := pool.QueryRow(ctx,
		`SELECT to_regclass('evaluations_one_inflight_per_task') IS NOT NULL`,
	).Scan(&indexExists); err != nil {
		t.Fatalf("check index existence: %v", err)
	}
	if !indexExists {
		t.Fatal("evaluations_one_inflight_per_task index not found after migration 8")
	}

	// ── Step 6: second in-flight insert now rejected with 23505 ───────────────
	// trigger has DEFAULT 'manual' after migration 8; omitting it is fine.
	_, insertErr := pool.Exec(ctx,
		`INSERT INTO evaluations (task_id, scores, narrative, model, tier, status)
		 VALUES ($1, '[]', '', 'deepseek-reasoner', 'flagship', 'queued')`,
		taskID)
	var pgErr *pgconn.PgError
	if !errors.As(insertErr, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("second in-flight insert: err = %v, want SQLSTATE 23505 unique violation", insertErr)
	}
}
