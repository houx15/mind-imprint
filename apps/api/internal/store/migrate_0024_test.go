package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0024SessionScope exercises A1's migration: event and
// evaluations gain a nullable session_id, evaluations_scope_ck widens to
// include it, and event_scope_ck is added NOT VALID — new rows are enforced
// while the pre-existing unattributable chat/course rows (written by
// InsertUserEvent with project_id NULL and no scope column to fill) are
// grandfathered rather than deleted or given a fabricated scope.
func TestMigration0024SessionScope(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	courseID := "00000000-0000-0000-0000-0000000000c1"

	var sessionID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO course_session (user_id, course_id, skill_id, phase)
		VALUES ($1, $2, 'info-literacy-course', 'demonstrate')
		RETURNING id::text`, refactor2SeededStudentID, courseID).Scan(&sessionID); err != nil {
		t.Fatalf("seed course_session: %v", err)
	}

	// 1. A session-scoped event satisfies event_scope_ck with only session_id.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, session_id, surface, type, payload)
		VALUES ($1, $2, 'course', 'course_message', '{"unprompted":true}'::jsonb)`,
		refactor2SeededStudentID, sessionID); err != nil {
		t.Fatalf("session-scoped event must satisfy event_scope_ck: %v", err)
	}

	// 2. A brand-new scopeless COURSE event is REJECTED — the hole A1 closes.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'course', 'course_message', '{}'::jsonb)`,
		refactor2SeededStudentID); err == nil {
		t.Fatal("a new course event with project_id AND session_id both NULL should violate event_scope_ck, got no error")
	}

	// 2b. (Superseded by A2.) 0024 added a temporary `surface = 'chat'`
	// exemption so an unscoped chat event was accepted until chat had a scope to
	// satisfy. Migration 0025 closes that arm now that chat's writes carry
	// thread_id, so at head an unscoped chat event is REJECTED — asserted in
	// TestMigration0025ThreadScope. This harness migrates to head, so the old
	// acceptance assertion no longer holds and there is nothing to assert here.

	// 3. NOT VALID's actual contract: a pre-existing unattributable row still
	// reads fine. Simulate one by inserting with the constraint disabled the
	// only way an old row could exist — via a direct pre-validation path.
	if _, err := pool.Exec(ctx, `ALTER TABLE event DROP CONSTRAINT event_scope_ck`); err != nil {
		t.Fatalf("drop constraint: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'course', 'course_message', '{}'::jsonb)`, refactor2SeededStudentID); err != nil {
		t.Fatalf("seed legacy unattributable row: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		ALTER TABLE event ADD CONSTRAINT event_scope_ck
		CHECK (surface = 'chat' OR num_nonnulls(project_id, session_id) >= 1) NOT VALID`); err != nil {
		t.Fatalf("re-add NOT VALID constraint over a legacy row — this is the whole point of NOT VALID: %v", err)
	}
	var legacy int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM event WHERE surface = 'course' AND project_id IS NULL AND session_id IS NULL`).Scan(&legacy); err != nil {
		t.Fatalf("read legacy unattributable course rows: %v", err)
	}
	if legacy != 1 {
		t.Fatalf("legacy unattributable course rows = %d, want 1 (grandfathered, never deleted)", legacy)
	}

	// 4. evaluations accepts a session-scoped row and still rejects a scopeless one.
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (session_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'course narrative', 'deepseek-reasoner', 'flagship', 'done')`,
		sessionID); err != nil {
		t.Fatalf("session-scoped evaluation must satisfy evaluations_scope_ck: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (scores, narrative, model, tier, status)
		VALUES ('[]'::jsonb, 'unscoped', 'deepseek-reasoner', 'flagship', 'done')`); err == nil {
		t.Fatal("evaluation with task_id, project_id AND session_id all NULL should violate evaluations_scope_ck, got no error")
	}
}

// TestMigration0024Down verifies 0024 reverses cleanly. No test in this repo
// runs a migration Down anywhere (pre-existing, repo-wide gap) — A1 does it
// for its own migration. Safe because newTestPool gives this test its own
// container.
//
// It seeds a session-scoped evaluation (task_id AND project_id NULL,
// session_id set — exactly what InsertSessionEvaluation writes for a real
// 学习报告) BEFORE running Down. Without that seed the Down runs against an
// empty evaluations table and the restored 0021 CHECK validates trivially,
// which is precisely how the original version of this test passed while the
// Down it exercised could not survive a real course report.
//
// A2 note: newTestPool migrates to head, which is now 0025, so a single
// goose Down would reverse 0025, not 0024. DownTo(23) rolls back to before
// 0024 (reversing 0025 then 0024), so 0024's own Down block still runs with
// its load-bearing session-scoped row present.
func TestMigration0024Down(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	courseID := "00000000-0000-0000-0000-0000000000c1"

	var sessionID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO course_session (user_id, course_id, skill_id, phase)
		VALUES ($1, $2, 'info-literacy-course', 'demonstrate')
		RETURNING id::text`, refactor2SeededStudentID, courseID).Scan(&sessionID); err != nil {
		t.Fatalf("seed course_session: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (session_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'course narrative', 'deepseek-reasoner', 'flagship', 'done')`,
		sessionID); err != nil {
		t.Fatalf("seed session-scoped evaluation (what a real 学习报告 writes): %v", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}

	if err := goose.DownToContext(ctx, db, "migrations", 23); err != nil {
		t.Fatalf("goose down to v23 (reversing 0025 then 0024) with a session-scoped evaluation present: %v", err)
	}

	// The columns and constraints are gone.
	for _, q := range []struct{ name, sql string }{
		{"event.session_id", `SELECT 1 FROM information_schema.columns WHERE table_name='event' AND column_name='session_id'`},
		{"evaluations.session_id", `SELECT 1 FROM information_schema.columns WHERE table_name='evaluations' AND column_name='session_id'`},
		{"event_scope_ck", `SELECT 1 FROM pg_constraint WHERE conname='event_scope_ck'`},
	} {
		var one int
		err := pool.QueryRow(ctx, q.sql).Scan(&one)
		if err == nil {
			t.Fatalf("%s still present after Down", q.name)
		}
	}

	// evaluations_scope_ck is restored to its 0021 form: a project-scoped
	// insert still works, a scopeless one is still rejected.
	var projectID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, 'down-test') RETURNING id::text`,
		refactor2SeededStudentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (project_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'n', 'm', 'flagship', 'done')`, projectID); err != nil {
		t.Fatalf("project-scoped evaluation must still work after Down: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (scores, narrative, model, tier, status)
		VALUES ('[]'::jsonb, 'n', 'm', 'flagship', 'done')`); err == nil {
		t.Fatal("restored evaluations_scope_ck should reject a scopeless row, got no error")
	}

	// And Up restores it.
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose up after down: %v", err)
	}
	var one int
	if err := pool.QueryRow(ctx, `
		SELECT 1 FROM information_schema.columns WHERE table_name='event' AND column_name='session_id'`).Scan(&one); err != nil {
		t.Fatalf("event.session_id missing after re-Up: %v", err)
	}
}
