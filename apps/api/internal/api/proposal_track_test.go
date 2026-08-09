package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func guideCardProvider() *gateway.SequenceStubProvider {
	card := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"prompt":"针对当前题目的引导问题","example":"An English example","refHint":"framework 目标"}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 5}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	}
	return gateway.NewSequenceStubProvider(card) // repeats the one script
}

func proposalTrackHandler(t *testing.T, prov gateway.Provider) (http.Handler, *http.Cookie, *pgxpool.Pool) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: prov, ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(),
		EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool), pool
}

func trackReq(t *testing.T, h http.Handler, cookie *http.Cookie, method, path, body string) proposalStepResp {
	t.Helper()
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	} else {
		rdr = strings.NewReader("")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest(method, "/api/v1/projects/"+seedProjectID+path, rdr), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("%s %s = %d — %s", method, path, rr.Code, rr.Body)
	}
	var out proposalStepResp
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %s: %v — %s", path, err, rr.Body)
	}
	return out
}

type proposalStepResp struct {
	Key          string `json:"key"`
	Kind         string `json:"kind"`
	Index        int    `json:"index"`
	Total        int    `json:"total"`
	Mode         string `json:"mode"`
	Started      bool   `json:"started"`
	SubQuestions []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"subQuestions"`
	Card *struct {
		Prompt  string `json:"prompt"`
		Example string `json:"example"`
	} `json:"card"`
}

func TestProposalTrack_FreshUnchosen(t *testing.T) {
	h, cookie, _ := proposalTrackHandler(t, guideCardProvider())
	got := trackReq(t, h, cookie, "GET", "/proposal-track", "")
	if got.Mode != "" || got.Started {
		t.Fatalf("fresh track should be unchosen: %+v", got)
	}
	if got.Total != 9 || got.Key != "understanding" {
		t.Fatalf("fresh derived list wrong: %+v", got)
	}
	if got.Card != nil {
		t.Fatalf("no card before guided+started, got %+v", got.Card)
	}
}

func TestProposalTrack_GuidedStartGeneratesCardOnce(t *testing.T) {
	prov := guideCardProvider()
	h, cookie, _ := proposalTrackHandler(t, prov)

	if got := trackReq(t, h, cookie, "POST", "/proposal-track/mode", `{"mode":"guided"}`); got.Mode != "guided" || got.Started {
		t.Fatalf("after mode=guided: %+v", got)
	}
	got := trackReq(t, h, cookie, "POST", "/proposal-track/start", "")
	if !got.Started || got.Index != 0 || got.Card == nil {
		t.Fatalf("after start expected started+card at index 0: %+v", got)
	}
	callsAfterStart := prov.Calls

	// A second GET on the SAME step must read the cache — no new model call.
	got2 := trackReq(t, h, cookie, "GET", "/proposal-track", "")
	if got2.Card == nil {
		t.Fatal("card should still be present (cached)")
	}
	if prov.Calls != callsAfterStart {
		t.Fatalf("guide card re-generated on re-read: calls %d → %d (should be cached)", callsAfterStart, prov.Calls)
	}
}

func TestProposalTrack_SubQuestionsExpandTrack(t *testing.T) {
	h, cookie, _ := proposalTrackHandler(t, guideCardProvider())
	trackReq(t, h, cookie, "POST", "/proposal-track/mode", `{"mode":"guided"}`)
	trackReq(t, h, cookie, "POST", "/proposal-track/start", "")

	got := trackReq(t, h, cookie, "POST", "/proposal-track/subquestions",
		`{"subQuestions":[{"text":"q1"},{"text":"q2"},{"text":"q3"}]}`)
	if got.Total != 12 || len(got.SubQuestions) != 3 {
		t.Fatalf("3 sub-questions should give total 12: %+v", got)
	}
	if got.SubQuestions[0].ID == "" {
		t.Fatal("server must mint sub-question ids")
	}

	// Walk to the first sub-question card (understanding→scope→thesis→define→subq).
	var last proposalStepResp
	for i := 0; i < 4; i++ {
		last = trackReq(t, h, cookie, "POST", "/proposal-track/advance", `{"dir":"next"}`)
	}
	if last.Kind != "subq" {
		t.Fatalf("after 4 nexts expected a subq step, got %+v", last)
	}
}

// (The 我写好了 review now returns 批注, not a ReviewVerdict — covered by
// TestProposalTrackReview_ReturnsAnnotations in proposal_annotations_test.go.)
