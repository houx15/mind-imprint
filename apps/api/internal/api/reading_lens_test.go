package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// reading_lens_test.go — the two student-driven halves of the reading card
// loop: she picks the lens (透镜库 summon) and she picks the sentence (选句
// 复核). Both were missing when Task 12 came to mount the real reading room:
// without them the room's 透镜库 button and its proposed→active→feedback
// pick flow have nothing to call.

// exampleScript is what ProposeCardExample's model must answer: one sentence
// quoted VERBATIM out of b1 of turnArticle, plus a why.
const exampleScript = `{"block_id":"b1","quote":"中国的太阳能装机量在过去十年增长了十倍。",` +
	`"why":"这句给了一个没有出处的数字。"}`

// evalScript is what EvaluateSelection's model must answer: three checks whose
// evidence is verbatim from the student's own span (anything else is dropped
// by the integrity rule), plus the finding/judgment prose.
const evalScript = `{"checks":[` +
	`{"key":"target","status":"pass","evidence":"碳排放总量","explanation":"你找的正是这篇的反例。"},` +
	`{"key":"evidence","status":"pass","evidence":"全球第一","explanation":"句子里有可直接引用的线索。"},` +
	`{"key":"centrality","status":"pass","evidence":"","explanation":"这条线索足够关键。"}],` +
	`"finding":"这句给出了与全文乐观基调相反的事实。","judgment":"文章回避了排放总量。",` +
	`"support":"碳排放总量仍居全球第一。","caveat":"总量不等于人均。","next_step":"去查人均排放。"}`

func postSummon(t *testing.T, h http.Handler, cookie *http.Cookie, id, cardID string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/summon", strings.NewReader(`{"cardId":"`+cardID+`"}`)), cookie))
	return rec
}

// lensCard is the card DTO as the reading room actually consumes it — the
// anchors array is the field this whole slice exists for.
type lensCard struct {
	ID      string  `json:"id"`
	CardID  string  `json:"cardId"`
	BlockID *string `json:"blockId"`
	Status  string  `json:"status"`
	Anchors []struct {
		BlockID  string `json:"block_id"`
		Start    int    `json:"start"`
		End      int    `json:"end"`
		Quote    string `json:"quote"`
		Author   string `json:"author"`
		Question string `json:"question"`
	} `json:"anchors"`
	Framework json.RawMessage `json:"framework"`
}

type lensTurn struct {
	Reply    string    `json:"reply"`
	Decision string    `json:"decision"`
	Nudge    string    `json:"nudge"`
	Card     *lensCard `json:"card"`
}

func decodeLensTurn(t *testing.T, rec *httptest.ResponseRecorder) lensTurn {
	t.Helper()
	var out lensTurn
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode summon reply: %v — body=%s", err, rec.Body)
	}
	return out
}

// TestSummonCard_MintsAProposedCardWithTheExampleAnchor — the 透镜库 path.
// The card must come back `proposed` (铁律②: she still confirms the open) and
// carry the FULL example anchor, not merely a block id: the room highlights
// the example sentence inside the paragraph and refuses a student pick that
// overlaps it, and both need the rune offsets and the verbatim quote.
func TestSummonCard_MintsAProposedCardWithTheExampleAnchor(t *testing.T) {
	h, cookie, id := turnHandler(t, routerStubProvider(exampleScript))

	rec := postSummon(t, h, cookie, id, "craap")
	if rec.Code != http.StatusOK {
		t.Fatalf("summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Decision != "summon" || out.Card == nil {
		t.Fatalf("summon reply = %+v, want a summon carrying a card", out)
	}
	if out.Card.CardID != "craap" || out.Card.Status != "proposed" {
		t.Fatalf("card = %+v, want craap/proposed", out.Card)
	}
	if out.Card.BlockID == nil || *out.Card.BlockID != "b1" {
		t.Fatalf("card blockId = %v, want b1", out.Card.BlockID)
	}
	if len(out.Card.Anchors) != 1 {
		t.Fatalf("anchors = %+v, want exactly one example anchor", out.Card.Anchors)
	}
	a := out.Card.Anchors[0]
	if a.BlockID != "b1" || a.Author != "ai" || a.Quote != "中国的太阳能装机量在过去十年增长了十倍。" {
		t.Fatalf("anchor = %+v, want the ai example grounded in b1", a)
	}
	if a.End <= a.Start {
		t.Fatalf("anchor span = [%d,%d), want a real non-empty range", a.Start, a.End)
	}
	if out.Nudge != "这句给了一个没有出处的数字。" {
		t.Fatalf("nudge = %q, want the model's why", out.Nudge)
	}

	// The SAME card must be what GET /cards returns — one card shape, no
	// second representation.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/cards", nil), cookie))
	var listed struct {
		Cards []lensCard `json:"cards"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode cards: %v — body=%s", err, rec2.Body)
	}
	if len(listed.Cards) != 1 || listed.Cards[0].ID != out.Card.ID || len(listed.Cards[0].Anchors) != 1 {
		t.Fatalf("GET /cards = %+v, want the same single anchored card", listed.Cards)
	}
}

// TestSummonCard_DeclinesWhileAnotherCardIsOpen — the one-active mutex. A
// decline is a conversation, not an error: 200 with a plain coach line and no
// second card, so the room shows a reply instead of a red failure.
func TestSummonCard_DeclinesWhileAnotherCardIsOpen(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(exampleScript))
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	seedCardWith(t, pool, id, "sift", "b1", "proposed")

	rec := postSummon(t, h, cookie, id, "craap")
	if rec.Code != http.StatusOK {
		t.Fatalf("busy summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Card != nil || out.Decision != "respond" {
		t.Fatalf("busy summon = %+v, want a card-less respond", out)
	}
	if !strings.Contains(out.Reply, "先完成") {
		t.Fatalf("busy reply = %q, want the 先完成当前这副透镜 line", out.Reply)
	}
}

// TestSummonCard_RefusesSiftBeforeCraap — the source-check ordering prior,
// derived the same way the router's OrderingGuard is. Nothing is minted.
func TestSummonCard_RefusesSiftBeforeCraap(t *testing.T) {
	h, cookie, id := turnHandler(t, routerStubProvider(exampleScript))

	rec := postSummon(t, h, cookie, id, "sift")
	if rec.Code != http.StatusOK {
		t.Fatalf("sift-first summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Card != nil || !strings.Contains(out.Reply, "CRAAP") {
		t.Fatalf("sift-first summon = %+v, want the CRAAP-first decline", out)
	}
}

// TestSummonCard_RejectsACardOutsideTheReadingDeck — the lens library may only
// summon a card that is actually in the reading deck, never an arbitrary
// registry id.
func TestSummonCard_RejectsACardOutsideTheReadingDeck(t *testing.T) {
	h, cookie, id := turnHandler(t, routerStubProvider(exampleScript))

	rec := postSummon(t, h, cookie, id, "concession")
	if rec.Code != http.StatusOK {
		t.Fatalf("off-deck summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Card != nil || !strings.Contains(out.Reply, "可用的阅读透镜") {
		t.Fatalf("off-deck summon = %+v, want the not-a-lens decline", out)
	}
}

// TestSummonCard_OpensTheLensEvenWithNoGroundableExample — a student-chosen
// summon must NEVER dead-end. When the model cannot ground one illustrative
// sentence the lens still opens (anchors empty) and she goes straight to
// finding her own.
func TestSummonCard_OpensTheLensEvenWithNoGroundableExample(t *testing.T) {
	// A quote that is not a substring of any block → ProposeCardExample rejects.
	h, cookie, id := turnHandler(t, routerStubProvider(`{"block_id":"b1","quote":"这句话不在文章里。","why":"x"}`))

	rec := postSummon(t, h, cookie, id, "craap")
	if rec.Code != http.StatusOK {
		t.Fatalf("ungrounded summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Card == nil || out.Card.Status != "proposed" {
		t.Fatalf("ungrounded summon = %+v, want the lens to open anyway", out)
	}
	if len(out.Card.Anchors) != 0 {
		t.Fatalf("anchors = %+v, want none — never fabricate an example", out.Card.Anchors)
	}
	if !strings.Contains(out.Nudge, "挑一句") {
		t.Fatalf("nudge = %q, want the find-your-own-sentence nudge", out.Nudge)
	}
}

// TestSummonCard_MetersTheGroundingCall — the summon spends a real model call,
// so it must leave a metering row against this atom.
func TestSummonCard_MetersTheGroundingCall(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(exampleScript))
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := postSummon(t, h, cookie, id, "craap"); rec.Code != http.StatusOK {
		t.Fatalf("summon = %d; body=%s", rec.Code, rec.Body)
	}
	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM llm_call WHERE atom_id = $1 AND surface = 'lite' AND purpose = 'read_card_example'`,
		mustUUID(id)).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if n != 1 {
		t.Fatalf("read_card_example llm_call rows = %d, want 1", n)
	}
}

// TestEvaluateSelection_ReviewsThePickAndPersistsIt — the 选句复核. The room
// blocks on this response before it can show feedback, and 过程即数据 says the
// AI's judgment of her pick is a recorded fact, not just browser state.
func TestEvaluateSelection_ReviewsThePickAndPersistsIt(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(evalScript))
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	cardID := seedCard(t, pool, id, "b1")

	body := `{"block_id":"b2","start":0,"end":10,` +
		`"quote":"但同一时期，中国的碳排放总量仍居全球第一。","dimension":"craap"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/cards/"+cardID+"/evaluate", strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("evaluate = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Verdict      string `json:"verdict"`
		VerdictLabel string `json:"verdictLabel"`
		Checks       []struct {
			Key      string `json:"key"`
			Status   string `json:"status"`
			Evidence string `json:"evidence"`
		} `json:"checks"`
		Finding string   `json:"finding"`
		SpanIDs []string `json:"spanIds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode eval: %v — body=%s", err, rec.Body)
	}
	if out.Verdict != "strong" || len(out.Checks) != 3 {
		t.Fatalf("eval = %+v, want a 3-check strong verdict", out)
	}
	if out.Finding != "这句给出了与全文乐观基调相反的事实。" {
		t.Fatalf("finding = %q, want the model's finding", out.Finding)
	}
	if len(out.SpanIDs) != 1 || out.SpanIDs[0] != "sel0" {
		t.Fatalf("spanIds = %v, want only the student's own span", out.SpanIDs)
	}

	// The card must NOT have been advanced — evaluate is a read-with-a-model,
	// the confirm step is what submits.
	var status string
	var framework []byte
	if err := pool.QueryRow(t.Context(),
		`SELECT status, framework_fill FROM atom_card WHERE id = $1`, mustUUID(cardID)).
		Scan(&status, &framework); err != nil {
		t.Fatalf("read card: %v", err)
	}
	if status != "proposed" {
		t.Fatalf("card status = %q after evaluate, want it untouched", status)
	}
	if !strings.Contains(string(framework), "这句给出了与全文乐观基调相反的事实。") {
		t.Fatalf("framework_fill = %s, want the review persisted", framework)
	}

	// And it must be metered — this spends a flagship call.
	var n int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM llm_call WHERE atom_id = $1 AND purpose = 'read_eval'`,
		mustUUID(id)).Scan(&n); err != nil {
		t.Fatalf("count llm_call: %v", err)
	}
	if n != 1 {
		t.Fatalf("read_eval llm_call rows = %d, want 1", n)
	}
}

// TestEvaluateSelection_DegradesRatherThanBreakingTheLoop — a provider outage
// during the review must NOT strand the student mid-card with no way forward.
// Unlike the coach turn (where a canned reply would be a lie about a
// conversation), the deterministic fallback here states plainly that it is
// recording her choice and makes no specific claim about her sentence.
func TestEvaluateSelection_DegradesRatherThanBreakingTheLoop(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, streamErrorProvider{})
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	cardID := seedCard(t, pool, id, "b1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/cards/"+cardID+"/evaluate",
		strings.NewReader(`{"block_id":"b1","start":0,"end":5,"quote":"中国的太阳能装机量在过去十年增长了十倍。","dimension":"craap"}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("degraded evaluate = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Verdict string `json:"verdict"`
		Checks  []struct {
			Evidence string `json:"evidence"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Verdict != "partial" || len(out.Checks) != 3 {
		t.Fatalf("degraded eval = %+v, want the neutral 3-check partial", out)
	}
	for _, c := range out.Checks {
		if c.Evidence != "" {
			t.Fatalf("degraded eval fabricated evidence %q", c.Evidence)
		}
	}
}

// TestEvaluateSelection_RequiresAQuote — an empty pick is a client bug, not a
// reason to spend a flagship call.
func TestEvaluateSelection_RequiresAQuote(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(evalScript))
	id := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, id, "b1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/cards/"+cardID+"/evaluate",
		strings.NewReader(`{"block_id":"b1","start":0,"end":0,"quote":"  ","dimension":"craap"}`)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("blank pick = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

// TestEvaluateSelection_CrossAtomIs404 — the same IDOR guard every other card
// endpoint carries: a card that belongs to a DIFFERENT reading is invisible.
func TestEvaluateSelection_CrossAtomIs404(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, routerStubProvider(evalScript))
	mine := createReadingAtom(t, h, cookie)
	other := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, other, "b1")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+mine+"/cards/"+cardID+"/evaluate",
		strings.NewReader(`{"block_id":"b1","start":0,"end":5,"quote":"中国的太阳能装机量在过去十年增长了十倍。","dimension":"craap"}`)), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-atom evaluate = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}

// TestSubmitCard_RecordsTheStudentsOwnAnchor — 过程即数据. The submit used to
// drop `anchors` on the floor, which meant the one thing the reading loop is
// ABOUT — which sentence she chose — was never recorded.
func TestSubmitCard_RecordsTheStudentsOwnAnchor(t *testing.T) {
	h, cookie, _, pool := liteHandler(t)
	id := createReadingAtom(t, h, cookie)
	cardID := seedCard(t, pool, id, "b1")

	body := `{"fieldValues":{},"eventTrace":[],"anchors":[{"id":"sel0","material_id":"` + id +
		`","block_id":"b2","start":0,"end":8,"quote":"但同一时期","dimension":"craap","author":"student","question":"","answer":""}]}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/cards/"+cardID+"/submit", strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out lensCard
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Anchors) != 1 || out.Anchors[0].Author != "student" || out.Anchors[0].BlockID != "b2" {
		t.Fatalf("anchors = %+v, want the student's own pick", out.Anchors)
	}
}

// TestSubmitCard_KeepsTheSummonAnchorWhenNoneIsSent — omitting anchors must
// not erase the example the card was summoned with.
func TestSubmitCard_KeepsTheSummonAnchorWhenNoneIsSent(t *testing.T) {
	h, cookie, id := turnHandler(t, routerStubProvider(exampleScript))
	summoned := decodeLensTurn(t, postSummon(t, h, cookie, id, "craap"))
	if summoned.Card == nil {
		t.Fatalf("summon produced no card: %+v", summoned)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/cards/"+summoned.Card.ID+"/submit",
		strings.NewReader(`{"fieldValues":{},"eventTrace":[]}`)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out lensCard
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if len(out.Anchors) != 1 || out.Anchors[0].Author != "ai" {
		t.Fatalf("anchors = %+v, want the summon's example kept", out.Anchors)
	}
}

// TestReadingTurn_SummonPersistsTheExampleAnchor — the router-proposed summon
// must persist the SAME full anchor the student-chosen one does. Without it
// the card hangs on the first paragraph instead of the one the AI grounded it
// in, and there is no example sentence to show her.
func TestReadingTurn_SummonPersistsTheExampleAnchor(t *testing.T) {
	h, cookie, id := turnHandler(t, routerStubProvider(summonScript))

	rec := postTurn(t, h, cookie, id, `{"text":"这个十倍的数字是谁统计的？"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Decision != "summon" || out.Card == nil {
		t.Fatalf("turn = %+v, want a summon with a card", out)
	}
	if len(out.Card.Anchors) != 1 {
		t.Fatalf("anchors = %+v, want the resolved example anchor", out.Card.Anchors)
	}
	a := out.Card.Anchors[0]
	if a.BlockID != "b1" || a.Quote != "中国的太阳能装机量在过去十年增长了十倍。" || a.End <= a.Start {
		t.Fatalf("anchor = %+v, want the verbatim b1 span", a)
	}
	if a.Question != "这句给了一个没有出处的数字。" {
		t.Fatalf("anchor question = %q, want the example_why", a.Question)
	}
}
