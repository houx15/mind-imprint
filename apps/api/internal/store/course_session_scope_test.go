package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// TestCourseSessionScope exercises the Slice 12 session-scoped queries
// introduced by migration 0023: a course_session (the runtime unit, holding
// the phase) can own materials and card_instances directly (task_id/
// project_id/thread_id NULL, session_id set), which the widened scope CHECK
// constraints allow. course_step/course_progress are untouched content; the
// seeded course …c1 (migration 0012) is the FK target.
func TestCourseSessionScope(t *testing.T) {
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

	// One session per (user, course): re-creating resumes, never duplicates.
	// The second call passes a DIFFERENT phase ("guided") than the first
	// ("demonstrate") — a regression guard for the ON CONFLICT clause: it
	// must NOT include `phase = EXCLUDED.phase` (whole-branch Minor), which
	// would silently restart a student's course back to the skill's first
	// phase on every re-entry. The first phase must survive untouched.
	again, err := q.CreateCourseSession(ctx, sqlc.CreateCourseSessionParams{
		UserID: seededStudentID, CourseID: courseID, SkillID: "info-literacy-course", Phase: "guided",
	})
	if err != nil {
		t.Fatalf("re-create session: %v", err)
	}
	if again.ID != s.ID {
		t.Fatalf("second create minted a new session %s, want the existing %s", again.ID, s.ID)
	}
	if again.Phase != "demonstrate" {
		t.Fatalf("phase after conflict = %q, want the FIRST phase (demonstrate) to survive — "+
			"ON CONFLICT must never overwrite phase", again.Phase)
	}

	// GetCourseSession / GetCourseSessionByUserCourse both resolve the same row.
	got, err := q.GetCourseSession(ctx, s.ID)
	if err != nil || got.ID != s.ID {
		t.Fatalf("GetCourseSession = %+v (err %v), want %s", got, err, s.ID)
	}
	byUC, err := q.GetCourseSessionByUserCourse(ctx, sqlc.GetCourseSessionByUserCourseParams{
		UserID: seededStudentID, CourseID: courseID,
	})
	if err != nil || byUC.ID != s.ID {
		t.Fatalf("GetCourseSessionByUserCourse = %+v (err %v), want %s", byUC, err, s.ID)
	}

	// A session-scoped material satisfies the widened scope CHECK with only
	// session_id set — no task, no project, no thread. kind/source stay within
	// the existing ('article','draft') / ('fetched','pasted') CHECKs — the
	// course anchor material is not a new kind/source value.
	sid := pgtype.UUID{Bytes: s.ID, Valid: true}
	m, err := q.CreateSessionMaterial(ctx, sqlc.CreateSessionMaterialParams{
		SessionID: sid,
		Kind:      "article", Source: "pasted", Title: "待核实的说法", Blocks: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("session material must satisfy the scope CHECK: %v", err)
	}

	mats, err := q.ListMaterialsBySession(ctx, sid)
	if err != nil || len(mats) != 1 || mats[0].ID != m.ID {
		t.Fatalf("ListMaterialsBySession = %v (err %v), want the one material", mats, err)
	}

	// A session-scoped card_instance: no material_id/tool_id column on
	// card_instances (the offer's material link lives in Go's CardOffer, not
	// the row) — mirrors CreateThreadCardInstance's column list exactly.
	ci, err := q.CreateSessionCardInstance(ctx, sqlc.CreateSessionCardInstanceParams{
		SessionID: sid, CardID: "craap", Status: "proposed",
	})
	if err != nil {
		t.Fatalf("session card instance: %v", err)
	}

	cards, err := q.ListCardInstancesBySession(ctx, sid)
	if err != nil || len(cards) != 1 || cards[0].ID != ci.ID {
		t.Fatalf("ListCardInstancesBySession = %v (err %v), want the one card", cards, err)
	}

	// SubmitSessionCardInstance sets completed_at deliberately (unlike the
	// thread equivalent, which does not — logged as a Slice-11 minor).
	submitted, err := q.SubmitSessionCardInstance(ctx, sqlc.SubmitSessionCardInstanceParams{
		ID: ci.ID, SessionID: sid, FieldValues: []byte(`{"a":1}`), EventTrace: []byte("[]"), Status: "completed",
	})
	if err != nil || submitted.Status != "completed" || !submitted.CompletedAt.Valid {
		t.Fatalf("SubmitSessionCardInstance = %+v (err %v), want completed with completed_at set", submitted, err)
	}

	// SetSessionCardInstanceStatus is also session-scoped.
	skipped, err := q.SetSessionCardInstanceStatus(ctx, sqlc.SetSessionCardInstanceStatusParams{
		ID: ci.ID, SessionID: sid, Status: "skipped",
	})
	if err != nil || skipped.Status != "skipped" {
		t.Fatalf("SetSessionCardInstanceStatus = %+v (err %v), want skipped", skipped, err)
	}

	// Phase moves.
	moved, err := q.SetCourseSessionPhase(ctx, sqlc.SetCourseSessionPhaseParams{ID: s.ID, Phase: "guided"})
	if err != nil || moved.Phase != "guided" {
		t.Fatalf("SetCourseSessionPhase = %+v (err %v), want phase guided", moved, err)
	}

	// Messages + the student-turn count the floor reads.
	for _, role := range []string{"student", "assistant", "student"} {
		if _, err := q.CreateCourseMessage(ctx, sqlc.CreateCourseMessageParams{
			SessionID: s.ID, Phase: "guided", Role: role, Content: "x",
		}); err != nil {
			t.Fatalf("create message: %v", err)
		}
	}
	msgs, err := q.ListMessagesBySession(ctx, s.ID)
	if err != nil || len(msgs) != 3 {
		t.Fatalf("ListMessagesBySession = %v (err %v), want 3 messages", msgs, err)
	}
	n, err := q.CountStudentTurnsInPhase(ctx, sqlc.CountStudentTurnsInPhaseParams{SessionID: s.ID, Phase: "guided"})
	if err != nil || n != 2 {
		t.Fatalf("CountStudentTurnsInPhase = %d (err %v), want 2", n, err)
	}
	if n, err := q.CountStudentTurnsInPhase(ctx, sqlc.CountStudentTurnsInPhaseParams{SessionID: s.ID, Phase: "reflect"}); err != nil || n != 0 {
		t.Fatalf("turns in an untouched phase = %d (err %v), want 0", n, err)
	}
}
