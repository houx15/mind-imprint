package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

func annotationProvider() gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"annotations":[` +
			`{"level":"paper","nature":"good","quote":"","locator":"","note":"整体结构清晰"},` +
			`{"level":"sentence","nature":"problem","quote":"中国一定会成功","locator":"第2段","note":"这是断言，缺证据"}` +
			`]}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 20, OutputTokens: 10}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func annotationsHandler(t *testing.T, prov gateway.Provider) (http.Handler, *http.Cookie) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: prov, ChatResolver: fakeResolver(), FastChatResolver: fakeResolver(),
		EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool)
}

func decodeAnnotations(t *testing.T, body []byte) []draftAnnotationJSON {
	t.Helper()
	var resp struct {
		Annotations []draftAnnotationJSON `json:"annotations"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, body)
	}
	return resp.Annotations
}

type draftAnnotationJSON struct {
	ID      string `json:"id"`
	Level   string `json:"level"`
	Nature  string `json:"nature"`
	Quote   string `json:"quote"`
	Locator string `json:"locator"`
	Note    string `json:"note"`
}

func TestProposalAnnotations_ReviewAndList(t *testing.T) {
	h, cookie := annotationsHandler(t, annotationProvider())
	base := "/api/v1/projects/" + seedProjectID

	// Seed a proposal buffer.
	rrBuf := httptest.NewRecorder()
	h.ServeHTTP(rrBuf, withCookie(httptest.NewRequest("PUT", base+"/buffer?doc=proposal", strings.NewReader(`{"content":"我的提案……中国一定会成功。"}`)), cookie))
	if rrBuf.Code != http.StatusNoContent && rrBuf.Code != http.StatusOK {
		t.Fatalf("PUT buffer = %d — %s", rrBuf.Code, rrBuf.Body)
	}

	// Whole-draft review → 批注.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/proposal-annotations/review", strings.NewReader("")), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("review = %d — %s", rr.Code, rr.Body)
	}
	got := decodeAnnotations(t, rr.Body.Bytes())
	if len(got) != 2 {
		t.Fatalf("want 2 annotations, got %d: %+v", len(got), got)
	}
	var paper, sentence *draftAnnotationJSON
	for i := range got {
		switch got[i].Level {
		case "paper":
			paper = &got[i]
		case "sentence":
			sentence = &got[i]
		}
	}
	if paper == nil || paper.Nature != "good" {
		t.Fatalf("paper/good annotation missing: %+v", got)
	}
	if sentence == nil || sentence.Nature != "problem" || sentence.Quote != "中国一定会成功" {
		t.Fatalf("sentence/problem annotation missing: %+v", got)
	}

	// GET returns the same persisted set.
	rrGet := httptest.NewRecorder()
	h.ServeHTTP(rrGet, withCookie(httptest.NewRequest("GET", base+"/proposal-annotations", nil), cookie))
	if rrGet.Code != http.StatusOK {
		t.Fatalf("GET = %d — %s", rrGet.Code, rrGet.Body)
	}
	if len(decodeAnnotations(t, rrGet.Body.Bytes())) != 2 {
		t.Fatalf("GET should return the persisted 2 annotations")
	}

	// A second review REPLACES (no accretion).
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("POST", base+"/proposal-annotations/review", strings.NewReader("")), cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("review2 = %d — %s", rr2.Code, rr2.Body)
	}
	if len(decodeAnnotations(t, rr2.Body.Bytes())) != 2 {
		t.Fatalf("re-review must REPLACE, not accrete — got %d", len(decodeAnnotations(t, rr2.Body.Bytes())))
	}
}

func TestProposalTrackReview_ReturnsAnnotations(t *testing.T) {
	h, cookie := annotationsHandler(t, annotationProvider())
	base := "/api/v1/projects/" + seedProjectID

	rrBuf := httptest.NewRecorder()
	h.ServeHTTP(rrBuf, withCookie(httptest.NewRequest("PUT", base+"/buffer?doc=proposal", strings.NewReader(`{"content":"草稿。中国一定会成功。"}`)), cookie))
	if rrBuf.Code != http.StatusNoContent && rrBuf.Code != http.StatusOK {
		t.Fatalf("PUT buffer = %d — %s", rrBuf.Code, rrBuf.Body)
	}

	// 我写好了 now returns {annotations:[...]}, not {ready,...}.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/proposal-track/review", strings.NewReader(`{}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("review = %d — %s", rr.Code, rr.Body)
	}
	if len(decodeAnnotations(t, rr.Body.Bytes())) == 0 {
		t.Fatalf("我写好了 should return 批注 — %s", rr.Body)
	}
}
