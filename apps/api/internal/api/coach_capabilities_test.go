package api_test

// coach_capabilities_test.go — P2b Task 4: two orchestrator tools the coach
// APPLIES server-side (Tasks 1-3 already parse/validate them):
//   - generate_plan: 印记 triggers plan generation itself (button retired) via
//     the shared regeneratePlan helper (workspace_plan_generate.go), and opens
//     the 管理 (plan) tool on success.
//   - propose_question: surfaces a question proposal on the reply for the
//     student to confirm — it must NOT write an exploration_lead row itself
//     (that only happens once the student confirms via its own endpoint).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// sequenceOrchestratorProvider replays a DIFFERENT orchestrator/completion
// JSON blob per model call, in order — the coach turn is call 1, and any
// server-side follow-up completion the tool itself triggers (e.g.
// regeneratePlan's own one-shot completion) is call 2, since both share
// a.d.Provider.
func sequenceOrchestratorProvider(jsonOuts ...string) gateway.Provider {
	scripts := make([][]gateway.StreamEvent, len(jsonOuts))
	for i, out := range jsonOuts {
		scripts[i] = []gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: out},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 12}},
			{Kind: gateway.EventDone, StopReason: gateway.StopStop},
		}
	}
	return gateway.NewSequenceStubProvider(scripts...)
}

// TestFunnelAutoGeneratesPlanAndCoachOffersProposal — the status-router
// replacement for the retired generate_plan tool: filling all four proposal
// dims (PUT /proposal) auto-generates the plan DETERMINISTICALLY (the funnel,
// not a model tool), and a subsequent framework coach turn offers the one-tap
// nextStep to 写提案 instead of re-offering plan generation.
func TestFunnelAutoGeneratesPlanAndCoachOffersProposal(t *testing.T) {
	pool := newAPITestPool(t)
	// Call 1 = the one-shot completion regeneratePlan makes during PUT /proposal
	// (auto-gen). Call 2 = the coach status turn (neutral, no recording claim so
	// no note-recover call).
	prov := sequenceOrchestratorProvider(
		planGenReply,
		`{"narrate":"计划有了，我们下一步写提案。","tools":[]}`,
	)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: prov, ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	setStudioStage(t, pool, seedProjectID, agent.StageProposalForming) // started framework
	base := "/api/v1/projects/" + seedProjectID

	// Fill all four dims — the funnel auto-generates the plan on this write.
	rrProp := httptest.NewRecorder()
	h.ServeHTTP(rrProp, withCookie(httptest.NewRequest("PUT", base+"/proposal",
		strings.NewReader(`{"objective":"论证国内新能源投资","reason":"关心气候","activities":"读NASA/Nature","resources":"Zotero"}`)), cookie))
	if rrProp.Code != http.StatusOK {
		t.Fatalf("PUT proposal = %d — %s", rrProp.Code, rrProp.Body)
	}

	// Plan items exist now — the funnel ran regeneratePlan for real.
	rrList := httptest.NewRecorder()
	h.ServeHTTP(rrList, withCookie(httptest.NewRequest("GET", base+"/plan", nil), cookie))
	if rrList.Code != http.StatusOK {
		t.Fatalf("GET plan = %d — %s", rrList.Code, rrList.Body)
	}
	var listed struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode plan list: %v — %s", err, rrList.Body)
	}
	if len(listed.Items) != 5 {
		t.Fatalf("plan items after auto-gen = %d, want 5 (the model reply)", len(listed.Items))
	}

	// A framework coach turn now offers the one-tap nextStep to 写提案.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"接下来做什么"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		NextStep *struct {
			ToStatus string `json:"toStatus"`
			Surface  string `json:"surface"`
		} `json:"nextStep"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.NextStep == nil || resp.NextStep.ToStatus != "proposal" || resp.NextStep.Surface != "writing" {
		t.Fatalf("expected nextStep → proposal/writing, got %+v — %s", resp.NextStep, rr.Body)
	}
}

// TestFrameworkReviewerSurfacesVerdictOnceAfterPlanGen — slice 2: when the
// framework's 4 dims fill and the plan auto-generates, the flagship reasoning
// reviewer (EvalResolver) reads the framework; its verdict is stashed and
// surfaced on the NEXT coach turn (reply.reviewVerdict), then cleared so a
// later turn carries none. Plan-gen is NOT gated by the verdict.
func TestFrameworkReviewerSurfacesVerdictOnceAfterPlanGen(t *testing.T) {
	pool := newAPITestPool(t)
	// Shared provider call order during PUT /proposal: (1) regeneratePlan's
	// completion, (2) the framework reviewer (EvalResolver). Then each coach turn
	// is one more call (clamped to the last script once exhausted).
	prov := sequenceOrchestratorProvider(
		planGenReply,
		`{"ready":true,"why":"四点都扎实。","suggestions":["资源那条可以更具体","补一个反例"]}`,
		`{"narrate":"计划有了，我们继续。","tools":[]}`,
	)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: prov, ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	setStudioStage(t, pool, seedProjectID, agent.StageProposalForming)
	base := "/api/v1/projects/" + seedProjectID

	// Fill the 4 dims → plan auto-gens AND the reviewer runs (verdict stashed).
	rrProp := httptest.NewRecorder()
	h.ServeHTTP(rrProp, withCookie(httptest.NewRequest("PUT", base+"/proposal",
		strings.NewReader(`{"objective":"论证国内新能源投资","reason":"关心气候","activities":"读NASA/Nature","resources":"Zotero"}`)), cookie))
	if rrProp.Code != http.StatusOK {
		t.Fatalf("PUT proposal = %d — %s", rrProp.Code, rrProp.Body)
	}

	// First coach turn surfaces the stashed verdict.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"接下来做什么"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		ReviewVerdict *struct {
			Ready       bool     `json:"ready"`
			Why         string   `json:"why"`
			Suggestions []string `json:"suggestions"`
		} `json:"reviewVerdict"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.ReviewVerdict == nil || !resp.ReviewVerdict.Ready || len(resp.ReviewVerdict.Suggestions) != 2 || resp.ReviewVerdict.Why == "" {
		t.Fatalf("want framework verdict with 2 suggestions, got %+v — %s", resp.ReviewVerdict, rr.Body)
	}

	// Second coach turn: verdict already surfaced + cleared → none.
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"好的"}`)), cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("coach2 = %d — %s", rr2.Code, rr2.Body)
	}
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode2: %v — %s", err, rr2.Body)
	}
	if resp.ReviewVerdict != nil {
		t.Fatalf("verdict must surface once, got %+v on the 2nd turn", resp.ReviewVerdict)
	}
}

// TestPostCoach_ProposeQuestionPopulatesReplyNoLead — a coach turn whose
// stubbed model emits propose_question populates reply.question.text for the
// student to confirm, and writes NO exploration_lead row itself (the student
// hasn't confirmed anything yet — that's a separate endpoint).
func TestPostCoach_ProposeQuestionPopulatesReplyNoLead(t *testing.T) {
	pool := newAPITestPool(t)
	const question = "中国的碳排放总量是否抵消了新能源投资的净效应？"
	out := `{"narrate":"这个问题值得追一下。","tools":[` +
		`{"name":"propose_question","args":{"text":"` + question + `"}}]}`
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: orchestratorStubProvider(out), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我在读这篇关于碳排放的论文"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		Question *struct {
			Text string `json:"text"`
		} `json:"question"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.Question == nil || resp.Question.Text != question {
		t.Fatalf("question proposal wrong: %+v — %s", resp.Question, rr.Body)
	}

	leads, err := sqlc.New(pool).ListExplorationLeads(context.Background(), mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("ListExplorationLeads: %v", err)
	}
	if len(leads) != 0 {
		t.Fatalf("propose_question must write NO exploration lead itself, got %d — %+v", len(leads), leads)
	}
}
