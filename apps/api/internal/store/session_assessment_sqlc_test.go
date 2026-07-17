package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// TestSessionAssessmentQueries exercises A1's three new queries: the course
// session's event stream becomes readable (it was unreachable — the only
// SELECT against event filtered WHERE project_id = $1, which can never match
// a NULL), and evaluations round-trip at session scope.
func TestSessionAssessmentQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	courseID := uuid.MustParse("00000000-0000-0000-0000-0000000000c1")

	s, err := q.CreateCourseSession(ctx, sqlc.CreateCourseSessionParams{
		UserID: seededStudentID, CourseID: courseID, SkillID: "info-literacy-course", Phase: "demonstrate",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sid := pgtype.UUID{Bytes: s.ID, Valid: true}

	// Two session events, in order.
	for _, ev := range []struct{ typ, payload string }{
		{"course_message", `{"unprompted":true}`},
		{"phase_advanced", `{"to":"guided"}`},
	} {
		if _, err := q.AppendEvent(ctx, sqlc.AppendEventParams{
			UserID: seededStudentID, SessionID: sid, Surface: "course",
			Type: ev.typ, Payload: []byte(ev.payload),
		}); err != nil {
			t.Fatalf("append %s: %v", ev.typ, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	// A second seeded course is not present in migration 0012 (only …c1 is
	// seeded) — insert it directly so the cross-session leak guard below has
	// a real FK target for `other`.
	if _, err := pool.Exec(ctx, `
		INSERT INTO course (id, branch, title) VALUES ($1, 'test', 'leak-guard course')
		ON CONFLICT (id) DO NOTHING`, "00000000-0000-0000-0000-0000000000c2"); err != nil {
		t.Fatalf("seed second course: %v", err)
	}

	// A DIFFERENT session's event must not leak into this session's stream.
	other, err := q.CreateCourseSession(ctx, sqlc.CreateCourseSessionParams{
		UserID: seededStudentID, CourseID: uuid.MustParse("00000000-0000-0000-0000-0000000000c2"),
		SkillID: "info-literacy-course", Phase: "demonstrate",
	})
	if err != nil {
		t.Fatalf("create other session: %v", err)
	}
	if _, err := q.AppendEvent(ctx, sqlc.AppendEventParams{
		UserID: seededStudentID, SessionID: pgtype.UUID{Bytes: other.ID, Valid: true},
		Surface: "course", Type: "course_finished", Payload: []byte(`{}`),
	}); err != nil {
		t.Fatalf("append other: %v", err)
	}

	got, err := q.ListEventsBySession(ctx, sid)
	if err != nil {
		t.Fatalf("ListEventsBySession: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListEventsBySession returned %d events, want exactly this session's 2", len(got))
	}
	if got[0].Type != "course_message" || got[1].Type != "phase_advanced" {
		t.Fatalf("events = %q,%q — want temporal order course_message,phase_advanced", got[0].Type, got[1].Type)
	}

	// Evaluations round-trip at session scope; latest wins.
	first, err := q.InsertSessionEvaluation(ctx, sqlc.InsertSessionEvaluationParams{
		SessionID: sid, Scores: []byte(`[{"code":"D1"}]`), Narrative: "first",
		Model: "deepseek-reasoner", Tier: "flagship",
	})
	if err != nil {
		t.Fatalf("InsertSessionEvaluation: %v", err)
	}
	if first.ProjectID.Valid || first.TaskID.Valid {
		t.Fatalf("session evaluation has project/task scope set: %+v", first)
	}
	if first.Status != "done" {
		t.Fatalf("first.Status = %q, want done", first.Status)
	}
	time.Sleep(10 * time.Millisecond)
	second, err := q.InsertSessionEvaluation(ctx, sqlc.InsertSessionEvaluationParams{
		SessionID: sid, Scores: []byte(`[{"code":"D1"}]`), Narrative: "second",
		Model: "deepseek-reasoner", Tier: "flagship",
	})
	if err != nil {
		t.Fatalf("InsertSessionEvaluation (second): %v", err)
	}
	latest, err := q.GetLatestSessionEvaluation(ctx, sid)
	if err != nil {
		t.Fatalf("GetLatestSessionEvaluation: %v", err)
	}
	if latest.ID != second.ID {
		t.Fatalf("latest = %s, want the newest %s (first was %s)", latest.ID, second.ID, first.ID)
	}
}
