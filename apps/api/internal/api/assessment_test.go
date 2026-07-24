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

// dualAxisReply is a valid canonical Report reply (agent.AssessReport's
// reportWire shape) — the shared success fixture for the project, course, and
// chat surfaces. It carries all 6 depth dims + 6 autonomy signals + the
// officialProjection/workAndProcess superset; AssessReport only keeps that
// superset when in.ProjectProjection is true (project surface), and nils it
// out for chat/course regardless of what the model emits.
const dualAxisReply = `{"depthAxis":[
  {"code":"D1","level":"L3","evidence":"限定判断了论点范围","promptEvidence":"R2"},
  {"code":"D2","level":"L4","evidence":"交叉验证了两个一手源","promptEvidence":"R4"},
  {"code":"D3","level":"L3","evidence":"NASA 数据与结论对齐","promptEvidence":"R5"},
  {"code":"D4","level":"L3","evidence":"warrant 说明清楚","promptEvidence":""},
  {"code":"D5","level":"L3","evidence":"理由链完整","promptEvidence":""},
  {"code":"D6","level":"L3","evidence":"复盘时能说出下一步","promptEvidence":""}],
  "autonomyAxis":[
  {"code":"A1","level":2,"opportunity":"given_taken","evidence":"自己定了检索范围","promptEvidence":"R1"},
  {"code":"A2","level":1,"opportunity":"given_taken","evidence":"","promptEvidence":""},
  {"code":"A3","level":3,"opportunity":"given_taken","evidence":"设了字数与来源边界","promptEvidence":"R3"},
  {"code":"A4","level":0,"opportunity":"not_supplied","evidence":"","promptEvidence":""},
  {"code":"A5","level":2,"opportunity":"given_not_taken","evidence":"","promptEvidence":""},
  {"code":"A6","level":1,"opportunity":"given_taken","evidence":"","promptEvidence":""}],
  "promptLens":{
    "stats":[{"label":"提问轮次","value":"10"},{"label":"边界设定","value":"3"},{"label":"对抗性邀请","value":"0"}],
    "lenses":[
      {"code":"L_decisions","level":3,"evidence":"决策清楚"},
      {"code":"L_maturity","level":3,"evidence":"成熟度稳定"},
      {"code":"L_boundary","level":3,"evidence":"边界设定明确"},
      {"code":"L_adversary","level":1,"evidence":"较少邀请对抗视角"},
      {"code":"L_directive","level":2,"evidence":"指令型提问适中"},
      {"code":"L_acceptance","level":3,"evidence":"能筛选采纳"}]},
  "interactionEvidence":[
    {"round":4,"student":"我想把 thesis 限定到国内新能源投资","aiSummary":"帮你梳理了限定范围的两种做法","signal":"边界设定"}],
  "narrative":"这次协作里，你在信源辨识上完成了一次明显的跃迁。",
  "guidance":{"nextSteps":[{"title":"强化反例检验","task":"针对碳排放数据补一版反例段"}]},
  "officialProjection":{
    "standard":{"id":"ap-research","name":"AP Research"},
    "components":[
      {"name":"Academic Paper","judgement":"接近达标","reason":"论证结构完整，证据链清楚"},
      {"name":"训练用折算","judgement":"仅供参考","reason":"内部折算不等于官方评分"}],
    "alignment":[
      {"item":"论证结构","standard":"清晰的主张-证据-推理链","performance":"三段论证均有 warrant","impact":"支撑 Academic Paper 的组织维度"}],
    "readiness":{"score":72,"note":"仅作作品就绪度参考，不与 D/A 双轴合成"}},
  "workAndProcess":{
    "workSamples":[{"title":"最新稿件","text":"中国的可再生能源投资规模已经连续五年位居全球第一。"}],
    "processMaterials":[{"name":"SIFT 溯源记录","status":"completed","diagnosis":"两个信源均可交叉验证"}]}}`

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

	// The DTO's narrative comes from row.Scores (json.Unmarshal into
	// agent.Report), NOT the separate Narrative column — so the assertion below
	// must match dualAxisReply's own embedded narrative field, not this column.
	if _, err := sqlc.New(pool).InsertProjectEvaluation(t.Context(), sqlc.InsertProjectEvaluationParams{
		ProjectID: pgUUID(mustUUID(projectID)),
		Scores:    []byte(dualAxisReply),
		Narrative: "这次协作里，你在信源辨识上完成了一次明显的跃迁。",
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
		DepthAxis []struct {
			Code  string `json:"code"`
			Level string `json:"level"`
		} `json:"depthAxis"`
		Narrative   string `json:"narrative"`
		GeneratedAt string `json:"generatedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode assessment DTO: %v — body=%s", err, rec.Body)
	}
	if dto.Narrative != "这次协作里，你在信源辨识上完成了一次明显的跃迁。" {
		t.Fatalf("narrative = %q, want the persisted (Scores-embedded) narrative", dto.Narrative)
	}
	if dto.GeneratedAt == "" {
		t.Error("generatedAt empty, want RFC3339 timestamp")
	}
	if len(dto.DepthAxis) != 6 {
		t.Fatalf("depthAxis len = %d, want 6 (D1-D6 always present)", len(dto.DepthAxis))
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
