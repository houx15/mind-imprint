package api_test

// lite_teacher_workspace_surface_test.go — which surfaces the workspace turn
// serves. home and parentReport get their own tools and canvas in D2/D3; until
// each lands, a turn naming it must be refused rather than answered with the
// assignment tool set. Delete a row here in the task that opens that surface.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
)

func TestWorkspaceTurnRefusesSurfacesNotOpenYet(t *testing.T) {
	h, pool, teacher, classID, _ := liteTeacherFixtureWithProvider(t, writingTextStubProvider("好的。"))

	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing", map[string]any{"classId": classID, "text": "布置作业"}},
		{"unknown", map[string]any{"surface": "gradebook", "classId": classID, "text": "布置作业"}},
		{"home", map[string]any{"surface": "home", "classId": classID, "text": "这个班怎么样"}},
		{"parentReport", map[string]any{
			"surface": "parentReport", "classId": classID,
			"reportId": "00000000-0000-0000-0000-000000000001", "text": "改一下第一段",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			rec := postWorkspaceTurn(t, h, teacher, string(body))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), "unknown_surface") {
				t.Fatalf("body = %s, want code unknown_surface", rec.Body)
			}
		})
	}

	// A refused surface is refused before any model call, so nothing is billed.
	var calls int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM llm_call`).Scan(&calls); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if calls != 0 {
		t.Fatalf("llm_call rows = %d, want 0", calls)
	}
}

// TestWorkspaceTurnWritesASurfaceBuildFailureBeforeTheModel — a surface that
// cannot be built answers with its own error, after the ownership check and
// before any model call, so nothing is billed.
func TestWorkspaceTurnWritesASurfaceBuildFailureBeforeTheModel(t *testing.T) {
	h, pool, teacher, classID, _ := liteTeacherFixtureWithProvider(t, writingTextStubProvider("好的。"))
	body, _ := json.Marshal(map[string]any{
		"surface": LiteWorkspaceBrokenSurfaceForTest, "classId": classID, "text": "布置作业",
	})

	// Ownership still comes first: a teacher of another class gets 404, not
	// the build error.
	other := signInAs(t, pool, createTeacher(t, pool, SeedSchoolID, "ws-broken-other@demo.local"))
	if rec := postWorkspaceTurn(t, h, other, string(body)); rec.Code != http.StatusNotFound {
		t.Fatalf("other teacher = %d, want 404; body=%s", rec.Code, rec.Body)
	}

	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "surface_unavailable") {
		t.Fatalf("status = %d body = %s, want 409 surface_unavailable", rec.Code, rec.Body)
	}

	var calls int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM llm_call`).Scan(&calls); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if calls != 0 {
		t.Fatalf("llm_call rows = %d, want 0", calls)
	}
}

// TestWorkspaceTurnIgnoresReportIDOnTheAssignmentSurface — reportId is part of
// the request now, and only the parentReport surface will read it. An
// assignment turn that carries one is still an assignment turn.
func TestWorkspaceTurnIgnoresReportIDOnTheAssignmentSurface(t *testing.T) {
	h, _, teacher, classID, _ := liteTeacherFixtureWithProvider(t, writingTextStubProvider("好的。"))
	body, _ := json.Marshal(map[string]any{
		"surface": "assignment", "classId": classID, "reportId": "not-a-uuid", "text": "布置阅读作业",
	})
	rec := postWorkspaceTurn(t, h, teacher, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if got := decodeWorkspaceTurn(t, rec); got.Reply != "好的。" {
		t.Fatalf("reply = %q, want 好的。", got.Reply)
	}
}
