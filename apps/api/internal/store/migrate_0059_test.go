package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0059StudioStartedBackfill exercises 0059's backfill: the new
// studio_state.started flag must come up true for a project that already has
// a studio-surface conversation (mid-journey, must not be re-gated by the new
// start gate) and false for one that doesn't.
//
// newTestPool migrates straight to head (which already includes 0059), so
// this reverses just 0059 (DownTo 58), seeds the two projects under the
// PRE-0059 default (no "started" key at all — the realistic starting
// condition for every existing row), then re-applies 0059 (UpTo 59) and
// asserts the backfill landed correctly.
func TestMigration0059StudioStartedBackfill(t *testing.T) {
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

	if err := goose.DownToContext(ctx, db, "migrations", 58); err != nil {
		t.Fatalf("goose down to v58 (reversing 0059): %v", err)
	}

	// Project A: has an existing studio-surface conversation → must be
	// backfilled started=true.
	var projectA string
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, 'studio-started-a')
		RETURNING id::text`, refactor2SeededStudentID).Scan(&projectA); err != nil {
		t.Fatalf("seed project A: %v", err)
	}
	var threadID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO chat_thread (user_id, seeded_project_id) VALUES ($1, $2)
		RETURNING id::text`, refactor2SeededStudentID, projectA).Scan(&threadID); err != nil {
		t.Fatalf("seed chat_thread for project A: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO chat_message (thread_id, role, content, surface)
		VALUES ($1, 'assistant', 'let''s start writing', 'studio')`, threadID); err != nil {
		t.Fatalf("seed studio-surface chat_message: %v", err)
	}

	// Project B: no conversation at all → must stay started=false.
	var projectB string
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, 'studio-started-b')
		RETURNING id::text`, refactor2SeededStudentID).Scan(&projectB); err != nil {
		t.Fatalf("seed project B: %v", err)
	}

	if err := goose.UpToContext(ctx, db, "migrations", 59); err != nil {
		t.Fatalf("goose up to v59 (0059 default + backfill): %v", err)
	}

	var startedA string
	if err := pool.QueryRow(ctx, `SELECT studio_state->>'started' FROM project WHERE id = $1`, projectA).Scan(&startedA); err != nil {
		t.Fatalf("read project A started: %v", err)
	}
	if startedA != "true" {
		t.Fatalf("project A (has studio thread) started = %q, want \"true\"", startedA)
	}

	var startedB string
	if err := pool.QueryRow(ctx, `SELECT studio_state->>'started' FROM project WHERE id = $1`, projectB).Scan(&startedB); err != nil {
		t.Fatalf("read project B started: %v", err)
	}
	if startedB != "false" {
		t.Fatalf("project B (no studio thread) started = %q, want \"false\"", startedB)
	}
}
