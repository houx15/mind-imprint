package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/store/sqlc"
)

// TestMigration0021ProjectScopedEvaluations exercises Slice 10 Task 2:
// migration 0021 makes evaluations.task_id nullable and adds the
// evaluations_scope_ck CHECK (task_id IS NOT NULL OR project_id IS NOT NULL).
// evaluations.project_id itself already existed since migration 0016.
func TestMigration0021ProjectScopedEvaluations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newTestPool(t)
	q := sqlc.New(pool)
	seededStudentID := refactor2SeededStudentID

	project, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:        seededStudentID,
		Qualification: "EE",
		Title:         "migrate-0021 project-scoped eval",
		BoardCfgVer:   1,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	projectID := pgtype.UUID{Bytes: project.ID, Valid: true}

	// 1. A project-scoped insert works: TaskID comes back NULL, ProjectID set.
	first, err := q.InsertProjectEvaluation(ctx, sqlc.InsertProjectEvaluationParams{
		ProjectID: projectID,
		Scores:    []byte(`{"D1": 3}`),
		Narrative: "first pass narrative",
		Model:     "deepseek-reasoner",
		Tier:      "flagship",
	})
	if err != nil {
		t.Fatalf("InsertProjectEvaluation (first): %v", err)
	}
	if first.TaskID.Valid {
		t.Fatalf("first.TaskID.Valid = true, want false (project-only row)")
	}
	if !first.ProjectID.Valid || first.ProjectID.Bytes != project.ID {
		t.Fatalf("first.ProjectID = %+v, want valid %s", first.ProjectID, project.ID)
	}
	if first.Status != "done" {
		t.Fatalf("first.Status = %q, want done", first.Status)
	}

	// A second, later row for the same project so a query ordered by
	// created_at DESC has something to distinguish.
	time.Sleep(10 * time.Millisecond)
	second, err := q.InsertProjectEvaluation(ctx, sqlc.InsertProjectEvaluationParams{
		ProjectID: projectID,
		Scores:    []byte(`{"D1": 4}`),
		Narrative: "second pass narrative",
		Model:     "deepseek-reasoner",
		Tier:      "flagship",
	})
	if err != nil {
		t.Fatalf("InsertProjectEvaluation (second): %v", err)
	}

	// 2. The most recent row for the project is the second insert. The old
	// GetLatestProjectEvaluation sqlc query was retired 2026-08-14
	// (retire-old-evaluation-pipeline, Task 3) along with the rest of the old
	// dual-axis read path; the CHECK constraint this test actually exercises
	// doesn't depend on that query, so a plain ordered SELECT stands in.
	var latestID uuid.UUID
	var latestNarrative string
	if err := pool.QueryRow(ctx, `
		SELECT id, narrative FROM evaluations
		WHERE project_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, project.ID).Scan(&latestID, &latestNarrative); err != nil {
		t.Fatalf("read latest evaluation row: %v", err)
	}
	if latestID != second.ID {
		t.Fatalf("latest evaluation id = %s, want latest %s (first was %s)", latestID, second.ID, first.ID)
	}
	if latestNarrative != "second pass narrative" {
		t.Fatalf("latest.Narrative = %q, want %q", latestNarrative, "second pass narrative")
	}

	// 3. A legacy task-scoped insert still satisfies the CHECK (back-compat):
	// seeding a task is cheap in this harness (q.CreateTask), so exercise the
	// real path via a raw insert rather than only the negative case.
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{
		UserID: seededStudentID,
		Title:  "migrate-0021 legacy task",
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	var taskRowID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO evaluations (task_id, scores, narrative, model, tier, status)
		VALUES ($1, '{}'::jsonb, 'legacy task-scoped narrative', 'deepseek-reasoner', 'flagship', 'done')
		RETURNING id::text`, task.ID).Scan(&taskRowID); err != nil {
		t.Fatalf("legacy task-scoped insert should satisfy evaluations_scope_ck: %v", err)
	}

	// And the CHECK rejects a row with BOTH ids NULL.
	_, err = pool.Exec(ctx, `
		INSERT INTO evaluations (scores, narrative, model, tier, status)
		VALUES ('{}'::jsonb, 'unscoped narrative', 'deepseek-reasoner', 'flagship', 'done')`)
	if err == nil {
		t.Fatal("insert with task_id AND project_id both NULL should violate evaluations_scope_ck, got no error")
	}
}
