package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// seedCourse inserts a minimal course v2 row (course v2 / migration 0050
// deletes 0012's seeded course, and every column it added — structure,
// render_cache, etc — has a default, so branch/title are the only required
// fields) and returns its id. Replaces the old seededCourseID constant, which
// pointed at 0012's row and no longer resolves to anything once 0050 has run.
func seedCourse(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(ctx, `
		INSERT INTO course (branch, title) VALUES ('批判性思维', 'migrate-0031 test course')
		RETURNING id::text`).Scan(&id); err != nil {
		t.Fatalf("seed course: %v", err)
	}
	return id
}

// TestMigration0031EventCourseScope: event gains a nullable course_id,
// event_scope_ck widens to include it, and the widened check still rejects a
// brand-new fully-unscoped row while grandfathering pre-existing ones (NOT
// VALID). Mirrors TestMigration0025ThreadScope's structure for the course
// scope D2 adds. Each test gets its own dedicated testcontainers database
// (newTestPool spins up a fresh container per call), so there is no shared
// state at risk from the DownTo/Up round-trip TestMigration0031Down performs
// below.
func TestMigration0031EventCourseScope(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	courseID := seedCourse(t, ctx, pool)

	// 1. A course-scoped event satisfies event_scope_ck with only course_id.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, course_id, surface, type, payload)
		VALUES ($1, $2, 'course', 'step_viewed', '{"ordinal":0}'::jsonb)`,
		refactor2SeededStudentID, courseID); err != nil {
		t.Fatalf("course-scoped event must satisfy event_scope_ck: %v", err)
	}

	// 2. A brand-new fully-unscoped event is still rejected — 0031 only widens
	// the set of accepted scopes, it does not open a new exemption.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'course', 'step_viewed', '{}'::jsonb)`,
		refactor2SeededStudentID); err == nil {
		t.Fatal("a new unscoped event should violate event_scope_ck, got no error")
	}

	// 3. NOT VALID's contract still holds: a pre-existing unattributable row
	// reads fine. Simulate one via the only path it could exist — constraint
	// dropped, row inserted, constraint re-added NOT VALID over it.
	if _, err := pool.Exec(ctx, `ALTER TABLE event DROP CONSTRAINT event_scope_ck`); err != nil {
		t.Fatalf("drop constraint: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'studio', 'legacy_event', '{}'::jsonb)`, refactor2SeededStudentID); err != nil {
		t.Fatalf("seed legacy unattributable row: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		ALTER TABLE event ADD CONSTRAINT event_scope_ck
		CHECK (num_nonnulls(project_id, session_id, thread_id, course_id) >= 1) NOT VALID`); err != nil {
		t.Fatalf("re-add NOT VALID over a legacy row — the point of NOT VALID: %v", err)
	}
	var legacy int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM event
		WHERE surface = 'studio' AND type = 'legacy_event' AND project_id IS NULL
		  AND session_id IS NULL AND thread_id IS NULL AND course_id IS NULL`).Scan(&legacy); err != nil {
		t.Fatalf("read legacy unattributable rows: %v", err)
	}
	if legacy != 1 {
		t.Fatalf("legacy unattributable rows = %d, want 1 (grandfathered, never deleted)", legacy)
	}
}

// TestMigration0031Down verifies 0031 reverses cleanly WITH a course-scoped
// event present — the load-bearing case (a scopeless Down test would pass for
// the wrong reason). Mirrors TestMigration0025Down's DownTo/Up round-trip.
//
// newTestPool migrates to head, which is 0031 as of this task, so a bare
// goose Down reverses exactly 0031 (no migrations sit on top of it yet).
func TestMigration0031Down(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	courseID := seedCourse(t, ctx, pool)

	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, course_id, surface, type, payload)
		VALUES ($1, $2, 'course', 'step_viewed', '{"ordinal":0}'::jsonb)`,
		refactor2SeededStudentID, courseID); err != nil {
		t.Fatalf("seed course-scoped event (what a real render writes): %v", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}

	if err := goose.DownToContext(ctx, db, "migrations", 30); err != nil {
		t.Fatalf("goose down to v30 (reversing 0031) with a course-scoped event present: %v", err)
	}

	// Column and index are gone.
	var one int
	if err := pool.QueryRow(ctx, `
		SELECT 1 FROM information_schema.columns WHERE table_name='event' AND column_name='course_id'`).Scan(&one); err == nil {
		t.Fatal("event.course_id still present after Down")
	}
	if err := pool.QueryRow(ctx, `
		SELECT 1 FROM pg_indexes WHERE indexname='event_course_created_idx'`).Scan(&one); err == nil {
		t.Fatal("event_course_created_idx still present after Down")
	}

	// The restored 0025-shape constraint is actually back, not just dropped
	// and never replaced: a fully-unscoped new row still violates it.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'chat', 'prompt_sent', '{}'::jsonb)`, refactor2SeededStudentID); err == nil {
		t.Fatal("after Down, an unscoped event should still violate the restored event_scope_ck")
	}

	// And Up restores the course scope. UpToContext(..., 31), not UpContext:
	// course v2 (0050) permanently drops course_session (Down does not
	// recreate it — no back-compat), so a bare Up-to-head would replay
	// 0032's original CREATE VIEW (LEFT JOIN course_session) and fail with
	// "relation course_session does not exist". Stop exactly at 0031, the
	// migration under test — same reasoning as TestMigration0024Down /
	// TestMigration0025Down.
	if err := goose.UpToContext(ctx, db, "migrations", 31); err != nil {
		t.Fatalf("goose up after down: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT 1 FROM information_schema.columns WHERE table_name='event' AND column_name='course_id'`).Scan(&one); err != nil {
		t.Fatalf("event.course_id missing after re-Up: %v", err)
	}
}
