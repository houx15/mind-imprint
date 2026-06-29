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

func TestPutCard_TriggersMilestoneEval(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "T"})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	card1, _ := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "sift_craap", TaskID: task.ID})
	card2, _ := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "concession", TaskID: task.ID})

	rec := &recordingEnqueuer{}
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, Enqueuer: rec}).Handler()
	cookie := signInSeed(t, pool)

	put := func(cid string) int {
		body := strings.NewReader(`{"field_values":{},"event_trace":[],"status":"completed"}`)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("PUT", "/api/v1/tasks/"+task.ID.String()+"/cards/"+cid, body), cookie))
		return rr.Code
	}

	// First completed card → milestone 1 → exactly one auto-enqueue.
	if code := put(card1.ID.String()); code != 200 {
		t.Fatalf("put card1 status=%d", code)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("after card1: enqueues=%d, want 1", len(rec.calls))
	}
	if rec.calls[0].TaskID != task.ID {
		t.Fatalf("enqueued wrong task: %s", rec.calls[0].TaskID)
	}

	// Second completed card while the first eval is still queued → debounced (no new enqueue).
	if code := put(card2.ID.String()); code != 200 {
		t.Fatalf("put card2 status=%d", code)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("after card2: enqueues=%d, want 1 (in-flight debounce)", len(rec.calls))
	}
}
