package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestRenderCourseStepGeneratesAndCaches(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	courses, _ := q.ListCourses(context.Background())
	id := courses[0].ID.String()

	// Stub provider returns valid teaching JSON for ordinal 0 (a teaching step).
	stub := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"title":"先别急着信","subtitle":"停一下。","body":["一段。"],"foreground_asset_id":"a0"}`},
		{Kind: gateway.EventDone},
	})
	h := New(Deps{Queries: q, Pool: pool, Provider: stub,
		ChatResolver: func(context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil },
		EvalResolver: func(context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil }}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+id+"/steps/0/render", nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"generated"`)) || !bytes.Contains(rec.Body.Bytes(), []byte("先别急着信")) {
		t.Fatalf("render: %d %s", rec.Code, rec.Body)
	}

	// Cache row now exists.
	step, _ := q.GetCourseStepByOrdinal(context.Background(), sqlc.GetCourseStepByOrdinalParams{CourseID: courses[0].ID, Ordinal: 0})
	if _, err := q.GetCourseStepRender(context.Background(), step.ID); err != nil {
		t.Fatalf("expected cache row after render: %v", err)
	}

	// Unknown ordinal → 404.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+id+"/steps/99/render", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown ordinal: want 404 got %d", rec.Code)
	}
}
