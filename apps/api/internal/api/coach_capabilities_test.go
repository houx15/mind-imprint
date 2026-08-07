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

// TestPostCoach_GeneratePlanGeneratesItemsAndOpensPlan — a coach turn whose
// stubbed model emits generate_plan results in plan rows actually existing
// after (regeneratePlan ran for real, not just a directive flip) and the
// reply's directive opens the 管理 (plan) tool at wide width.
func TestPostCoach_GeneratePlanGeneratesItemsAndOpensPlan(t *testing.T) {
	pool := newAPITestPool(t)
	// Call 1 = the coach orchestrator turn (emits generate_plan). Call 2 = the
	// one-shot completion regeneratePlan's generatePlanItems makes itself.
	prov := sequenceOrchestratorProvider(
		`{"narrate":"四项都齐了，我把计划排出来。","tools":[{"name":"generate_plan","args":{}}]}`,
		planGenReply,
	)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: prov, ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	// Fill the proposal first — an empty kick-off makes generate_plan a no-op
	// (errProposalEmpty), and this test wants the success path.
	rrProp := httptest.NewRecorder()
	h.ServeHTTP(rrProp, withCookie(httptest.NewRequest("PUT", base+"/proposal",
		strings.NewReader(`{"objective":"论证国内新能源投资","reason":"关心气候","activities":"读NASA/Nature","resources":"Zotero"}`)), cookie))
	if rrProp.Code != http.StatusOK {
		t.Fatalf("PUT proposal = %d — %s", rrProp.Code, rrProp.Body)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"四项都填好了，可以生成计划了吗"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		Directive struct {
			OpenTool  string `json:"openTool"`
			WidthTier string `json:"widthTier"`
		} `json:"directive"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.Directive.OpenTool != "plan" || resp.Directive.WidthTier != "wide" {
		t.Fatalf("directive not applied: %+v — %s", resp.Directive, rr.Body)
	}

	// The plan items must actually exist now — regeneratePlan really ran.
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
		t.Fatalf("plan items after generate_plan tool = %d, want 5 (the model reply)", len(listed.Items))
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
