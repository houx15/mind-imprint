package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// TestMigration0034Down verifies 0034's Down removes exactly what its Up
// inserted (children-first, mirroring 0029's Down), and that Up/Down/Up all
// succeed — the round-trip idiom TestMigration0031Down/TestMigration0025Down
// use. newTestPool migrates to head, which is 0034 as of this task, so a bare
// DownTo(33) reverses exactly 0034.
//
// 0034 also UPDATEs (not just INSERTs) two batches of 0029's pre-existing
// prompt_sent events, spreading them across distinct days. Down does not
// revert those UPDATEs — only 0029's own Down (which deletes the owning users
// and cascades their events) needs to undo them, and the UPDATEs are written
// relative to the ISO week, so re-running Up after Down is idempotent by
// construction rather than requiring a symmetric revert.
func TestMigration0034Down(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t) // migrates to head (0034)

	countEvals := func() int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM evaluations
			WHERE id IN ('00000000-0000-0000-0000-000000000981','00000000-0000-0000-0000-000000000982')`).
			Scan(&n); err != nil {
			t.Fatalf("count 0034 evaluations: %v", err)
		}
		return n
	}
	countChenEvents := func() int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM event
			WHERE user_id = '00000000-0000-0000-0000-000000000914' AND type = 'course_message'`).
			Scan(&n); err != nil {
			t.Fatalf("count 陈屿 course_message events: %v", err)
		}
		return n
	}
	countStepViewed := func() int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM event
			WHERE type = 'step_viewed'
			  AND user_id IN ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000915','00000000-0000-0000-0000-000000000918')`).
			Scan(&n); err != nil {
			t.Fatalf("count step_viewed events: %v", err)
		}
		return n
	}
	countExtraPromptSent := func() int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM event
			WHERE type = 'prompt_sent'
			  AND user_id IN ('00000000-0000-0000-0000-000000000911','00000000-0000-0000-0000-000000000918')
			  AND created_at > date_trunc('week', now()) + interval '2 days'`).
			Scan(&n); err != nil {
			t.Fatalf("count extra prompt_sent events: %v", err)
		}
		return n
	}

	// After the initial Up (via newTestPool): 2 evaluations, 6 course_message
	// (5 last week + 1 this week for 陈屿), 13 step_viewed (3 ordinals × 3
	// students = 9 this week for 911/915/918, 2 ordinals × 2 students = 4 last
	// week for 911/915 — each batch is a CROSS JOIN of ordinals × students, not
	// an additive count), 4 extra prompt_sent (2 for 911, 2 for 918).
	if n := countEvals(); n != 2 {
		t.Fatalf("evaluations after Up = %d, want 2", n)
	}
	if n := countChenEvents(); n != 6 {
		t.Fatalf("陈屿 course_message events after Up = %d, want 6", n)
	}
	if n := countStepViewed(); n != 13 {
		t.Fatalf("step_viewed events after Up = %d, want 13", n)
	}
	if n := countExtraPromptSent(); n != 4 {
		t.Fatalf("extra prompt_sent events after Up = %d, want 4", n)
	}

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialect: %v", err)
	}

	if err := goose.DownToContext(ctx, db, "migrations", 33); err != nil {
		t.Fatalf("goose down to v33 (reversing 0034): %v", err)
	}
	if n := countEvals(); n != 0 {
		t.Fatalf("evaluations after Down = %d, want 0", n)
	}
	if n := countChenEvents(); n != 0 {
		t.Fatalf("陈屿 course_message events after Down = %d, want 0", n)
	}
	if n := countStepViewed(); n != 0 {
		t.Fatalf("step_viewed events after Down = %d, want 0", n)
	}
	if n := countExtraPromptSent(); n != 0 {
		t.Fatalf("extra prompt_sent events after Down = %d, want 0", n)
	}

	// And Up again must succeed and restore the same counts.
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose up after down: %v", err)
	}
	if n := countEvals(); n != 2 {
		t.Fatalf("evaluations after re-Up = %d, want 2", n)
	}
	if n := countChenEvents(); n != 6 {
		t.Fatalf("陈屿 course_message events after re-Up = %d, want 6", n)
	}
	if n := countStepViewed(); n != 13 {
		t.Fatalf("step_viewed events after re-Up = %d, want 13", n)
	}
	if n := countExtraPromptSent(); n != 4 {
		t.Fatalf("extra prompt_sent events after re-Up = %d, want 4", n)
	}
}
