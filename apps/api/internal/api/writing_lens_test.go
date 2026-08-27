package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// writing_lens_test.go — writing's own 工具卡 door: POST /writings/{id}/summon
// plus the five shared card-lifecycle routes (list/activate/skip/submit/
// evaluate) newly mounted under /writings/*. Mirrors reading_lens_test.go and
// reading_cards_test.go's shape wherever the two domains overlap; the tests
// distinctive to writing are the grounding-in-outline/snippets behaviour and
// the cross-kind isolation / metering-purpose checks this task exists for.

// dynamicProvider lets a test script the model's reply AFTER creating rows
// whose ids the script needs to reference — e.g. a snippet uuid only known
// from an earlier HTTP response. liteHandlerWithProvider fixes Provider at
// handler construction, before those ids exist; swap Inner any time before
// the next call that reaches the model.
type dynamicProvider struct {
	Inner gateway.Provider
}

func (p *dynamicProvider) Stream(ctx context.Context, r gateway.Resolved, req gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	return p.Inner.Stream(ctx, r, req)
}

func postWritingSummon(t *testing.T, h http.Handler, cookie *http.Cookie, id, cardID string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/writings/"+id+"/summon", strings.NewReader(`{"cardId":"`+cardID+`"}`)), cookie))
	return rec
}

// TestWritingSummonCard_GroundsInHerOwnSnippet — the whole point of this
// task's writing summon: it grounds the card in what writing actually has.
// Here that is a snippet she has already written; the anchor the model comes
// back with must point at that snippet's own id, not at an article block
// (writing has no article).
func TestWritingSummonCard_GroundsInHerOwnSnippet(t *testing.T) {
	dyn := &dynamicProvider{Inner: writingTextStubProvider(`{"decision":"never used"}`)}
	h, cookie, _, _ := liteHandlerWithProvider(t, dyn)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于可持续发展的议论文")

	if rec := putWritingSnippetsHTTP(t, h, cookie, id,
		`{"snippets":[{"position":0,"text":"中国的太阳能装机量在过去十年增长了十倍。"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put snippet = %d; body=%s", rec.Code, rec.Body)
	}
	snippets := getWritingSnippetsHTTP(t, h, cookie, id)
	if len(snippets) != 1 {
		t.Fatalf("snippets = %+v, want 1", snippets)
	}
	blockID := "snippet:" + snippets[0].ID

	dyn.Inner = writingTextStubProvider(`{"block_id":"` + blockID +
		`","quote":"中国的太阳能装机量在过去十年增长了十倍。","why":"这是一个可以核实的具体主张。"}`)

	rec := postWritingSummon(t, h, cookie, id, "toulmin")
	if rec.Code != http.StatusOK {
		t.Fatalf("summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Decision != "summon" || out.Card == nil {
		t.Fatalf("summon reply = %+v, want a summon carrying a card", out)
	}
	if out.Card.CardID != "toulmin" || out.Card.Status != "proposed" {
		t.Fatalf("card = %+v, want toulmin/proposed", out.Card)
	}
	if out.Card.BlockID == nil || *out.Card.BlockID != blockID {
		t.Fatalf("card blockId = %v, want %q (her own snippet)", out.Card.BlockID, blockID)
	}
	if len(out.Card.Anchors) != 1 || out.Card.Anchors[0].BlockID != blockID {
		t.Fatalf("anchors = %+v, want exactly one anchor grounded in %q", out.Card.Anchors, blockID)
	}
	if out.Card.Anchors[0].Author != "ai" {
		t.Fatalf("anchor author = %q, want ai (the summon's own example)", out.Card.Anchors[0].Author)
	}
}

// TestWritingSummonCard_OpensWithNoGroundableMaterial — a brand-new writing
// has neither a confirmed outline point nor a written snippet yet (still in
// 构思). The summon must still open the card (a student-chosen summon never
// dead-ends, mirroring reading), AND — 铁律②'s restraint on spend, not just
// dialogue — it must not even attempt a model call when there is nothing to
// ground an example in. The counting provider proves the second half.
func TestWritingSummonCard_OpensWithNoGroundableMaterial(t *testing.T) {
	counter := &countingProvider{inner: writingTextStubProvider(`{"block_id":"x","quote":"x","why":"x"}`)}
	h, cookie, _, _ := liteHandlerWithProvider(t, counter)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于可持续发展的议论文")

	rec := postWritingSummon(t, h, cookie, id, "toulmin")
	if rec.Code != http.StatusOK {
		t.Fatalf("summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Card == nil || out.Card.Status != "proposed" {
		t.Fatalf("ungrounded summon = %+v, want the card to open anyway", out)
	}
	if len(out.Card.Anchors) != 0 {
		t.Fatalf("anchors = %+v, want none — nothing to ground on yet", out.Card.Anchors)
	}
	if !strings.Contains(out.Nudge, "提纲") && !strings.Contains(out.Nudge, "段落") {
		t.Fatalf("nudge = %q, want the find-it-yourself-in-outline/snippets nudge", out.Nudge)
	}
	if counter.calls != 0 {
		t.Fatalf("provider was called %d time(s) with nothing to ground on — must skip the model call entirely", counter.calls)
	}
}

// TestWritingSummonCard_RejectsACardOutsideTheWritingDeck — the writing tool
// library may only summon a card that is actually in its deck (the
// 论证写作-category cards), never an arbitrary registry id — and certainly
// not a reading-only lens like craap.
func TestWritingSummonCard_RejectsACardOutsideTheWritingDeck(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider(`{}`))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于可持续发展的议论文")

	rec := postWritingSummon(t, h, cookie, id, "craap")
	if rec.Code != http.StatusOK {
		t.Fatalf("off-deck summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Card != nil || !strings.Contains(out.Reply, "可用的写作工具卡") {
		t.Fatalf("off-deck summon = %+v, want the not-a-writing-card decline", out)
	}
}

// TestWritingSummonCard_DeclinesWhileAnotherCardIsOpen — same one-active
// mutex as reading (atom_card_one_open_idx is atom-scoped, not
// reading-scoped).
func TestWritingSummonCard_DeclinesWhileAnotherCardIsOpen(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, writingTextStubProvider(`{}`))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于可持续发展的议论文")
	seedCardWith(t, pool, id, "pee", "b1", "proposed")

	rec := postWritingSummon(t, h, cookie, id, "toulmin")
	if rec.Code != http.StatusOK {
		t.Fatalf("busy summon = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	out := decodeLensTurn(t, rec)
	if out.Card != nil || out.Decision != "respond" {
		t.Fatalf("busy summon = %+v, want a card-less respond", out)
	}
	if !strings.Contains(out.Reply, "先完成") {
		t.Fatalf("busy reply = %q, want the 先完成当前这张工具卡 line", out.Reply)
	}
}

// TestWritingCardLifecycle_ListActivateSubmitAndSkip — the five newly-mounted
// card routes actually work end to end for a writing: a card summoned there
// can be listed, activated, submitted, and (a second one) skipped.
func TestWritingCardLifecycle_ListActivateSubmitAndSkip(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, writingTextStubProvider(`{}`))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于可持续发展的议论文")
	cardID := seedCard(t, pool, id, "b1") // seedCard is atom-id-keyed, kind-agnostic

	// list
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/cards", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET cards = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var listed struct {
		Cards []lensCard `json:"cards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode cards: %v — body=%s", err, rec.Body)
	}
	if len(listed.Cards) != 1 || listed.Cards[0].ID != cardID {
		t.Fatalf("GET /writings/%s/cards = %+v, want the seeded card", id, listed.Cards)
	}

	// activate
	actRec := httptest.NewRecorder()
	h.ServeHTTP(actRec, withCookie(httptest.NewRequest("POST",
		"/api/v1/writings/"+id+"/cards/"+cardID+"/activate", strings.NewReader("{}")), cookie))
	if actRec.Code != http.StatusOK {
		t.Fatalf("activate = %d, want 200; body=%s", actRec.Code, actRec.Body)
	}

	// submit
	subRec := httptest.NewRecorder()
	h.ServeHTTP(subRec, withCookie(httptest.NewRequest("POST",
		"/api/v1/writings/"+id+"/cards/"+cardID+"/submit",
		strings.NewReader(`{"fieldValues":{"claim":"值得推广"},"eventTrace":[{"t":"open"}]}`)), cookie))
	if subRec.Code != http.StatusOK {
		t.Fatalf("submit = %d, want 200; body=%s", subRec.Code, subRec.Body)
	}
	var subOut struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(subRec.Body.Bytes(), &subOut)
	if subOut.Status != "submitted" {
		t.Fatalf("status after submit = %q, want submitted", subOut.Status)
	}

	// skip a SECOND card
	cardID2 := seedCard(t, pool, id, "b1")
	skipRec := httptest.NewRecorder()
	h.ServeHTTP(skipRec, withCookie(httptest.NewRequest("POST",
		"/api/v1/writings/"+id+"/cards/"+cardID2+"/skip", strings.NewReader("{}")), cookie))
	if skipRec.Code != http.StatusOK {
		t.Fatalf("skip = %d, want 200; body=%s", skipRec.Code, skipRec.Body)
	}
	var status string
	if err := pool.QueryRow(t.Context(),
		`SELECT status FROM atom_card WHERE id = $1`, mustUUID(cardID2)).Scan(&status); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if status != "skipped" {
		t.Fatalf("status = %q, want skipped (the row must survive)", status)
	}
}

// TestWritingCardDoor_CrossKindIsolation — a reading atom id must not be
// reachable through a /writings/* card route, and a writing atom id must not
// be reachable through a /readings/* card route. Both are the SAME flat 404
// loadOwnedAtom already gives for any kind mismatch — this pins it for the
// newly-mounted writing routes specifically.
func TestWritingCardDoor_CrossKindIsolation(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, writingTextStubProvider(`{}`))
	readingID := createReadingAtom(t, h, cookie)
	writingID := createWritingAtomHTTP(t, h, cookie, "写一篇关于可持续发展的议论文")

	readingCard := seedCard(t, pool, readingID, "b1")
	writingCard := seedCard(t, pool, writingID, "b1")

	// A reading id at a writing card route → 404.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+readingID+"/cards", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("reading id at /writings/*/cards = %d, want 404; body=%s", rec.Code, rec.Body)
	}
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("POST",
		"/api/v1/writings/"+readingID+"/cards/"+readingCard+"/activate", strings.NewReader("{}")), cookie))
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("reading id at /writings/*/cards/activate = %d, want 404; body=%s", rec2.Code, rec2.Body)
	}

	// A writing id at a reading card route → 404, the other direction.
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+writingID+"/cards", nil), cookie))
	if rec3.Code != http.StatusNotFound {
		t.Fatalf("writing id at /readings/*/cards = %d, want 404; body=%s", rec3.Code, rec3.Body)
	}
	rec4 := httptest.NewRecorder()
	h.ServeHTTP(rec4, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+writingID+"/cards/"+writingCard+"/activate", strings.NewReader("{}")), cookie))
	if rec4.Code != http.StatusNotFound {
		t.Fatalf("writing id at /readings/*/cards/activate = %d, want 404; body=%s", rec4.Code, rec4.Body)
	}

	// And the writing summon door itself must refuse a reading id.
	rec5 := postWritingSummon(t, h, cookie, readingID, "toulmin")
	if rec5.Code != http.StatusNotFound {
		t.Fatalf("reading id at /writings/*/summon = %d, want 404; body=%s", rec5.Code, rec5.Body)
	}
}

// TestWritingEvaluateCardSelection_MetersWriteEvalNotReadEval — Part A item
// 3: once writing mounts the shared evaluate route, its flagship-tier spend
// must be metered under its OWN purpose, not silently rolled into reading's
// "read_eval" cost bucket.
func TestWritingEvaluateCardSelection_MetersWriteEvalNotReadEval(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, writingTextStubProvider(evalScript))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于可持续发展的议论文")
	cardID := seedCard(t, pool, id, "b1")

	body := `{"block_id":"b1","start":0,"end":10,"quote":"值得推广的做法","dimension":"toulmin"}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/writings/"+id+"/cards/"+cardID+"/evaluate", strings.NewReader(body)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("evaluate = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var writeCount, readCount int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM llm_call WHERE atom_id = $1 AND purpose = 'write_eval'`,
		mustUUID(id)).Scan(&writeCount); err != nil {
		t.Fatalf("count write_eval llm_call: %v", err)
	}
	if writeCount != 1 {
		t.Fatalf("write_eval llm_call rows = %d, want 1", writeCount)
	}
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM llm_call WHERE atom_id = $1 AND purpose = 'read_eval'`,
		mustUUID(id)).Scan(&readCount); err != nil {
		t.Fatalf("count read_eval llm_call: %v", err)
	}
	if readCount != 0 {
		t.Fatalf("read_eval llm_call rows for a WRITING evaluate = %d, want 0 — must not file under reading's bucket", readCount)
	}
}
