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
