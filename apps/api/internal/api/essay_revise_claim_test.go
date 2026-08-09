package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// claimRevisionProvider returns a total_change verdict for the classifier call.
func claimRevisionProvider() gateway.Provider {
	return gateway.NewSequenceStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"kind":"total_change","why":"问的是另一个对象，原材料不再适用"}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 5}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func reviseClaimHandler(t *testing.T) (http.Handler, *http.Cookie, *sqlc.Queries) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: claimRevisionProvider(), ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(),
		EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool), sqlc.New(pool)
}

// seedResearchStatement seeds a project in the essay RESEARCH stage with two
// sub-questions (for the research→statement deep-link).
func seedResearchStatement(t *testing.T, q *sqlc.Queries) {
	t.Helper()
	st := agent.DefaultStudioState()
	st.Started = true
	st.Stage = agent.StageBodyWriting
	st.EssayTrack = &agent.EssayTrack{Stage: agent.EssayResearch}
	st.ProposalTrack = &agent.WritingTrack{SubQuestions: []agent.SubQuestion{{ID: "a", Text: "论点甲"}, {ID: "b", Text: "论点乙"}}}
	b, _ := json.Marshal(st)
	if err := q.SetStudioState(context.Background(), sqlc.SetStudioStateParams{ID: mustUUID(seedProjectID), StudioState: b}); err != nil {
		t.Fatalf("set state: %v", err)
	}
}

func TestReviseEssayClaim(t *testing.T) {
	h, cookie, q := reviseClaimHandler(t)
	seedStatementProject(t, q) // subs a="论点甲", b="论点乙"
	base := "/api/v1/projects/" + seedProjectID

	post := func(path, body string) (int, map[string]any) {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+path, strings.NewReader(body)), cookie))
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return rr.Code, out
	}
	currentText := func(id string) string {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/essay-statement", nil), cookie))
		var out struct {
			SubQuestions []struct{ ID, Text string } `json:"subQuestions"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		for _, sq := range out.SubQuestions {
			if sq.ID == id {
				return sq.Text
			}
		}
		return ""
	}

	// Empty new text → 400.
	if code, _ := post("/essay-statement/revise-claim", `{"subQuestionId":"a","newText":"  "}`); code != http.StatusBadRequest {
		t.Fatalf("empty newText = %d, want 400", code)
	}
	// Unchanged text → 400.
	if code, _ := post("/essay-statement/revise-claim", `{"subQuestionId":"a","newText":"论点甲"}`); code != http.StatusBadRequest {
		t.Fatalf("unchanged newText = %d, want 400", code)
	}
	// Unknown sub-question → 404.
	if code, _ := post("/essay-statement/revise-claim", `{"subQuestionId":"zzz","newText":"新"}`); code != http.StatusNotFound {
		t.Fatalf("unknown sq = %d, want 404", code)
	}

	// confirm=false → verdict returned, NOT persisted.
	code, out := post("/essay-statement/revise-claim", `{"subQuestionId":"a","newText":"一个全新的问题","confirm":false}`)
	if code != http.StatusOK {
		t.Fatalf("classify = %d", code)
	}
	if out["applied"] != false {
		t.Fatalf("confirm=false should not apply: %+v", out)
	}
	if v, _ := out["verdict"].(map[string]any); v["kind"] != "total_change" {
		t.Fatalf("verdict kind = %+v", out["verdict"])
	}
	if got := currentText("a"); got != "论点甲" {
		t.Fatalf("confirm=false persisted text: %q", got)
	}

	// confirm=true → persisted.
	code, out = post("/essay-statement/revise-claim", `{"subQuestionId":"a","newText":"一个全新的问题","confirm":true}`)
	if code != http.StatusOK || out["applied"] != true {
		t.Fatalf("confirm=true = %d %+v", code, out)
	}
	if got := currentText("a"); got != "一个全新的问题" {
		t.Fatalf("confirm=true not persisted: %q", got)
	}
}

func TestAdvanceEssayStage_DeepLinkClaim(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: guideCardProvider(), ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(),
		EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	// research-stage project with two sub-questions.
	seedResearchStatement(t, q)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/essay-track/advance-stage", strings.NewReader(`{"stage":"statement","claimId":"b"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("advance-stage = %d — %s", rr.Code, rr.Body)
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/essay-statement", nil), cookie))
	var got map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got["started"] != true || got["key"] != "claim:b" {
		t.Fatalf("deep-link should land on claim:b started, got %+v", got)
	}
}
