package agent_test

// coursestore_test.go — real-DB regression guard for sqlcCourseStore's event
// scoping (A1 Task 3). Follows agentstore_chat_sqlc_test.go's harness:
// newTurnTestPool + seededStudentID (testpool_test.go), package agent_test.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// TestInsertSessionEventScopesTheRow is the regression guard for A1's whole
// reason to exist: the old InsertUserEvent took no scope and hard-coded
// project_id NULL, so every course event was written unattributable and
// unreadable. InsertSessionEvent must set session_id and resolve user_id from
// the session — never take either on faith from a caller.
func TestInsertSessionEventScopesTheRow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTurnTestPool(t)
	q := sqlc.New(pool)
	store := agent.NewSqlcCourseStore(q)

	s, err := q.CreateCourseSession(ctx, sqlc.CreateCourseSessionParams{
		UserID: seededStudentID, CourseID: uuid.MustParse("00000000-0000-0000-0000-0000000000c1"),
		SkillID: "info-literacy-course", Phase: "demonstrate",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := store.InsertSessionEvent(ctx, s.ID, "course_message", []byte(`{"unprompted":true}`)); err != nil {
		t.Fatalf("InsertSessionEvent: %v", err)
	}

	rows, err := q.ListEventsBySession(ctx, pgtype.UUID{Bytes: s.ID, Valid: true})
	if err != nil || len(rows) != 1 {
		t.Fatalf("ListEventsBySession = %v (err %v), want 1 row", rows, err)
	}
	got := rows[0]
	if got.Surface != "course" {
		t.Fatalf("surface = %q, want course (the store writes its own surface)", got.Surface)
	}
	if got.UserID != seededStudentID {
		t.Fatalf("user_id = %s, want it resolved from the session (%s)", got.UserID, seededStudentID)
	}
	if !got.SessionID.Valid || got.SessionID.Bytes != s.ID {
		t.Fatalf("session_id = %+v, want %s — an unscoped course event is the A1 bug", got.SessionID, s.ID)
	}
	if got.ProjectID.Valid {
		t.Fatalf("project_id = %+v, want NULL (a course session has no project)", got.ProjectID)
	}
}
