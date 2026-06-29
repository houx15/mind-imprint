package api_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// recordingEnqueuer captures the args it was asked to enqueue so the test can
// assert the handler queued exactly one evaluation job.
type recordingEnqueuer struct{ calls []agent.EvaluateArgs }

func (r *recordingEnqueuer) EnqueueEvaluate(_ context.Context, a agent.EvaluateArgs) error {
	r.calls = append(r.calls, a)
	return nil
}

func TestPostEvaluate_EnqueuesQueued(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "T"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	// Seed a tiny transcript so the task is realistic (the worker re-reads state).
	_, _ = q.AppendMessage(ctx, sqlc.AppendMessageParams{TaskID: task.ID, Role: "user", Content: "hi"})

	rec := &recordingEnqueuer{}
	h := New(Deps{
		Queries:  sqlc.New(pool),
		Pool:     pool,
		Enqueuer: rec,
	}).Handler()
	cookie := signInSeed(t, pool)

	// POST evaluate → 202 + queued status; no inline LLM call.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/evaluate", nil), cookie))
	if rr.Code != 202 {
		t.Fatalf("status=%d want 202; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"status":"queued"`) {
		t.Errorf("body missing queued status: %s", rr.Body.String())
	}
	if len(rec.calls) != 1 {
		t.Fatalf("expected 1 enqueue, got %d", len(rec.calls))
	}
	if rec.calls[0].TaskID != task.ID {
		t.Errorf("enqueued task id = %s, want %s", rec.calls[0].TaskID, task.ID)
	}

	// GET evaluation → 200 + still queued (no worker ran in this unit test).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/tasks/"+task.ID.String()+"/evaluation", nil), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"status":"queued"`) {
		t.Fatalf("get eval: %d %s", rr.Code, rr.Body.String())
	}

	// GET on a task with no evaluation → 404
	other, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "U"})
	if err != nil {
		t.Fatalf("CreateTask other: %v", err)
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/tasks/"+other.ID.String()+"/evaluation", nil), cookie))
	if rr.Code != 404 {
		t.Fatalf("want 404 for no eval, got %d — body: %s", rr.Code, rr.Body.String())
	}
}
