package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func TestQuestionCardTurn_Endpoint(t *testing.T) {
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"narrate":"用你自己的话说说你对题目的理解？","suggestedObjective":"","done":false}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 5}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	h, cookie, _ := proposalTrackHandler(t, prov)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+seedProjectID+"/cards/question-card/turn",
		strings.NewReader(`{"messages":[{"role":"student","text":"帮我想想这题"}]}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("turn = %d — %s", rr.Code, rr.Body)
	}
	var reply struct {
		Narrate            string  `json:"narrate"`
		SuggestedObjective *string `json:"suggestedObjective"`
		Done               bool    `json:"done"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if reply.Narrate == "" || reply.Done {
		t.Fatalf("reply = %+v — %s", reply, rr.Body)
	}
}

// TestQuestionCardTurn_SurfacesRealError — when the model reply can't be parsed
// (envelope drift, truncation, etc.), the endpoint must return a real error, NOT
// a fabricated 200 reply. A disguised canned sentence loops forever because the
// student can't tell the turn is dead. The client gets a 502 + machine reason.
func TestQuestionCardTurn_SurfacesRealError(t *testing.T) {
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "这个方向挺具体的，那按你的理解……"}, // prose, no JSON
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 5}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	h, cookie, _ := proposalTrackHandler(t, prov)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+seedProjectID+"/cards/question-card/turn",
		strings.NewReader(`{"messages":[{"role":"student","text":"帮我想想这题"}]}`)), cookie))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("unparseable reply should surface as 502, got %d — %s", rr.Code, rr.Body)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if body.Error.Code != "ai_dialogue_failed" {
		t.Fatalf("want ai_dialogue_failed, got %q — %s", body.Error.Code, rr.Body)
	}
	// The response must NOT smuggle a fabricated coach sentence.
	if strings.Contains(rr.Body.String(), "从你自己的直觉开始") {
		t.Fatalf("must not fabricate the canned opener: %s", rr.Body)
	}
}

func TestQuestionCardCommit_FillsObjective(t *testing.T) {
	h, cookie, pool := proposalTrackHandler(t, guideCardProvider())

	// empty objective → 422
	rrEmpty := httptest.NewRecorder()
	h.ServeHTTP(rrEmpty, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+seedProjectID+"/cards/question-card/commit",
		strings.NewReader(`{"objective":"  "}`)), cookie))
	if rrEmpty.Code != http.StatusBadRequest {
		t.Fatalf("empty objective should be rejected, got %d — %s", rrEmpty.Code, rrEmpty.Body)
	}

	// real objective → written
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+seedProjectID+"/cards/question-card/commit",
		strings.NewReader(`{"objective":"太阳能装机增长在多大程度上降低了中国单位GDP碳排放"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("commit = %d — %s", rr.Code, rr.Body)
	}
	prop, err := sqlc.New(pool).GetProjectProposal(context.Background(), mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("GetProjectProposal: %v", err)
	}
	if !strings.Contains(prop.Objective, "太阳能") {
		t.Fatalf("objective not written: %q", prop.Objective)
	}
}

// TestQuestionCard_NeverSummonedByCoach — the 提问卡 is no longer an
// AI-summonable card: it's a chatbox button that opens the modal directly
// (§2). Even a coach turn that emits summon_card{question-card} must be
// dropped — whether or not 目标 is empty.
func TestQuestionCard_NeverSummonedByCoach(t *testing.T) {
	summon := `{"narrate":"要不要用提问卡帮你想想？","tools":[{"name":"summon_card","args":{"card_id":"question-card","reason":"目标太泛","nudge_text":"一起收窄"}}]}`
	h, cookie, pool := orchestratorHandler(t, summon)
	setStudioStage(t, pool, seedProjectID, agent.StageProposalForming) // framework — question-card is in its deck
	base := "/api/v1/projects/" + seedProjectID

	call := func() *struct {
		Card *struct {
			CardID string `json:"cardId"`
		} `json:"card"`
	} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach", strings.NewReader(`{"user_input":"这题不知道怎么下手"}`)), cookie))
		if rr.Code != http.StatusOK {
			t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
		}
		var resp struct {
			Card *struct {
				CardID string `json:"cardId"`
			} `json:"card"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v — %s", err, rr.Body)
		}
		return &resp
	}

	// objective empty → the coach still must NOT summon question-card (it's a
	// button now, not a summonable card).
	if got := call(); got.Card != nil {
		t.Fatalf("coach must never summon question-card (it's a chatbox button), got %+v", got.Card)
	}

	// fill objective → still no summon.
	if _, err := sqlc.New(pool).UpsertProjectProposal(context.Background(), sqlc.UpsertProjectProposalParams{
		ProjectID: mustUUID(seedProjectID), Objective: "一个已经形成的研究问题", Reason: "", Activities: "", Resources: "", Counterpoints: "",
	}); err != nil {
		t.Fatalf("seed objective: %v", err)
	}
	if got := call(); got.Card != nil {
		t.Fatalf("coach must never summon question-card, got %+v", got.Card)
	}
}
