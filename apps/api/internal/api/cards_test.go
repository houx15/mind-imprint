package api_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestCardLifecycleRoutes(t *testing.T) {
	q := newAPITestQueries(t)
	defer func() {}()
	h := New(Deps{Queries: q}).Handler()
	ctx := context.Background()

	task, _ := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: SeedUserID, Title: "T"})
	card, _ := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "sift_craap", TaskID: task.ID})
	base := "/api/v1/tasks/" + task.ID.String() + "/cards/" + card.ID.String()

	// PATCH active
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("PATCH", base, strings.NewReader(`{"status":"active"}`)))
	if rr.Code != 200 {
		t.Fatalf("patch: %d %s", rr.Code, rr.Body.String())
	}

	// PUT completed (valid envelope)
	rr = httptest.NewRecorder()
	body := `{"status":"completed","field_values":{"sift":{"stop":"x"}},"event_trace":[{"kind":"submit"}]}`
	h.ServeHTTP(rr, httptest.NewRequest("PUT", base, strings.NewReader(body)))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"status":"completed"`) {
		t.Fatalf("put: %d %s", rr.Code, rr.Body.String())
	}

	// PUT bad event_trace kind → 400
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("PUT", base, strings.NewReader(`{"status":"completed","field_values":{},"event_trace":[{"kind":"bogus"}]}`)))
	if rr.Code != 400 {
		t.Fatalf("bad trace: want 400, got %d", rr.Code)
	}

	// skip on a second card
	c2, _ := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "concession", TaskID: task.ID})
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/tasks/"+task.ID.String()+"/cards/"+c2.ID.String()+"/skip", strings.NewReader(`{"event_trace":[{"kind":"skip"}]}`)))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"status":"skipped"`) {
		t.Fatalf("skip: %d %s", rr.Code, rr.Body.String())
	}

	// cross-task ownership → 404
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("PATCH", "/api/v1/tasks/"+uuid.NewString()+"/cards/"+card.ID.String(), strings.NewReader(`{"status":"active"}`)))
	if rr.Code != 404 {
		t.Fatalf("cross-task: want 404, got %d", rr.Code)
	}
}
