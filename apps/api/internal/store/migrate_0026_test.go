package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0026ProjectStatusCheck: after Up, project.status is a closed
// enum — 'active'/'finished' accepted, anything else rejected. Existing rows
// (all 'active') survive the validated CHECK.
func TestMigration0026ProjectStatusCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t) // migrates to head (0026)

	// A seeded project already exists as 'active' and survived the Up.
	var active int
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title, status) VALUES ($1, 'a3-finish', 'finished')
		RETURNING 1`, refactor2SeededStudentID).Scan(&active); err != nil {
		t.Fatalf("'finished' must satisfy project_status_ck: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO project (user_id, title, status) VALUES ($1, 'a3-bogus', 'archived')`,
		refactor2SeededStudentID); err == nil {
		t.Fatal("status='archived' should violate project_status_ck, got no error")
	}
}

// TestMigration0026Down removes the CHECK, restoring the pre-A3 free-text
// column. newTestPool migrates to head (0026); DownTo(25) reverses only 0026.
func TestMigration0026Down(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.DownToContext(ctx, db, "migrations", 25); err != nil {
		t.Fatalf("goose down to v25 (reversing 0026): %v", err)
	}

	// The CHECK is gone: a previously-illegal status now inserts fine.
	if _, err := pool.Exec(ctx, `
		INSERT INTO project (user_id, title, status) VALUES ($1, 'a3-down', 'archived')`,
		refactor2SeededStudentID); err != nil {
		t.Fatalf("after Down, free-text status must insert: %v", err)
	}

	// And Up restores the CHECK.
	if err := goose.UpContext(ctx, db, "migrations"); err == nil {
		t.Fatal("re-Up over an 'archived' row must fail the validated CHECK, got no error")
	}
}
