package api_test

// assessment_test.go — Task 6: GET/POST /api/v1/projects/{id}/assessment.
// GET is the growth-report read path (no model call, ever); POST runs the
// isolated flagship growth-assessor once over the project's process record,
// persists the report, and returns it. Never in the coach loop.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// assessStubProvider is a gateway.Provider whose Stream emits a scripted
// assessment JSON reply then closes — same scripted-provider shape as
// writing_test.go's reviewStubProvider, just carrying the assessor's own
// {"dimensions":[...],"narrative":"..."} wire shape.
func assessStubProvider(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 80, OutputTokens: 40}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

const assessReply = `{"dimensions":[{"code":"D2","level":"L4","evidence":"交叉验证两个一手源"},{"code":"D5","level":"L3","evidence":"论证拆解清楚"}],"narrative":"你这次最大的跃迁在信源辨识。"}`

// TestGetAssessment_EmptyBeforeGenerate — before any POST, GET must return
// 200 with an explicit JSON null (a normal "not yet assessed" state, never a
// 404) and must record NO llm_call — the read path never calls a model.
func TestGetAssessment_EmptyBeforeGenerate(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/assessment", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET assessment (empty) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("GET assessment (empty) body = %s, want literal null", rec.Body)
	}
	if n := countLLMCalls(t, pool, projectID); n != 0 {
		t.Fatalf("llm_call rows after empty GET = %d, want 0 (no model call on read path)", n)
	}
}

// TestGenerateAssessment_PersistsAndReturnsDTO — POST makes one flagship
// call, persists the report, records its cost, and returns an AssessmentDTO
// covering all 10 rubric dimensions. A subsequent GET replays the SAME
// persisted report with NO second model call.
func TestGenerateAssessment_PersistsAndReturnsDTO(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     assessStubProvider(assessReply),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/assessment", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST assessment = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var dto struct {
		Dimensions []struct {
			Code     string `json:"code"`
			Name     string `json:"name"`
			Level    string `json:"level"`
			Evidence string `json:"evidence"`
		} `json:"dimensions"`
		Narrative   string `json:"narrative"`
		GeneratedAt string `json:"generatedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode assessment DTO: %v — body=%s", err, rec.Body)
	}
	if len(dto.Dimensions) != 10 {
		t.Fatalf("dimensions len = %d, want 10 (every rubric dimension, defaulting to NA)", len(dto.Dimensions))
	}
	codes := map[string]bool{}
	for _, d := range dto.Dimensions {
		codes[d.Code] = true
	}
	for i := 1; i <= 10; i++ {
		code := "D" + string(rune('0'+i))
		if i == 10 {
			code = "D10"
		}
		if !codes[code] {
			t.Errorf("dimensions missing code %s: %+v", code, dto.Dimensions)
		}
	}
	if dto.Narrative == "" {
		t.Error("narrative empty, want the model's growth narrative")
	}
	if dto.GeneratedAt == "" {
		t.Error("generatedAt empty, want RFC3339 timestamp")
	}

	// Persisted: GetLatestProjectEvaluation returns the same report.
	row, err := sqlc.New(pool).GetLatestProjectEvaluation(req.Context(), pgUUID(mustUUID(projectID)))
	if err != nil {
		t.Fatalf("GetLatestProjectEvaluation: %v", err)
	}
	var scores []agent.DimensionScore
	if err := json.Unmarshal(row.Scores, &scores); err != nil {
		t.Fatalf("decode persisted scores: %v", err)
	}
	if len(scores) != 10 {
		t.Fatalf("persisted scores len = %d, want 10", len(scores))
	}
	if row.Narrative != dto.Narrative {
		t.Fatalf("persisted narrative = %q, want %q", row.Narrative, dto.Narrative)
	}

	// One llm_call recorded for this generation.
	if n := countLLMCalls(t, pool, projectID); n != 1 {
		t.Fatalf("llm_call rows after generate = %d, want 1", n)
	}

	// GET after POST returns the persisted assessment, no new model call.
	rec2 := httptest.NewRecorder()
	req2 := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/assessment", nil), cookie)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET assessment (after generate) = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}
	var dto2 struct {
		Dimensions []struct {
			Code string `json:"code"`
		} `json:"dimensions"`
		Narrative string `json:"narrative"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &dto2); err != nil {
		t.Fatalf("decode GET-after-POST DTO: %v — body=%s", err, rec2.Body)
	}
	if len(dto2.Dimensions) != 10 || dto2.Narrative != dto.Narrative {
		t.Fatalf("GET-after-POST assessment mismatch: %+v", dto2)
	}
	if n := countLLMCalls(t, pool, projectID); n != 1 {
		t.Fatalf("llm_call rows after GET replay = %d, want 1 (no new model call)", n)
	}
}

// TestGenerateAssessment_RejectsOtherUsersProject — ownership hidden as
// not-found, same as every other project-scoped route (loadOwnedProject).
func TestGenerateAssessment_RejectsOtherUsersProject(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     assessStubProvider(assessReply),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	other := createStudent(t, pool, SeedSchoolID, "assessment-other@demo.local")
	cookie := signInAs(t, pool, other)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/assessment", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST assessment (other user) = %d, want 404 (ownership hidden as not-found); body=%s", rec.Code, rec.Body)
	}

	rec2 := httptest.NewRecorder()
	req2 := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/assessment", nil), cookie)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("GET assessment (other user) = %d, want 404 (ownership hidden as not-found); body=%s", rec2.Code, rec2.Body)
	}

	if n := countLLMCalls(t, pool, projectID); n != 0 {
		t.Fatalf("llm_call rows after other-user attempt = %d, want 0", n)
	}
}
