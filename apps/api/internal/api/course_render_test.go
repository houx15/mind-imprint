package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

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

// TestRenderCourseStepRecordsUsage — 5d review CRITICAL fix, the third live
// gateway.Collect call site (agent/course.go's renderTeaching): a generated
// course step is a real model call and must be metered the same way the
// studio surface's coach/anchors calls are (purpose="course_render",
// surface="course", project_id NULL — a course render has no owning
// project).
func TestRenderCourseStepRecordsUsage(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	courses, _ := q.ListCourses(context.Background())
	id := courses[0].ID.String()

	stub := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"title":"先别急着信","subtitle":"停一下。","body":["一段。"],"foreground_asset_id":"a0"}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 88, OutputTokens: 33}},
		{Kind: gateway.EventDone},
	})
	h := New(Deps{Queries: q, Pool: pool, Provider: stub,
		ChatResolver: func(context.Context) (gateway.Resolved, error) { return gateway.Resolved{Provider: "stub"}, nil },
		EvalResolver: func(context.Context) (gateway.Resolved, error) {
			return gateway.Resolved{Provider: "stub", Model: "stub-model", Tier: "flagship"}, nil
		}}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+id+"/steps/0/render", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("render: %d %s", rec.Code, rec.Body)
	}

	rows, err := q.GetSchoolUsageByTier(context.Background(), SeedSchoolID)
	if err != nil {
		t.Fatalf("GetSchoolUsageByTier: %v", err)
	}
	var flagship *sqlc.GetSchoolUsageByTierRow
	for i, r := range rows {
		if r.Tier == "flagship" {
			flagship = &rows[i]
			break
		}
	}
	if flagship == nil {
		t.Fatalf("no flagship-tier usage row after a course render: %+v", rows)
	}
	if flagship.PromptTokens == 0 && flagship.CompletionTokens == 0 {
		t.Fatalf("expected non-zero token counts, got %+v", flagship)
	}
}

// TestRenderCourseStepEmitsStepViewedEvent — D2 Task 2: a course page-turn
// must land a timestamped, course-scoped event, not just the mutable
// completed_ordinals array (course_progress has no timestamp, so "steps
// completed this week" was unqueryable). The event write sits right after
// RecordCourseStepViewed in course_render.go and must satisfy
// event_scope_ck via course_id.
func TestRenderCourseStepEmitsStepViewedEvent(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	courses, _ := q.ListCourses(context.Background())
	courseID := courses[0].ID
	id := courseID.String()

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
	if rec.Code != http.StatusOK {
		t.Fatalf("render: %d %s", rec.Code, rec.Body)
	}

	var eventCourseID pgtype.UUID
	var ordinal string
	err := pool.QueryRow(context.Background(),
		`SELECT course_id, payload->>'ordinal' FROM event
		  WHERE user_id = $1 AND type = 'step_viewed' ORDER BY created_at DESC LIMIT 1`,
		SeedUserID).Scan(&eventCourseID, &ordinal)
	if err != nil {
		t.Fatalf("no step_viewed event recorded: %v", err)
	}
	if !eventCourseID.Valid {
		t.Fatal("step_viewed event has no course_id — it would violate event_scope_ck")
	}
	if eventCourseID.Bytes != [16]byte(courseID) {
		t.Fatalf("step_viewed event course_id = %x, want %x", eventCourseID.Bytes, [16]byte(courseID))
	}
	if ordinal != "0" {
		t.Fatalf("payload ordinal = %q; want \"0\"", ordinal)
	}
}
