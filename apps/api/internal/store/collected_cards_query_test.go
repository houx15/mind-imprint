package store

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// ListCollectedCardsByUser returns one row per card_id the caller has COMPLETED,
// across all three scopes, with uses (count), surfaces (distinct), and last_used
// (max created_at). Skipped instances and other users' rows are excluded.
func TestListCollectedCardsByUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)

	// Owned project with TWO completed instances of card "concession" (dedupe →
	// uses 2, last_used = the later one) at controlled created_at.
	var projectID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '中国可持续') RETURNING id`,
		refactor2SeededStudentID).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status, created_at)
		VALUES ($1, 'concession', 'completed', now() - interval '3 days')`, projectID); err != nil {
		t.Fatalf("seed concession #1: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status, created_at)
		VALUES ($1, 'concession', 'completed', now() - interval '1 day')`, projectID); err != nil {
		t.Fatalf("seed concession #2: %v", err)
	}
	// A SKIPPED instance of a different card — must be excluded entirely.
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status)
		VALUES ($1, 'toulmin', 'skipped')`, projectID); err != nil {
		t.Fatalf("seed skipped toulmin: %v", err)
	}

	// Owned course session with ONE completed instance of card "opcvl".
	var sessionID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO course_session (user_id, course_id, skill_id, phase)
		VALUES ($1, '00000000-0000-0000-0000-0000000000c1', 'info-literacy-course', 'reflect')
		RETURNING id`, refactor2SeededStudentID).Scan(&sessionID); err != nil {
		t.Fatalf("seed course_session: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (session_id, card_id, status)
		VALUES ($1, 'opcvl', 'completed')`, sessionID); err != nil {
		t.Fatalf("seed opcvl: %v", err)
	}

	// A DIFFERENT user's completed card — must be excluded (owner isolation).
	var otherUser pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		SELECT 'cards-other@example.com', password_hash, 'student', school_id, 'Other', avatar_color
		FROM users WHERE id = $1
		RETURNING id`, refactor2SeededStudentID).Scan(&otherUser); err != nil {
		t.Fatalf("seed other user: %v", err)
	}
	var otherProject pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, 'not mine') RETURNING id`,
		otherUser).Scan(&otherProject); err != nil {
		t.Fatalf("seed other project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status)
		VALUES ($1, 'craap', 'completed')`, otherProject); err != nil {
		t.Fatalf("seed other completed card: %v", err)
	}

	rows, err := q.ListCollectedCardsByUser(ctx, refactor2SeededStudentID)
	if err != nil {
		t.Fatalf("ListCollectedCardsByUser: %v", err)
	}
	// Three owned cards: concession (uses 2, project) + opcvl (uses 1, course) +
	// steelman (uses 1, project — pre-existing completed instance from migration
	// 0018_seed_demo_project.sql, which seeds a demo project for this exact
	// seeded student). toulmin skipped → absent; craap belongs to another user
	// → absent.
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (concession + opcvl + seeded steelman; skipped & other-user excluded)", len(rows))
	}
	byCard := map[string]sqlc.ListCollectedCardsByUserRow{}
	for _, r := range rows {
		byCard[r.CardID] = r
	}
	con, ok := byCard["concession"]
	if !ok {
		t.Fatalf("concession missing; got %v", byCard)
	}
	// 3, not 2: migration 0018_seed_demo_project.sql already seeds one
	// completed "concession" instance for this exact user, in addition to
	// the two this test seeds above.
	if con.Uses != 3 {
		t.Errorf("concession uses = %d, want 3 (2 seeded here + 1 from migration 0018)", con.Uses)
	}
	if len(con.Surfaces) != 1 || con.Surfaces[0] != "project" {
		t.Errorf("concession surfaces = %v, want [project]", con.Surfaces)
	}
	op, ok := byCard["opcvl"]
	if !ok {
		t.Fatalf("opcvl missing; got %v", byCard)
	}
	if op.Uses != 1 || len(op.Surfaces) != 1 || op.Surfaces[0] != "course" {
		t.Errorf("opcvl = uses %d surfaces %v, want 1 / [course]", op.Uses, op.Surfaces)
	}
	steel, ok := byCard["steelman"]
	if !ok {
		t.Fatalf("steelman (pre-seeded by migration 0018) missing; got %v", byCard)
	}
	if steel.Uses != 1 || len(steel.Surfaces) != 1 || steel.Surfaces[0] != "project" {
		t.Errorf("steelman = uses %d surfaces %v, want 1 / [project]", steel.Uses, steel.Surfaces)
	}
	if _, bad := byCard["toulmin"]; bad {
		t.Error("toulmin (skipped) must not appear")
	}
	if _, bad := byCard["craap"]; bad {
		t.Error("craap (other user) must not appear")
	}
}
