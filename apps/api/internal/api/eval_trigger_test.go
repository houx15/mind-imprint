package api_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestMilestoneIndex(t *testing.T) {
	cases := []struct {
		cards, turns int64
		want         int32
	}{
		{0, 0, 0}, {0, 5, 0}, {0, 6, 1}, {0, 11, 1}, {0, 12, 2},
		{1, 0, 1}, {1, 6, 2}, {2, 7, 3},
	}
	for _, c := range cases {
		if got := MilestoneIndex(c.cards, c.turns); got != c.want {
			t.Errorf("MilestoneIndex(%d,%d) = %d, want %d", c.cards, c.turns, got, c.want)
		}
	}
}

func TestPostEvaluate_InflightIsIdempotent(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "T"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	// Pre-seed an in-flight (queued) eval directly.
	existing, err := q.EnqueueEvaluation(ctx, task.ID)
	if err != nil {
		t.Fatalf("seed enqueue: %v", err)
	}

	rec := &recordingEnqueuer{}
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Enqueuer: rec}).Handler()
	cookie := signInSeed(t, pool)

	// Manual POST /evaluate while one is in flight → 202 with the existing row, no new job.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/evaluate", nil), cookie))
	if rr.Code != 202 {
		t.Fatalf("status=%d want 202; body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), existing.ID.String()) {
		t.Fatalf("body should return the in-flight eval %s: %s", existing.ID, rr.Body.String())
	}
	if len(rec.calls) != 0 {
		t.Fatalf("no new job should be enqueued, got %d", len(rec.calls))
	}
}
