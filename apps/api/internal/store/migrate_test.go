package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestPool returns a pool onto a fresh, fully migrated database of its own,
// dropped via t.Cleanup. Backed by the package's single shared container (see
// testdb_internal_test.go): migrations run once into a template, and each call
// clones it — same isolation as the old container-per-test, far cheaper.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return NewTestDB(t)
}

func TestMigrationsCreateTablesAndSeed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	wantTables := []string{
		"schools", "classes", "users", "enrollments",
		"tasks", "messages", "card_instances", "evaluations",
		"sessions", "email_verification_tokens",
	}
	for _, name := range wantTables {
		var exists bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables
			 WHERE table_schema='public' AND table_name=$1)`, name).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", name, err)
		}
		if !exists {
			t.Fatalf("table %s missing after migration", name)
		}
	}

	// llm_usage view exists.
	var viewExists bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.views
		 WHERE table_schema='public' AND table_name='llm_usage')`).Scan(&viewExists); err != nil {
		t.Fatalf("check view: %v", err)
	}
	if !viewExists {
		t.Fatal("llm_usage view missing")
	}

	// Seeded student present and org-bound; school matches the class's school.
	var (
		studentSchool string
		classSchool   string
		role          string
	)
	err := pool.QueryRow(ctx, `
		SELECT u.school_id::text, c.school_id::text, e.role_in_class
		  FROM users u
		  JOIN enrollments e ON e.user_id = u.id
		  JOIN classes c ON c.id = e.class_id
		 WHERE u.email = 'phoebe@demo.mindimprint.local'`).
		Scan(&studentSchool, &classSchool, &role)
	if err != nil {
		t.Fatalf("seeded student/enrollment not found: %v", err)
	}
	if studentSchool != classSchool {
		t.Fatalf("invariant broken: user.school_id %s != class.school_id %s", studentSchool, classSchool)
	}
	if role != "student" {
		t.Fatalf("role_in_class = %q, want student", role)
	}
}

func TestMigration0020MaterialSourceLog(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	// material.task_id is nullable — a project material needs no task.
	var isNullable string
	if err := pool.QueryRow(ctx,
		`SELECT is_nullable FROM information_schema.columns
		 WHERE table_name='material' AND column_name='task_id'`).Scan(&isNullable); err != nil {
		t.Fatal(err)
	}
	if isNullable != "YES" {
		t.Fatalf("material.task_id is_nullable = %q, want YES", isNullable)
	}

	// source_log_entry.material_id exists and points at material.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.columns
		 WHERE table_name='source_log_entry' AND column_name='material_id'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("source_log_entry.material_id missing")
	}

	// The seeded demo materials carry real blocks — not '[]'.
	var blocks []byte
	if err := pool.QueryRow(ctx,
		`SELECT blocks FROM material WHERE id = '00000000-0000-0000-0000-000000000110'`).Scan(&blocks); err != nil {
		t.Fatal(err)
	}
	var parsed []map[string]any
	if err := json.Unmarshal(blocks, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed) < 3 {
		t.Fatalf("seeded blog material has %d blocks, want >= 3 (the real article text)", len(parsed))
	}

	// And each seeded material has a source-log entry.
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM source_log_entry WHERE project_id = '00000000-0000-0000-0000-000000000101'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("seeded source_log_entry count = %d, want 2", count)
	}
}
