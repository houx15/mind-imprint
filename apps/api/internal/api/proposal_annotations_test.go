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

// TestDemoEssayAnnotations_SeededReadEndpoint verifies migration 0083's six
// seeded essay 批注 on the guided-tour demo project (…0200) render through the
// real GET endpoint the tour spotlights — decoded HTTP JSON, not a DB query —
// signed in as a NON-owner (SeedAdminID …005 ≠ Phoebe …003), proving the demo
// project's world-readable GET semantics (0081) extend to this doc.
func TestDemoEssayAnnotations_SeededReadEndpoint(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, CookieSecure: false}).Handler()
	nonOwner := signInAdmin(t, pool)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest(http.MethodGet,
		"/api/v1/projects/"+demoProjectID+"/proposal-annotations?doc=essay", nil), nonOwner))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET demo essay 批注 (non-owner) = %d, want 200 — %s", rr.Code, rr.Body)
	}
	got := decodeAnnotations(t, rr.Body.Bytes())
	if len(got) != 6 {
		t.Fatalf("want 6 seeded essay 批注, got %d: %+v", len(got), got)
	}

	want := []draftAnnotationJSON{
		{Level: "sentence", Nature: "good", Locator: "第2段",
			Quote: "这是一手、经同行评审、可复核的数据，构成本文最坚实的证据锚点。",
			Note:  "很扎实：这是追回的一手、同行评审论文，作为证据锚点比自媒体转述可靠得多。"},
		{Level: "sentence", Nature: "suggest", Locator: "第3段",
			Quote: "但同一篇论文也给了我一个至关重要的限定：中国的变绿主要来自农业集约化（约 32%）与大规模人工造林（约 42%），而非森林生态的自然恢复。",
			Note:  "这个限定抓得准，但可以补上论文里农业集约化与人工造林占比的具体页码/图表号，方便答辩时直接定位。"},
		{Level: "sentence", Nature: "suggest", Locator: "第4段",
			Quote: "国际能源署（IEA）《World Energy Investment 2023》显示，中国连续多年是全球最大的清洁能源投资国，2023 年其清洁能源投资约占全球的三成。",
			Note:  "投资数据很有力，但目前只写了结论、没写清楚具体统计口径——补全后这条证据才经得起追问。"},
		{Level: "sentence", Nature: "problem", Locator: "第5段",
			Quote: "中国自 2006 年起就是全球最大的年度二氧化碳排放国，2022 年约占全球排放的 31%。",
			Note:  "这两个数字（起始年份、31% 占比）缺少明确的引用来源和统计口径说明，答辩时如果被追问「这个数据最初从哪来」会站不住脚，需要标注出处。"},
		{Level: "sentence", Nature: "good", Locator: "第5段",
			Quote: "这条让步不是对结论的削弱，而是给它装上必要的限定条件。",
			Note:  "让步段处理得很好：没有回避对自己不利的反例，而是把它转成了限定条件而不是削弱结论。"},
		{Level: "sentence", Nature: "good", Locator: "第7段",
			Quote: "我意识到，「不被叙事俘获」不是一种态度，而是一套可以练习的操作——溯源、交叉验证、主动证伪。",
			Note:  "反思落在了方法而不是结论上——把「不被叙事俘获」讲成一套可执行的操作，这正是过程评估最想看到的元认知。"},
	}
	if len(got) != len(want) {
		t.Fatalf("annotation count mismatch: got %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Level != w.Level || g.Nature != w.Nature || g.Locator != w.Locator || g.Quote != w.Quote || g.Note != w.Note {
			t.Errorf("annotation[%d] = %+v, want %+v", i, g, w)
		}
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
