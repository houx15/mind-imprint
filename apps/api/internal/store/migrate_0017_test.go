package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigrate0017DownUpRoundTrip: 0017 relaxed graph_node.type from a fixed
// 5-value enum to an open, skill-defined vocabulary (rubric_translation,
// perspective, preregistration, provisional_answer, concession, reflection,
// …) because intake mints skill-declared artifact types the DB schema must
// not enumerate. Its Down re-adds the old CHECK (type IN
// ('claim','evidence','plan','gate_state','note')) — which is exactly the
// fragile part: any graph_node row of an open type (e.g. 'perspective',
// produced by the real evaluate_perspectives station) violates that CHECK
// when Postgres validates it during ADD CONSTRAINT. newTestPool migrates to
// head; we seed a project + a 'perspective' graph_node row (an open type,
// mirroring real production data) before reversing past 0017, so the fragile
// path is actually exercised rather than merely reached.
func TestMigrate0017DownUpRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t) // migrates to head (past 0017)

	var projectID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, 'n6-0017-down') RETURNING id`,
		refactor2SeededStudentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO graph_node (project_id, type, author, body)
		VALUES ($1, 'perspective', 'student', '{"text":"国家视角"}'::jsonb)`,
		projectID); err != nil {
		t.Fatalf("seed open-type graph_node: %v", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}

	// migrate down past 0017, then back up — must not error even with
	// graph_node rows of the type 0017 introduced.
	if err := goose.DownToContext(ctx, db, "migrations", 16); err != nil {
		t.Fatalf("down to 16: %v", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("up: %v", err)
	}
}
