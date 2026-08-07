package api_test

// coach_orchestrator_test.go — Task 5 · POST /coach runs through the orchestrator
// agent loop. One turn whose stubbed model output emits set_status + open_tool +
// propose_note applies the directive (stage/openTool/widthTier), surfaces the note
// proposal, persists studio_state, and returns the OrchestratorReply wire shape.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// orchestratorStubProvider replays one orchestrator JSON turn (the model's whole
// output) with a non-zero usage so the metering path runs.
func orchestratorStubProvider(jsonOut string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: jsonOut},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 12}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func orchestratorHandler(t *testing.T, jsonOut string) (http.Handler, *http.Cookie, *pgxpool.Pool) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: orchestratorStubProvider(jsonOut), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool), pool
}

func TestPostCoach_AppliesOrchestratorDirective(t *testing.T) {
	out := `{"narrate":"写作面板开好了。","tools":[` +
		`{"name":"set_status","args":{"stage":"body_writing"}},` +
		`{"name":"open_tool","args":{"tool":"writing","reason":"该写正文"}},` +
		`{"name":"propose_note","args":{"section":"objective","value":"净影响"}}]}`
	h, cookie, pool := orchestratorHandler(t, out)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我准备好写了"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}

	var resp struct {
		Narrate   string `json:"narrate"`
		Directive struct {
			Stage     string `json:"stage"`
			OpenTool  string `json:"openTool"`
			WidthTier string `json:"widthTier"`
		} `json:"directive"`
		Note *struct {
			Section string `json:"section"`
			Value   string `json:"value"`
		} `json:"note"`
		Card            *json.RawMessage `json:"card"`
		ReviewRequested bool             `json:"reviewRequested"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if strings.TrimSpace(resp.Narrate) == "" {
		t.Fatalf("empty narrate — %s", rr.Body)
	}
	if resp.Directive.Stage != "body_writing" || resp.Directive.OpenTool != "writing" || resp.Directive.WidthTier != "wide" {
		t.Fatalf("directive not applied: %+v — %s", resp.Directive, rr.Body)
	}
	if resp.Note == nil || resp.Note.Section != "objective" || resp.Note.Value != "净影响" {
		t.Fatalf("note proposal wrong: %+v — %s", resp.Note, rr.Body)
	}

	// studio_state persisted with the advanced stage.
	raw, err := sqlc.New(pool).GetStudioState(context.Background(), mustUUID(seedProjectID))
	if err != nil {
		t.Fatalf("GetStudioState: %v", err)
	}
	var st agent.StudioState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal studio_state: %v — %s", err, raw)
	}
	if st.Stage != agent.StageBodyWriting {
		t.Fatalf("studio_state not persisted: %+v", st)
	}
	if st.OpenTool != agent.ToolWriting || st.WidthTier != agent.WidthWide {
		t.Fatalf("studio_state open tool/width not persisted: %+v", st)
	}
}

// TestPostCoach_SummonCardOffersCardAndRecordsEvent — the orchestrator's
// summon_card tool surfaces a card chip on the reply and records a coach_proposed
// event (the card-proposal path that replaced the retired classify chip).
// fact-opinion-value is proposable and not pre-seeded, so the in-flight guard
// (cardEligibleForSummon) admits it.
func TestPostCoach_SummonCardOffersCardAndRecordsEvent(t *testing.T) {
	out := `{"narrate":"这里适合停一下。","tools":[` +
		`{"name":"summon_card","args":{"card_id":"fact-opinion-value","reason":"事实与观点混在一起","nudge_text":"要不要用这张卡分一分？"}}]}`
	h, cookie, pool := orchestratorHandler(t, out)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我觉得中国显然让地球更可持续了，这就是事实"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		Card *struct {
			CardID    string `json:"cardId"`
			NudgeText string `json:"nudgeText"`
		} `json:"card"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if resp.Card == nil || resp.Card.CardID != "fact-opinion-value" {
		t.Fatalf("want fact-opinion-value card, got %+v — %s", resp.Card, rr.Body)
	}
	if resp.Card.NudgeText == "" {
		t.Fatalf("card must carry a nudge — %s", rr.Body)
	}
	if got := countCoachProposedEvents(t, pool, seedProjectID); got != 1 {
		t.Fatalf("coach_proposed events = %d, want 1", got)
	}
}

// TestPostCoach_SummonCardSkippedCardNotReoffered — 铁律 2 · 不操纵: once the
// student has dismissed a card (a `skipped` instance exists), the orchestrator's
// summon_card must NOT re-offer it. The card chip is suppressed and no new
// coach_proposed event is recorded. This restores the dismiss-suppression
// coverage the deleted classify test used to provide.
func TestPostCoach_SummonCardSkippedCardNotReoffered(t *testing.T) {
	out := `{"narrate":"这里适合停一下。","tools":[` +
		`{"name":"summon_card","args":{"card_id":"fact-opinion-value","reason":"事实与观点混在一起","nudge_text":"要不要用这张卡分一分？"}}]}`
	h, cookie, pool := orchestratorHandler(t, out)
	base := "/api/v1/projects/" + seedProjectID

	// The student dismissed this card earlier → a `skipped` instance exists.
	rrD := httptest.NewRecorder()
	h.ServeHTTP(rrD, withCookie(httptest.NewRequest("POST", base+"/cards/dismiss-proposal",
		strings.NewReader(`{"card_id":"fact-opinion-value"}`)), cookie))
	if rrD.Code != http.StatusNoContent {
		t.Fatalf("dismiss = %d, want 204 — %s", rrD.Code, rrD.Body)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"我觉得中国显然让地球更可持续了，这就是事实"}`)), cookie))
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
	if resp.Card != nil {
		t.Fatalf("dismissed card must NOT be re-offered, got %+v — %s", resp.Card, rr.Body)
	}
	// dismiss records a coach_proposal_skipped event, but the suppressed summon
	// must record NO coach_proposed event.
	if got := countCoachProposedEvents(t, pool, seedProjectID); got != 0 {
		t.Fatalf("coach_proposed events = %d, want 0 (suppressed)", got)
	}
}

// plainReplyStubProvider replays a plain conversational reply (NOT orchestrator
// JSON) — what the legacy ProposeProjectCoachReply producer expects, since it
// just takes the model's raw text. Returns the concrete *gateway.StubProvider
// (not wrapped in the gateway.Provider interface) so a test can inspect
// .LastRequest after the call — e.g. to assert the built prompt does NOT
// duplicate the student's current turn (fix round 1).
func plainReplyStubProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 20, OutputTokens: 8}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// TestPostCoach_SubagentScopeUsesLegacyPath — Task 9a: a coach turn tagged with
// a SUB-AGENT scope (find_sources / reflection) must NOT be driven by the
// studio orchestrator. It takes the retained legacy per-surface path instead:
// the stub model returns a PLAIN reply string (not orchestrator tool-call
// JSON) and the response still narrates it correctly, proving the legacy
// producer (not ProposeOrchestratorTurn) ran. The turn is stored under the
// sub-agent's own surface, and studio_state is left untouched.
func TestPostCoach_SubagentScopeUsesLegacyPath(t *testing.T) {
	const reply = "先说说你觉得这个来源可信在哪？"
	const studentText = "这篇文章看起来靠谱吗"
	pool := newAPITestPool(t)
	stub := plainReplyStubProvider(reply)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: stub, ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"user_input":"`+studentText+`","scope":"find_sources"}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rr.Code, rr.Body)
	}

	// (fix round 1) the built prompt must carry the student's current turn
	// EXACTLY ONCE. The bug this regressed: persisting the student turn to
	// Postgres BEFORE loading history meant LoadActiveCoachHistory read the
	// just-committed row back, and the handler then appended the SAME turn a
	// second time in memory — so the model saw it twice back-to-back. Inspect
	// the captured provider request (ProposeProjectCoachReply's user message
	// embeds the whole history as "- 学生：<text>" lines via
	// BuildProjectCoachContext) rather than the reply text, since the stub's
	// reply is fixed regardless of prompt content.
	if got := len(stub.LastRequest.Messages); got != 2 {
		t.Fatalf("provider request messages = %d, want 2 (system+user) — %+v", got, stub.LastRequest.Messages)
	}
	userMsg := stub.LastRequest.Messages[1]
	if userMsg.Role != gateway.RoleUser {
		t.Fatalf("messages[1].role = %q, want %q — %+v", userMsg.Role, gateway.RoleUser, stub.LastRequest.Messages)
	}
	if n := strings.Count(userMsg.Content, studentText); n != 1 {
		t.Fatalf("student turn %q appears %d times in the built prompt, want exactly 1 — prompt:\n%s", studentText, n, userMsg.Content)
	}

	var resp struct {
		Narrate   string `json:"narrate"`
		Directive struct {
			Stage string `json:"stage"`
		} `json:"directive"`
		Note            *json.RawMessage `json:"note"`
		Card            *json.RawMessage `json:"card"`
		ReviewRequested bool             `json:"reviewRequested"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}

	// (a) narrate is the stub's plain reply — proves the LEGACY producer ran
	// (an orchestrator JSON blob would never come back verbatim as narrate).
	if resp.Narrate != reply {
		t.Fatalf("narrate = %q, want %q — %s", resp.Narrate, reply, rr.Body)
	}
	// (b) directive is the unchanged default studio_state.
	if resp.Directive.Stage != string(agent.StageTopicDiscussion) {
		t.Fatalf("directive.stage = %q, want default %q — %s", resp.Directive.Stage, agent.StageTopicDiscussion, rr.Body)
	}
	if resp.Note != nil || resp.Card != nil || resp.ReviewRequested {
		t.Fatalf("sub-agent reply must never carry note/card/reviewRequested — %s", rr.Body)
	}

	// (c) the turn was stored under surface "find_sources", not "studio".
	surface := "find_sources"
	rows, err := sqlc.New(pool).ListChatMessagesByProjectSurface(context.Background(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: mustUUID(seedProjectID), Valid: true},
		Surface:         &surface,
	})
	if err != nil {
		t.Fatalf("ListChatMessagesByProjectSurface: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("find_sources surface turns = %d, want 2 (student+assistant) — %+v", len(rows), rows)
	}
	var sawUser, sawAssistant bool
	for _, m := range rows {
		if m.Role == "user" && m.Content == "这篇文章看起来靠谱吗" {
			sawUser = true
		}
		if m.Role == "assistant" && m.Content == reply {
			sawAssistant = true
		}
	}
	if !sawUser || !sawAssistant {
		t.Fatalf("find_sources surface missing expected turns: %+v", rows)
	}

	// studio surface must stay untouched by this sub-agent turn.
	studioSurface := "studio"
	studioRows, err := sqlc.New(pool).ListChatMessagesByProjectSurface(context.Background(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: mustUUID(seedProjectID), Valid: true},
		Surface:         &studioSurface,
	})
	if err != nil {
		t.Fatalf("ListChatMessagesByProjectSurface(studio): %v", err)
	}
	if len(studioRows) != 0 {
		t.Fatalf("studio surface must stay empty, got %d rows — %+v", len(studioRows), studioRows)
	}

	// (d) studio_state was NOT changed: the `project` table's studio_state column
	// carries a DB-level default (migration 0057), so it's always readable — a
	// sub-agent turn must never call SetStudioState, so it stays exactly that
	// untouched default (stage topic_discussion, updatedAtTurn 0).
	raw, gerr := sqlc.New(pool).GetStudioState(context.Background(), mustUUID(seedProjectID))
	if gerr != nil {
		t.Fatalf("GetStudioState: %v", gerr)
	}
	var st agent.StudioState
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatalf("unmarshal studio_state: %v — %s", err, raw)
	}
	if def := agent.DefaultStudioState(); st.Stage != def.Stage || st.UpdatedAtTurn != def.UpdatedAtTurn {
		t.Fatalf("studio_state was mutated by a sub-agent turn: %+v (default %+v)", st, def)
	}
}
