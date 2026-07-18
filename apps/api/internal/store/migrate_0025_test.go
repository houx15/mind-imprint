package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0025ThreadScope: event and evaluations gain a nullable
// thread_id, evaluations_scope_ck widens to include it, and event_scope_ck is
// re-added WITHOUT A1's surface='chat' arm — so a new unscoped chat event is
// now rejected (the hole A2 closes), while pre-A2 unattributable rows stay
// grandfathered (still NOT VALID).
func TestMigration0025ThreadScope(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	var threadID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO chat_thread (user_id, title) VALUES ($1, '') RETURNING id::text`,
		refactor2SeededStudentID).Scan(&threadID); err != nil {
		t.Fatalf("seed chat_thread: %v", err)
	}

	// 1. A thread-scoped event satisfies event_scope_ck with only thread_id.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, thread_id, surface, type, payload)
		VALUES ($1, $2, 'chat', 'prompt_sent', '{}'::jsonb)`,
		refactor2SeededStudentID, threadID); err != nil {
		t.Fatalf("thread-scoped chat event must satisfy event_scope_ck: %v", err)
	}

	// 2. A brand-new UNSCOPED chat event is now REJECTED — the arm A1 parked is
	// gone. This is the whole point of A2's constraint change.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'chat', 'prompt_sent', '{}'::jsonb)`,
		refactor2SeededStudentID); err == nil {
		t.Fatal("a new unscoped chat event should now violate event_scope_ck (exemption closed), got no error")
	}

	// 3. NOT VALID's contract still holds: a pre-existing unattributable row
	// reads fine. Simulate one via the only path it could exist — constraint
	// dropped, row inserted, constraint re-added NOT VALID over it.
	if _, err := pool.Exec(ctx, `ALTER TABLE event DROP CONSTRAINT event_scope_ck`); err != nil {
		t.Fatalf("drop constraint: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'chat', 'prompt_sent', '{}'::jsonb)`, refactor2SeededStudentID); err != nil {
		t.Fatalf("seed legacy unattributable chat row: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		ALTER TABLE event ADD CONSTRAINT event_scope_ck
		CHECK (num_nonnulls(project_id, session_id, thread_id) >= 1) NOT VALID`); err != nil {
		t.Fatalf("re-add NOT VALID over a legacy row — the point of NOT VALID: %v", err)
	}
	var legacy int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM event
		WHERE surface = 'chat' AND project_id IS NULL AND session_id IS NULL AND thread_id IS NULL`).Scan(&legacy); err != nil {
		t.Fatalf("read legacy unattributable chat rows: %v", err)
	}
	if legacy != 1 {
		t.Fatalf("legacy unattributable chat rows = %d, want 1 (grandfathered, never deleted)", legacy)
	}

	// 4. evaluations accepts a thread-scoped row and still rejects a scopeless one.
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (thread_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'chat narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		threadID); err != nil {
		t.Fatalf("thread-scoped evaluation must satisfy evaluations_scope_ck: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (scores, narrative, model, tier, status)
		VALUES ('[]'::jsonb, 'unscoped', 'deepseek-v4-pro', 'flagship', 'done')`); err == nil {
		t.Fatal("evaluation with every scope NULL should violate evaluations_scope_ck, got no error")
	}
}

// TestMigration0025Down verifies 0025 reverses cleanly WITH a thread-scoped
// report present — the load-bearing case (a scopeless Down test would pass for
// the wrong reason, exactly the gap A1's whole-branch review caught in 0024).
// It also asserts the surface='chat' exemption is RESTORED: after Down an
// unscoped chat event must be accepted again.
//
// A3 note: newTestPool migrates to head, which is now 0026, so a bare goose
// Down would reverse 0026, not 0025. DownTo(24) rolls back to before 0025
// (reversing 0026 then 0025), mirroring how TestMigration0024Down was fixed
// when 0025 landed on top of 0024.
func TestMigration0025Down(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)

	var threadID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO chat_thread (user_id, title) VALUES ($1, '') RETURNING id::text`,
		refactor2SeededStudentID).Scan(&threadID); err != nil {
		t.Fatalf("seed chat_thread: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO evaluations (thread_id, scores, narrative, model, tier, status)
		VALUES ($1, '[]'::jsonb, 'chat narrative', 'deepseek-v4-pro', 'flagship', 'done')`,
		threadID); err != nil {
		t.Fatalf("seed thread-scoped evaluation (what a real chat report writes): %v", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}

	if err := goose.DownToContext(ctx, db, "migrations", 24); err != nil {
		t.Fatalf("goose down to v24 (reversing 0026 then 0025) with a thread-scoped evaluation present: %v", err)
	}

	// Columns and the thread indexes are gone.
	for _, q := range []struct{ name, sql string }{
		{"event.thread_id", `SELECT 1 FROM information_schema.columns WHERE table_name='event' AND column_name='thread_id'`},
		{"evaluations.thread_id", `SELECT 1 FROM information_schema.columns WHERE table_name='evaluations' AND column_name='thread_id'`},
	} {
		var one int
		if err := pool.QueryRow(ctx, q.sql).Scan(&one); err == nil {
			t.Fatalf("%s still present after Down", q.name)
		}
	}

	// The surface='chat' exemption is restored: an unscoped chat event is
	// accepted again (0024's state), otherwise chat would silently stop
	// recording between Down and a re-Up.
	if _, err := pool.Exec(ctx, `
		INSERT INTO event (user_id, surface, type, payload)
		VALUES ($1, 'chat', 'prompt_sent', '{}'::jsonb)`, refactor2SeededStudentID); err != nil {
		t.Fatalf("after Down the surface='chat' exemption must be restored: %v", err)
	}

	// And Up restores the thread scope.
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose up after down: %v", err)
	}
	var one int
	if err := pool.QueryRow(ctx, `
		SELECT 1 FROM information_schema.columns WHERE table_name='event' AND column_name='thread_id'`).Scan(&one); err != nil {
		t.Fatalf("event.thread_id missing after re-Up: %v", err)
	}
}
