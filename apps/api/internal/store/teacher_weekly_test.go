package store_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/store/sqlc"
)

// TestClassWeeklyProseFirstWriteWins guards DEC-2: class_weekly_prose's
// primary key (class_id, week_start) IS the first-open-wins lock.
// InsertClassWeeklyProse is ON CONFLICT DO NOTHING, so a concurrent second
// generation call is silently discarded and the first writer's prose stands.
func TestClassWeeklyProseFirstWriteWins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	classID := createClassRow(t, pool)
	week := pgtype.Date{Time: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), Valid: true}

	first := sqlc.InsertClassWeeklyProseParams{
		ClassID: classID, WeekStart: week,
		Comment: "第一次写的点评", DepthNote: "d1", AutonomyNote: "a1",
		Cards: []byte(`[{"userId":"u1","lead":"l1","action":"act1"}]`),
	}
	if err := q.InsertClassWeeklyProse(ctx, first); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	second := first
	second.Comment = "第二次写的点评"
	if err := q.InsertClassWeeklyProse(ctx, second); err != nil {
		t.Fatalf("second insert must not error: %v", err)
	}

	got, err := q.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{ClassID: classID, WeekStart: week})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Comment != "第一次写的点评" {
		t.Fatalf("comment = %q; want the FIRST write — first-open-wins", got.Comment)
	}
}

// TestAppendClassWeeklyProseCardsOnlyAppends guards DEC-6: the top-up writer
// may only append to cards. It must never rewrite comment/depth_note/autonomy_note
// on an existing row.
func TestAppendClassWeeklyProseCardsOnlyAppends(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	classID := createClassRow(t, pool)
	week := pgtype.Date{Time: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), Valid: true}

	if err := q.InsertClassWeeklyProse(ctx, sqlc.InsertClassWeeklyProseParams{
		ClassID: classID, WeekStart: week,
		Comment: "点评", DepthNote: "d", AutonomyNote: "a",
		Cards: []byte(`[{"userId":"u1","lead":"l1","action":"act1"}]`),
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := q.AppendClassWeeklyProseCards(ctx, sqlc.AppendClassWeeklyProseCardsParams{
		ClassID: classID, WeekStart: week,
		Cards: []byte(`[{"userId":"u2","lead":"l2","action":"act2"}]`),
	}); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, err := q.GetClassWeeklyProse(ctx, sqlc.GetClassWeeklyProseParams{ClassID: classID, WeekStart: week})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Comment != "点评" {
		t.Fatalf("comment = %q; the top-up must never rewrite it", got.Comment)
	}
	s := string(got.Cards)
	if !strings.Contains(s, `"u1"`) || !strings.Contains(s, `"u2"`) {
		t.Fatalf("cards = %s; want both the original and the appended entry", s)
	}
}

// createClassRow seeds a minimal school + class row. No equivalent helper
// exists in the store_test (external) package — teacher_query_test.go's
// class-seeding inline SQL lives in the internal `store` package test file
// and is not reusable from here.
func createClassRow(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var schoolID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO schools (name) VALUES ('D2 Weekly Prose Test School') RETURNING id`).
		Scan(&schoolID); err != nil {
		t.Fatalf("seed school: %v", err)
	}
	var classID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO classes (school_id, name, join_code) VALUES ($1, 'D2 Weekly Prose Class', $2) RETURNING id`,
		schoolID, uuid.NewString()).Scan(&classID); err != nil {
		t.Fatalf("seed class: %v", err)
	}
	return classID
}
