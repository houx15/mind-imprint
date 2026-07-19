package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigrate0027ClearsEvaluations: DualAxis replaces the flat 10-dim report
// shape. The product is not in use, so existing (flat-shaped) evaluation rows
// cannot be upgraded — 0027 clears them out entirely. newTestPool migrates to
// head (0027 already applied to a fresh, empty table), so this test reverses
// to v26 first, seeds a row against a real project, then re-applies 0027 and
// asserts the table is empty again. Mirrors TestMigration0026Down's
// stdlib.OpenDBFromPool + goose Down/Up pattern.
func TestMigrate0027ClearsEvaluations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t) // migrates to head (0027)

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.DownToContext(ctx, db, "migrations", 26); err != nil {
		t.Fatalf("goose down to v26 (reversing 0027): %v", err)
	}

	// Seed one evaluation row against a seeded project (valid project_id FK).
	var projectID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, 'a3-clean-slate') RETURNING id`,
		refactor2SeededStudentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'x', 'deepseek-v4-pro', 'flagship', 'done')`,
		projectID); err != nil {
		t.Fatalf("seed evaluation: %v", err)
	}

	var before int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM evaluations`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before == 0 {
		t.Fatal("seed failed: no evaluation row before 0027")
	}

	if err := goose.UpContext(ctx, db, "migrations"); err != nil { // re-apply 0027
		t.Fatalf("apply 0027: %v", err)
	}

	var after int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM evaluations`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != 0 {
		t.Fatalf("evaluations after 0027 = %d, want 0", after)
	}
}
