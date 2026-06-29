package store_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"mindimprint/api/internal/store/sqlc"
)

// seedTaskForEval inserts a task owned by the seeded demo student and returns its ID.
// The seeded student (seededStudentID) is present after RunMigrations completes.
func seedTaskForEval(t *testing.T, ctx context.Context, pool *pgxpool.Pool) sqlc.Task {
	t.Helper()
	q := sqlc.New(pool)
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: seededStudentID, Title: "lifecycle-test"})
	if err != nil {
		t.Fatalf("seedTaskForEval: %v", err)
	}
	return task
}

func TestEvaluationAsyncLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	ctx := context.Background()
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)

	task := seedTaskForEval(t, ctx, pool)

	ev, err := q.EnqueueEvaluation(ctx, task.ID)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if ev.Status != "queued" {
		t.Fatalf("status=%q want queued", ev.Status)
	}

	if err := q.MarkEvaluationRunning(ctx, ev.ID); err != nil {
		t.Fatalf("running: %v", err)
	}

	pt, ct := int32(10), int32(20)
	err = q.FinishEvaluation(ctx, sqlc.FinishEvaluationParams{
		ID:               ev.ID,
		Scores:           []byte(`[{"dim_id":"D2","level":"L3","note":"x"}]`),
		Narrative:        "ok",
		Signals:          []byte(`{"message_count":2}`),
		RubricVersion:    ptr("cognitive-model-v2"),
		Model:            "deepseek-reasoner",
		Tier:             "flagship",
		PromptTokens:     &pt,
		CompletionTokens: &ct,
	})
	if err != nil {
		t.Fatalf("finish: %v", err)
	}

	got, err := q.GetLatestEvaluation(ctx, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "done" || got.Narrative != "ok" {
		t.Fatalf("final row wrong: status=%q narrative=%q", got.Status, got.Narrative)
	}
}
