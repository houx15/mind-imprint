package api_test

// assessment_test.go — GET /api/v1/projects/{id}/assessment: the growth-report
// read path (no model call, ever). Generation itself moved to the project's
// one-time terminal, POST /api/v1/projects/{id}/finish (project_finish_test.go,
// A3 Task 4) — this endpoint no longer has a POST sibling.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
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

// dualAxisReply is a valid DualAxis report reply (agent.AssessReport's wire
// shape) — the shared success fixture for both the project surface
// (project_finish_test.go) and, later, course/chat once they flip too.
const dualAxisReply = `{"depthAxis":{"dims":[
  {"code":"D1","score":3,"evidence":"限定判断","promptEvidence":"R4"},
  {"code":"D3","score":2,"evidence":"NASA","promptEvidence":""},
  {"code":"D4","score":3,"evidence":"warrant","promptEvidence":""},
  {"code":"D5","score":3,"evidence":"理由","promptEvidence":""}]},
  "autonomyAxis":{"observation":"设边界","anchoredSignals":["R1"],"promptedSignals":["R3"],"adversaryInvites":0,"promptEvidence":""},
  "crossAxis":{"depthLevel":"L3","initiative":"引导后","prose":"能反思","promptEvidence":""},
  "solo":[{"round":4,"excerpt":"限定","level":"L3","rationale":"组织者","initiative":"自发"}],
  "promptLens":{"directiveRounds":3,"totalRounds":10,"boundarySettings":3,"adversaryInvites":0,
    "questions":[{"title":"一问","body":"…"}],"bestPrompt":{"round":8,"quote":"检查回扣","annotation":"齐备"},
    "takeaway":{"round":0,"quote":"苛刻审稿人","annotation":"P4"},"perRound":[{"round":1,"tier":"P3","label":"要过程·设边界"}]},
  "timeline":[{"round":1,"task":"上传","prompt":"不要重写","pTag":"P3","dimTags":["D1=2"]}],
  "keyEvidence":[{"label":"任务理解","quote":"改 thesis"}],
  "guidance":{"anchored":"限定 thesis","prompted":"SIFT","risk":"D3","nextSteps":[{"title":"强化 D3","body":"SIFT 记录"}]},
  "narrative":"深度 L3 稳定复现。"}`

// TestGetAssessment_EmptyBeforeGenerate — before any report exists, GET must
// return 200 with an explicit JSON null (a normal "not yet assessed" state,
// never a 404) and must record NO llm_call — the read path never calls a
// model.
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

// TestGetAssessment_ReturnsPersistedReport — GET replays a report persisted
// directly (standing in for finishProject's InsertProjectEvaluation write,
// covered end-to-end by project_finish_test.go) with no model call.
func TestGetAssessment_ReturnsPersistedReport(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	if _, err := sqlc.New(pool).InsertProjectEvaluation(t.Context(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgUUID(mustUUID(projectID)),
		Scores:    []byte(dualAxisReply),
		Narrative: "深度 L3 稳定复现。",
		Model:     "deepseek-v4-pro",
		Tier:      "flagship",
	}); err != nil {
		t.Fatalf("InsertProjectEvaluation: %v", err)
	}

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/assessment", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET assessment (persisted) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var dto struct {
		DepthAxis struct {
			Subtotal int `json:"subtotal"`
		} `json:"depthAxis"`
		Narrative   string `json:"narrative"`
		GeneratedAt string `json:"generatedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode assessment DTO: %v — body=%s", err, rec.Body)
	}
	if dto.Narrative != "深度 L3 稳定复现。" {
		t.Fatalf("narrative = %q, want the persisted narrative", dto.Narrative)
	}
	if dto.GeneratedAt == "" {
		t.Error("generatedAt empty, want RFC3339 timestamp")
	}
	if n := countLLMCalls(t, pool, projectID); n != 0 {
		t.Fatalf("llm_call rows after GET of persisted report = %d, want 0 (no model call on read path)", n)
	}
}

// TestGetAssessment_RejectsOtherUsersProject — ownership hidden as
// not-found, same as every other project-scoped route (loadOwnedProject).
func TestGetAssessment_RejectsOtherUsersProject(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	other := createStudent(t, pool, SeedSchoolID, "assessment-other@demo.local")
	cookie := signInAs(t, pool, other)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/assessment", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET assessment (other user) = %d, want 404 (ownership hidden as not-found); body=%s", rec.Code, rec.Body)
	}
	if n := countLLMCalls(t, pool, projectID); n != 0 {
		t.Fatalf("llm_call rows after other-user attempt = %d, want 0", n)
	}
}
