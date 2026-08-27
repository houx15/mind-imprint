package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// writing_turn_test.go — Task 4: the writing room's 陪练一轮, reusing
// agent.ProposeProjectCoachReply (see writing_turn.go's file comment for the
// reuse finding). These tests mirror reading_turn_test.go's shape: assemble /
// persist / meter, the 502-never-a-canned-reply rule, and the explicit
// RecentTurns window — the same hard requirements P1 Task 7 held reading to.

// writingTextStubProvider scripts a PLAIN TEXT reply — unlike reading's
// router, agent.ProposeProjectCoachReply parses no JSON contract out of the
// model's output; the raw text (trimmed) IS the reply.
func writingTextStubProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 90, OutputTokens: 30}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// writingStreamErrorProvider fails the call outright — the network/provider
// outage case. Must reach the student as the same honest 502 a bad completion
// does.
type writingStreamErrorProvider struct{}

func (writingStreamErrorProvider) Stream(context.Context, gateway.Resolved, gateway.ChatRequest) (<-chan gateway.StreamEvent, error) {
	return nil, errors.New("provider unreachable")
}

func postWritingTurn(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/writings/"+id+"/turn", strings.NewReader(body)), cookie))
	return rec
}

type writingTurnMessage struct {
	Seq     int    `json:"seq"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

func listWritingMessages(t *testing.T, h http.Handler, cookie *http.Cookie, id string) []writingTurnMessage {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/messages", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET messages = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Messages []writingTurnMessage `json:"messages"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode messages: %v — body=%s", err, rec.Body)
	}
	return out.Messages
}

type writingTurnReply struct {
	Reply      string  `json:"reply"`
	Decision   string  `json:"decision"`
	Nudge      string  `json:"nudge"`
	HintCardID *string `json:"hintCardId"`
	Card       *struct {
		ID string `json:"id"`
	} `json:"card"`
}

// TestWritingTurn_RespondPersistsBothMessages — the ordinary turn: the coach
// answers, and BOTH sides of the exchange land in atom_message with
// consecutive seq, continuing on top of createWriting's own seq=1 idea
// message (writings.go). The transcript is lite's whole memory (no
// compaction layer), so a dropped or reordered row is silent data loss.
func TestWritingTurn_RespondPersistsBothMessages(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("你想先从哪个角度切入？"))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	before := listWritingMessages(t, h, cookie, id)
	if len(before) != 1 || before[0].Role != "student" || before[0].Seq != 1 {
		t.Fatalf("baseline messages = %+v, want the idea alone at seq 1", before)
	}

	rec := postWritingTurn(t, h, cookie, id, `{"text":"我想写中国的新能源政策"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("turn = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out writingTurnReply
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v — body=%s", err, rec.Body)
	}
	if out.Decision != "respond" || out.Reply != "你想先从哪个角度切入？" {
		t.Fatalf("reply = %+v, want the model's reply verbatim", out)
	}
	if out.Card != nil {
		t.Fatalf("writing mounts no card-summon door yet — got %+v", out.Card)
	}
	if out.HintCardID != nil {
		t.Fatalf("writing mounts no card-summon door yet — got hintCardId %v", *out.HintCardID)
	}

	msgs := listWritingMessages(t, h, cookie, id)
	if len(msgs) != 3 {
		t.Fatalf("messages = %d (%+v), want 3 (idea + student + ai)", len(msgs), msgs)
	}
	if msgs[1].Role != "student" || msgs[1].Seq != 2 || msgs[1].Content != "我想写中国的新能源政策" {
		t.Fatalf("second message = %+v, want the student's turn at seq 2", msgs[1])
	}
	if msgs[2].Role != "ai" || msgs[2].Seq != 3 || msgs[2].Content != out.Reply {
		t.Fatalf("third message = %+v, want the ai reply at seq 3", msgs[2])
	}

	// A second turn continues the same thread rather than restarting it.
	if rec2 := postWritingTurn(t, h, cookie, id, `{"text":"具体写光伏产业"}`); rec2.Code != http.StatusOK {
		t.Fatalf("second turn = %d; body=%s", rec2.Code, rec2.Body)
	}
	msgs = listWritingMessages(t, h, cookie, id)
	if len(msgs) != 5 || msgs[4].Seq != 5 {
		t.Fatalf("after two turns messages = %+v, want 5 with seq ending at 5", msgs)
	}
}

// TestWritingTurn_ModelFailureSurfacesAsError — USER RULE: an AI-dialogue
// failure must reach the student as a real 502 ai_dialogue_failed, never a
// canned stand-in reply, and nothing fake lands in the transcript.
func TestWritingTurn_ModelFailureSurfacesAsError(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingStreamErrorProvider{})
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")
	before := listWritingMessages(t, h, cookie, id)

	rec := postWritingTurn(t, h, cookie, id, `{"text":"我该怎么开头"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("model failure = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "ai_dialogue_failed") {
		t.Fatalf("want ai_dialogue_failed in body, got %s", rec.Body)
	}
	after := listWritingMessages(t, h, cookie, id)
	if len(after) != len(before) {
		t.Fatalf("a failed turn must write nothing to the transcript: before=%+v after=%+v", before, after)
	}
}

// TestWritingTurn_EmptyReplyAlsoSurfaces — the other half of the same rule.
// agent.ProposeProjectCoachReply runs enforcement.ValidateOutput, which
// rejects an empty completion as a genuine error (unlike agent.RouteReading,
// this producer never dresses a bad reply up as a degraded-but-200 decision)
// — that error must still reach the student as 502, never a fallback
// sentence.
func TestWritingTurn_EmptyReplyAlsoSurfaces(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("   "))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	rec := postWritingTurn(t, h, cookie, id, `{"text":"我该怎么开头"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("empty reply = %d, want 502; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "ai_dialogue_failed") {
		t.Fatalf("want ai_dialogue_failed in body, got %s", rec.Body)
	}
}

// TestWritingTurn_MetersTheCallAgainstTheAtom — 硬约束: every real model call
// records 档位 + token + 成本. For lite that row carries surface='lite',
// purpose='writing_turn' and the atom id, with project_id NULL (a lite atom
// owns no project).
func TestWritingTurn_MetersTheCallAgainstTheAtom(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, writingTextStubProvider("你的论点是什么？"))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := postWritingTurn(t, h, cookie, id, `{"text":"还没想好"}`); rec.Code != http.StatusOK {
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
	if surface != "lite" || purpose != "writing_turn" {
		t.Fatalf("llm_call = %q/%q, want lite/writing_turn", surface, purpose)
	}
	if tier != "chaperone" || prompt != 90 || completion != 30 {
		t.Fatalf("llm_call tier/tokens = %q/%d/%d, want chaperone/90/30", tier, prompt, completion)
	}
	if !projectNull {
		t.Fatalf("lite llm_call must leave project_id NULL")
	}
}

// TestWritingTurn_RequiresText — an empty turn has nothing to talk about.
func TestWritingTurn_RequiresText(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")
	if rec := postWritingTurn(t, h, cookie, id, `{"text":"   "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank turn = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}

// TestWritingTurn_HistoryIsWindowed — RecentTurns is windowed via a NAMED
// CONSTANT (writingTurnsWindow, writing_turn.go), never the whole thread:
// lite has no compaction layer, so this window is the only bound on prompt
// growth. Twelve completed turns are posted, then a probe turn's OUTGOING
// prompt (captured on gateway.StubProvider.LastRequest) is asserted to still
// carry the most recent turn but to have already dropped the earliest ones
// from the CONVERSATION section — distinct from the always-on state
// projection (buildWritingCoachProjection), which legitimately keeps the
// title regardless of the window: that is a fact about the writing, not a
// turn either side "said", so it is asserted separately and is NOT expected
// to disappear.
func TestWritingTurn_HistoryIsWindowed(t *testing.T) {
	stub := writingTextStubProvider("好的，继续说。")
	h, cookie, _, _ := liteHandlerWithProvider(t, stub)
	id := createWritingAtomHTTP(t, h, cookie, "这是最初的写作想法-独特标记")

	const turns = 12
	for i := 1; i <= turns; i++ {
		body := fmt.Sprintf(`{"text":"第%d轮学生发言-独特标记"}`, i)
		if rec := postWritingTurn(t, h, cookie, id, body); rec.Code != http.StatusOK {
			t.Fatalf("turn %d = %d; body=%s", i, rec.Code, rec.Body)
		}
	}

	// One more probe turn — its OUTGOING request is what the window bounds.
	if rec := postWritingTurn(t, h, cookie, id, `{"text":"探测轮"}`); rec.Code != http.StatusOK {
		t.Fatalf("probe turn = %d; body=%s", rec.Code, rec.Body)
	}
	if len(stub.LastRequest.Messages) != 2 {
		t.Fatalf("provider request messages = %d, want 2 (system+user) — %+v", len(stub.LastRequest.Messages), stub.LastRequest.Messages)
	}
	sent := stub.LastRequest.Messages[1].Content
	// The title is a standing fact (buildWritingCoachProjection), not a
	// windowed turn — it is expected to survive every turn, including this
	// one, and asserting it were gone would be testing the wrong thing.
	if !strings.Contains(sent, "最初的写作想法") {
		t.Fatalf("the standing title projection should never be windowed out: %s", sent)
	}
	if strings.Contains(sent, "第1轮学生发言") {
		t.Fatalf("windowed prompt still carries turn 1's conversation line, want it dropped by writingTurnsWindow: %s", sent)
	}
	if !strings.Contains(sent, "第12轮学生发言") {
		t.Fatalf("windowed prompt dropped the most recent completed turn, want it kept: %s", sent)
	}
	if !strings.Contains(sent, "探测轮") {
		t.Fatalf("windowed prompt is missing the current turn itself: %s", sent)
	}
}

// TestWritingTurn_RequiresOwnWriting — cross-kind isolation: a reading atom's
// id at the writing turn route is a flat 404 (never leaked as "not a
// writing"), mirroring loadOwnedAtom's kind check (Task 1.5).
func TestWritingTurn_RequiresOwnWriting(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, writingTextStubProvider("x"))
	bogus := "00000000-0000-0000-0000-000000000000"
	rec := postWritingTurn(t, h, cookie, bogus, `{"text":"开始吧"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("turn on nonexistent writing = %d, want 404; body=%s", rec.Code, rec.Body)
	}
}
