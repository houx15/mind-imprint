package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// reading_turn_test.go — the lite edition's coach turn. The point of these
// tests is the reuse thesis: agent.RouteReading / ApplyReadingGate run
// UNCHANGED over the atom tables, so what is asserted here is the handler's
// three jobs — assemble, persist, meter — plus the standing product rule that
// a model failure is surfaced, never masked.

// turnArticle is the pasted body every turn test reads. Two paragraphs → b1,
// b2 (SplitBlocks). The summon script below quotes b1 verbatim, which is what
// agent.ResolveExampleAnchor validates.
const turnArticle = "中国的太阳能装机量在过去十年增长了十倍。\n\n但同一时期，中国的碳排放总量仍居全球第一。"

func routerStubProvider(jsonOut string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: jsonOut},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 120, OutputTokens: 40}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// streamErrorProvider fails the call outright — the network/provider outage
// case, as opposed to a model that answers with garbage. Both must reach the
// student as the SAME honest 502.
type streamErrorProvider struct{}

func (streamErrorProvider) Stream(context.Context, gateway.Resolved, gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	return nil, errors.New("provider unreachable")
}

// turnHandler wires a lite API around a scripted router, creates a reading and
// pastes the article, and returns everything a turn test needs.
func turnHandler(t *testing.T, prov gateway.Provider) (http.Handler, *http.Cookie, string) {
	t.Helper()
	h, cookie, _, _ := liteHandlerWithProvider(t, prov)
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	return h, cookie, id
}

func postTurn(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/readings/"+id+"/turn", strings.NewReader(body)), cookie))
	return rec
}

type turnMessage struct {
	Seq     int    `json:"seq"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

func listMessages(t *testing.T, h http.Handler, cookie *http.Cookie, id string) []turnMessage {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/messages", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET messages = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Messages []turnMessage `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode messages: %v — body=%s", err, rec.Body)
	}
	return out.Messages
}

type turnReply struct {
	Reply    string `json:"reply"`
	Decision string `json:"decision"`
	Nudge    string `json:"nudge"`
	Card     *struct {
		ID      string  `json:"id"`
		CardID  string  `json:"cardId"`
		BlockID *string `json:"blockId"`
		Status  string  `json:"status"`
	} `json:"card"`
}

// TestReadingTurn_RespondPersistsBothMessages — the ordinary turn: the router
// answers in chat, and BOTH sides of the exchange land in atom_message with
// consecutive seq. The transcript is the lite edition's whole memory (there is
// no compaction layer), so a dropped or reordered row is a silent data loss.
func TestReadingTurn_RespondPersistsBothMessages(t *testing.T) {
	prov := routerStubProvider(`{"decision":"respond","reply":"这个十倍是跟哪一年比的？"}`)
	h, cookie, id := turnHandler(t, prov)

	rec := postTurn(t, h, cookie, id, `{"text":"我觉得这篇文章在夸中国"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out turnReply
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Decision != "respond" || out.Reply != "这个十倍是跟哪一年比的？" {
		t.Fatalf("reply = %+v, want the model's respond reply", out)
	}
	if out.Card != nil {
		t.Fatalf("respond must not carry a card: %+v", out.Card)
	}

	msgs := listMessages(t, h, cookie, id)
	if len(msgs) != 2 {
		t.Fatalf("messages = %d (%+v), want 2", len(msgs), msgs)
	}
	if msgs[0].Role != "student" || msgs[0].Seq != 1 || msgs[0].Content != "我觉得这篇文章在夸中国" {
		t.Fatalf("first message = %+v, want the student's turn at seq 1", msgs[0])
	}
	if msgs[1].Role != "ai" || msgs[1].Seq != 2 || msgs[1].Content != out.Reply {
		t.Fatalf("second message = %+v, want the ai reply at seq 2", msgs[1])
	}

	// A second turn continues the same thread rather than restarting it.
	if rec2 := postTurn(t, h, cookie, id, `{"text":"那储能呢"}`); rec2.Code != http.StatusOK {
		t.Fatalf("second turn = %d; body=%s", rec2.Code, rec2.Body)
	}
	msgs = listMessages(t, h, cookie, id)
	if len(msgs) != 4 || msgs[3].Seq != 4 {
		t.Fatalf("after two turns messages = %+v, want 4 with seq ending at 4", msgs)
	}
}

// TestReadingTurn_SummonCreatesProposedCard — the summon path: the router names
// a card and a verbatim example sentence, so a 'proposed' atom_card is minted
// against that block and returned with the nudge.
func TestReadingTurn_SummonCreatesProposedCard(t *testing.T) {
	script := `{"decision":"summon","card_id":"craap",` +
		`"reason":"要不要用 CRAAP 查一下这个十倍是谁统计的？",` +
		`"reply":"你抓到的这个数字确实是全篇的支点。",` +
		`"example_block_id":"b1",` +
		`"example_quote":"中国的太阳能装机量在过去十年增长了十倍。",` +
		`"example_why":"这句给了一个没有出处的数字。"}`
	h, cookie, id := turnHandler(t, routerStubProvider(script))

	rec := postTurn(t, h, cookie, id,
		`{"text":"这个十倍的数字是真的吗","focusedSpans":[{"blockId":"b1","quote":"增长了十倍"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out turnReply
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Decision != "summon" {
		t.Fatalf("decision = %q, want summon — %s", out.Decision, rec.Body)
	}
	if out.Card == nil {
		t.Fatalf("summon must carry the card it proposed — %s", rec.Body)
	}
	if out.Card.CardID != "craap" || out.Card.Status != "proposed" {
		t.Fatalf("card = %+v, want a proposed craap", out.Card)
	}
	if out.Card.BlockID == nil || *out.Card.BlockID != "b1" {
		t.Fatalf("card blockId = %v, want b1", out.Card.BlockID)
	}
	if out.Nudge == "" {
		t.Fatalf("summon must carry the nudge that invites her to open it — %s", rec.Body)
	}

	// The same card is visible through Task 6's list endpoint (one shape, one
	// truth — the turn does not mint a second card representation).
	recList := httptest.NewRecorder()
	h.ServeHTTP(recList, withCookie(httptest.NewRequest("GET", "/api/v1/readings/"+id+"/cards", nil), cookie))
	var cards struct {
		Cards []struct {
			ID     string `json:"id"`
			CardID string `json:"cardId"`
			Status string `json:"status"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &cards); err != nil {
		t.Fatalf("decode cards: %v — %s", err, recList.Body)
	}
	if len(cards.Cards) != 1 || cards.Cards[0].ID != out.Card.ID || cards.Cards[0].Status != "proposed" {
		t.Fatalf("cards = %+v, want exactly the card the turn returned", cards.Cards)
	}
}

// TestReadingTurn_ModelFailureSurfacesAsError — USER RULE: an AI-dialogue
// failure must reach the student as a real 502 ai_dialogue_failed. A canned
// stand-in reply would leave her talking to a dead turn without ever being
// told, which is exactly the bug this rule exists to prevent. Nothing fake is
// written to the transcript either.
func TestReadingTurn_ModelFailureSurfacesAsError(t *testing.T) {
	h, cookie, id := turnHandler(t, streamErrorProvider{})

	rec := postTurn(t, h, cookie, id, `{"text":"这篇在说什么"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("model failure = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "ai_dialogue_failed") {
		t.Fatalf("want ai_dialogue_failed in body, got %s", rec.Body)
	}
	if msgs := listMessages(t, h, cookie, id); len(msgs) != 0 {
		t.Fatalf("a failed turn must write nothing to the transcript, got %+v", msgs)
	}
}

// TestReadingTurn_UnparseableReplyAlsoSurfaces — the other half of the same
// rule: a model that ANSWERS but not in the JSON contract has also failed.
// agent.RouteReading degrades that to an empty-reply "respond"; the lite
// handler must not dress that up as a real answer.
func TestReadingTurn_UnparseableReplyAlsoSurfaces(t *testing.T) {
	h, cookie, id := turnHandler(t, routerStubProvider("这个问题挺好的，你先自己想想……"))

	rec := postTurn(t, h, cookie, id, `{"text":"这篇在说什么"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unparseable reply = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "ai_dialogue_failed") {
		t.Fatalf("want ai_dialogue_failed in body, got %s", rec.Body)
	}
}

// TestReadingTurn_MetersTheCallAgainstTheAtom — 硬约束: every real model call
// records 档位 + token + 成本. For lite that row carries surface='lite' and the
// atom id, with project_id NULL (a lite atom owns no project).
func TestReadingTurn_MetersTheCallAgainstTheAtom(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t,
		routerStubProvider(`{"decision":"respond","reply":"你说的是哪一句？"}`))
	id := createReadingAtom(t, h, cookie)
	if rec := putSource(t, h, cookie, id, turnArticle); rec.Code != http.StatusOK {
		t.Fatalf("put source = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := postTurn(t, h, cookie, id, `{"text":"我不太懂这段"}`); rec.Code != http.StatusOK {
		t.Fatalf("turn = %d; body=%s", rec.Code, rec.Body)
	}

	var surface, purpose, tier string
	var prompt, completion int32
	var projectNull bool
	if err := pool.QueryRow(t.Context(),
		`SELECT surface, purpose, tier, prompt_tokens, completion_tokens, project_id IS NULL
		   FROM llm_call WHERE atom_id = $1`, mustUUID(id)).
		Scan(&surface, &purpose, &tier, &prompt, &completion, &projectNull); err != nil {
		t.Fatalf("read llm_call: %v", err)
	}
	if surface != "lite" || purpose != "reading_turn" {
		t.Fatalf("llm_call = %q/%q, want lite/reading_turn", surface, purpose)
	}
	if tier != "flagship" || prompt != 120 || completion != 40 {
		t.Fatalf("llm_call tier/tokens = %q/%d/%d, want flagship/120/40", tier, prompt, completion)
	}
	if !projectNull {
		t.Fatalf("lite llm_call must leave project_id NULL")
	}
}

// TestReadingTurn_RequiresText — an empty turn has nothing to route on.
func TestReadingTurn_RequiresText(t *testing.T) {
	h, cookie, id := turnHandler(t, routerStubProvider(`{"decision":"respond","reply":"x"}`))
	if rec := postTurn(t, h, cookie, id, `{"text":"   "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank turn = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

// TestReadingTurn_RequiresSource — the router cannot read WITH her before the
// article exists; that is a 404 (same posture as GET /source). Asserted on the
// JSON error envelope, not the bare status, so an unregistered route (the
// mux's plain-text 404) cannot make this pass vacuously.
func TestReadingTurn_RequiresSource(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, routerStubProvider(`{"decision":"respond","reply":"x"}`))
	id := createReadingAtom(t, h, cookie)
	rec := postTurn(t, h, cookie, id, `{"text":"开始吧"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("turn before source = %d, want 404; body=%s", rec.Code, rec.Body)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code == "" {
		t.Fatalf("want the API's JSON 404 envelope, got %s", rec.Body)
	}
}
