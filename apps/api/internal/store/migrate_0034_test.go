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
// use. newTestPool migrates to HEAD, which as of course v2 is 0050, not 0034
// — so a bare DownTo(33) reverses 0050 down through 0034, not just 0034
// (0050 sits far above 0034 in the chain but is still reversed by this call).
//
// 0034 also UPDATEs (not just INSERTs) two batches of 0029's pre-existing
// prompt_sent events, spreading them across distinct days. Down does not
// revert those UPDATEs — only 0029's own Down (which deletes the owning users
// and cascades their events) needs to undo them, and the UPDATEs are written
// relative to the ISO week, so re-running Up after Down is idempotent by
// construction rather than requiring a symmetric revert.
//
// Course v2 (0050) fallout: 0034 attaches two of its seed batches
// (course_message + step_viewed) to "the one course 0012 seeds", via
// `course_id`. 0050 permanently deletes that seeded course row (course.id
// FK is ON DELETE CASCADE), so those two batches are cascade-deleted the
// moment newTestPool reaches head — countChenEvents/countStepViewed read 0
// right after the initial migration, not 6/13 as they did before course v2
// existed. Re-running 0034's Up later (after DownTo(33)) hits the same
// fate from the other direction: the course table is empty at that point
// (0050's Down does not — and structurally cannot — restore a row it
// DELETEd; that's data loss, not a DDL revert), so 0034's `(SELECT id FROM
// course ORDER BY created_at LIMIT 1)` subquery returns no rows and its two
// INSERT ... SELECT ... FROM (subquery) CROSS JOIN generate_series(...)
// statements insert zero rows — silently, not an error. The course-scoped
// halves of 0034's seed are dead weight post-course-v2; this test now only
// asserts the parts of 0034 that still do something (evaluations, the extra
// project-scoped prompt_sent events).
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

	// After the initial Up (via newTestPool, which now migrates all the way
	// to 0050): 2 evaluations (project-scoped, untouched by course v2), 0
	// course_message and 0 step_viewed (both were course_id-scoped against
	// 0012's seeded course, cascade-deleted the moment 0050 runs — see the
	// course-v2-fallout comment above; they were 6 and 13 respectively before
	// course v2 existed), 4 extra prompt_sent (2 for 911, 2 for 918,
	// project-scoped — untouched by course v2).
	if n := countEvals(); n != 2 {
		t.Fatalf("evaluations after Up = %d, want 2", n)
	}
	if n := countChenEvents(); n != 0 {
		t.Fatalf("陈屿 course_message events after Up = %d, want 0 (course-scoped, cascade-deleted by course v2's 0050)", n)
	}
	if n := countStepViewed(); n != 0 {
		t.Fatalf("step_viewed events after Up = %d, want 0 (course-scoped, cascade-deleted by course v2's 0050)", n)
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

	// And Up again must succeed. DownToContext(33) reversed 0050 too (34..50
	// are all > 33), so this replays 0034's Up against an EMPTY course table
	// — 0050's Down deleted the seeded course row as data loss, not a DDL
	// revert, and nothing recreates it. 0034's course_message/step_viewed
	// INSERT ... SELECT ... FROM (SELECT id FROM course ...) subquery finds
	// no course row, so those two batches silently insert 0 rows this time
	// too (see the course-v2-fallout comment on this test). evaluations and
	// the extra prompt_sent events are project-scoped and unaffected, so
	// those two still restore to their original counts.
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatalf("goose up after down: %v", err)
	}
	if n := countEvals(); n != 2 {
		t.Fatalf("evaluations after re-Up = %d, want 2", n)
	}
	if n := countChenEvents(); n != 0 {
		t.Fatalf("陈屿 course_message events after re-Up = %d, want 0 (no course row for 0034's Up to attach to)", n)
	}
	if n := countStepViewed(); n != 0 {
		t.Fatalf("step_viewed events after re-Up = %d, want 0 (no course row for 0034's Up to attach to)", n)
	}
	if n := countExtraPromptSent(); n != 4 {
		t.Fatalf("extra prompt_sent events after re-Up = %d, want 4", n)
	}
}
