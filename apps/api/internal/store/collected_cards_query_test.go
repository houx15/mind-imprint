package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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

	// A FRESH throwaway subject user (NOT refactor2SeededStudentID, whose cards
	// are contaminated by migration 0018's seed). Copy the NOT-NULL columns from
	// the seeded student's own row rather than inventing values.
	var subject pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, school_id, display_name, avatar_color)
		SELECT 'cards-subject@example.com', password_hash, 'student', school_id, 'Subject', avatar_color
		FROM users WHERE id = $1
		RETURNING id`, refactor2SeededStudentID).Scan(&subject); err != nil {
		t.Fatalf("seed subject user: %v", err)
	}

	// Owned project: TWO completed "concession" (dedupe → uses 2, last_used = the
	// later one) + one SKIPPED "toulmin" (must be excluded).
	var projectID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO project (user_id, title) VALUES ($1, '中国可持续') RETURNING id`,
		subject).Scan(&projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status, created_at)
		VALUES ($1, 'concession', 'completed', now() - interval '5 days')`, projectID); err != nil {
		t.Fatalf("seed concession #1: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status, created_at)
		VALUES ($1, 'concession', 'completed', now() - interval '1 day')`, projectID); err != nil {
		t.Fatalf("seed concession #2: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (project_id, card_id, status)
		VALUES ($1, 'toulmin', 'skipped')`, projectID); err != nil {
		t.Fatalf("seed skipped toulmin: %v", err)
	}

	// Owned course session: ONE completed "opcvl".
	var sessionID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO course_session (user_id, course_id, skill_id, phase)
		VALUES ($1, '00000000-0000-0000-0000-0000000000c1', 'info-literacy-course', 'reflect')
		RETURNING id`, subject).Scan(&sessionID); err != nil {
		t.Fatalf("seed course_session: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (session_id, card_id, status)
		VALUES ($1, 'opcvl', 'completed')`, sessionID); err != nil {
		t.Fatalf("seed opcvl: %v", err)
	}

	// Owned chat thread: ONE completed "craap" — exercises the chat UNION branch.
	var threadID pgtype.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO chat_thread (user_id, title) VALUES ($1, 'CRAAP 溯源') RETURNING id`,
		subject).Scan(&threadID); err != nil {
		t.Fatalf("seed chat_thread: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO card_instances (thread_id, card_id, status)
		VALUES ($1, 'craap', 'completed')`, threadID); err != nil {
		t.Fatalf("seed craap: %v", err)
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
		VALUES ($1, 'concession', 'completed')`, otherProject); err != nil {
		t.Fatalf("seed other completed card: %v", err)
	}

	rows, err := q.ListCollectedCardsByUser(ctx, uuid.UUID(subject.Bytes))
	if err != nil {
		t.Fatalf("ListCollectedCardsByUser: %v", err)
	}
	// Three owned cards: concession (uses 2, project) + opcvl (course) + craap (chat).
	// toulmin skipped → absent; other user's concession → absent.
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (concession+opcvl+craap; skipped & other-user excluded)", len(rows))
	}
	byCard := map[string]sqlc.ListCollectedCardsByUserRow{}
	for _, r := range rows {
		byCard[r.CardID] = r
	}
	con, ok := byCard["concession"]
	if !ok {
		t.Fatalf("concession missing; got %v", byCard)
	}
	if con.Uses != 2 {
		t.Errorf("concession uses = %d, want 2 (deduped count, other user's excluded)", con.Uses)
	}
	if len(con.Surfaces) != 1 || con.Surfaces[0] != "project" {
		t.Errorf("concession surfaces = %v, want [project]", con.Surfaces)
	}
	// last_used is the LATER of the two concession timestamps (now()-1d, not -5d).
	if last, ok := con.LastUsed.(time.Time); !ok {
		t.Errorf("concession last_used is %T, want time.Time", con.LastUsed)
	} else if time.Since(last) > 48*time.Hour {
		t.Errorf("concession last_used = %v, want the later (~1 day ago) timestamp", last)
	}
	if op, ok := byCard["opcvl"]; !ok || op.Uses != 1 || len(op.Surfaces) != 1 || op.Surfaces[0] != "course" {
		t.Errorf("opcvl = %+v, want uses 1 / surfaces [course]", byCard["opcvl"])
	}
	if cr, ok := byCard["craap"]; !ok || cr.Uses != 1 || len(cr.Surfaces) != 1 || cr.Surfaces[0] != "chat" {
		t.Errorf("craap = %+v, want uses 1 / surfaces [chat]", byCard["craap"])
	}
	if _, bad := byCard["toulmin"]; bad {
		t.Error("toulmin (skipped) must not appear")
	}
}
