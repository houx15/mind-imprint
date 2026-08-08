package api_test

// coach_onboarding_test.go — Task 3 (studio onboarding): the two entry points
// that bracket the studio orchestrator's first turn — a real-AI opening
// welcome (POST /coach/opening, tool-less) and the explicit "start" gate
// (POST /coach/start, tool-aware, opens 提案/forming) — mirroring the
// idempotency and directive assertions coach_orchestrator_test.go already
// established for the always-on /coach path.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// coachOnboardingReplyDTO mirrors orchestratorReplyDTO's wire shape for these
// tests' assertions (directive.started/openTool are the load-bearing bits).
type coachOnboardingReplyDTO struct {
	Narrate   string `json:"narrate"`
	Directive struct {
		Stage     string `json:"stage"`
		OpenTool  string `json:"openTool"`
		WidthTier string `json:"widthTier"`
		Started   bool   `json:"started"`
	} `json:"directive"`
	PlanGenerated bool `json:"planGenerated"`
	Compacted     bool `json:"compacted"`
}

// TestCoachOpening — the real-AI opening welcome lands as the studio thread's
// sole (assistant-only) first turn, forces a chat-only/not-started directive,
// and is idempotent: a second call spends nothing and returns an empty
// narrate once the thread is non-empty.
func TestCoachOpening(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach/opening", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach/opening = %d — %s", rr.Code, rr.Body)
	}
	var resp coachOnboardingReplyDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if strings.TrimSpace(resp.Narrate) == "" {
		t.Fatalf("expected non-empty narrate on the first opening — %s", rr.Body)
	}
	if resp.Directive.Started {
		t.Fatalf("directive.started = true, want false (opening never starts the project) — %s", rr.Body)
	}
	if resp.Directive.OpenTool != "chat" {
		t.Fatalf("directive.openTool = %q, want %q — %s", resp.Directive.OpenTool, "chat", rr.Body)
	}

	// Exactly one assistant studio chat_message — and NO student turn.
	surface := "studio"
	rows, err := sqlc.New(pool).ListChatMessagesByProjectSurface(context.Background(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: mustUUID(seedProjectID), Valid: true}, Surface: &surface,
	})
	if err != nil {
		t.Fatalf("ListChatMessagesByProjectSurface: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("studio surface turns after opening = %d, want 1 — %+v", len(rows), rows)
	}
	if rows[0].Role != "assistant" {
		t.Fatalf("the one opening turn's role = %q, want assistant (no student turn) — %+v", rows[0].Role, rows[0])
	}

	// Idempotent: a second opening call must spend nothing and add no turn.
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("POST", base+"/coach/opening", nil), cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("second coach/opening = %d — %s", rr2.Code, rr2.Body)
	}
	var resp2 coachOnboardingReplyDTO
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("decode 2nd: %v — %s", err, rr2.Body)
	}
	if resp2.Narrate != "" {
		t.Fatalf("second opening narrate = %q, want empty (idempotent, no spend) — %s", resp2.Narrate, rr2.Body)
	}
	rows2, err := sqlc.New(pool).ListChatMessagesByProjectSurface(context.Background(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: mustUUID(seedProjectID), Valid: true}, Surface: &surface,
	})
	if err != nil {
		t.Fatalf("ListChatMessagesByProjectSurface (2nd): %v", err)
	}
	if len(rows2) != 1 {
		t.Fatalf("studio surface turns after 2nd opening = %d, want still 1 (no new turn)", len(rows2))
	}
}

// TestCoachStart — the explicit start gate opens 提案 (forming), persists a
// synthetic student turn standing in for the button press, runs a real
// tool-aware orchestrator turn, and is idempotent once started=true.
func TestCoachStart(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	// Opening happens first in the real flow — plain-text provider is enough
	// (ProposeOpeningTurn is tool-less, it never parses the reply as JSON).
	openH := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	rrOpen := httptest.NewRecorder()
	openH.ServeHTTP(rrOpen, withCookie(httptest.NewRequest("POST", base+"/coach/opening", nil), cookie))
	if rrOpen.Code != http.StatusOK {
		t.Fatalf("coach/opening = %d — %s", rrOpen.Code, rrOpen.Body)
	}

	// Start runs the tool-aware ProposeOrchestratorTurn path — needs a
	// provider that replies with valid orchestrator JSON, unlike opening's.
	startOut := `{"narrate":"我们先想目标","tools":[]}`
	startH := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: orchestratorStubProvider(startOut), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()

	rr := httptest.NewRecorder()
	startH.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach/start", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("coach/start = %d — %s", rr.Code, rr.Body)
	}
	var resp coachOnboardingReplyDTO
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if !resp.Directive.Started {
		t.Fatalf("directive.started = false, want true — %s", rr.Body)
	}
	if resp.Directive.OpenTool != "forming" {
		t.Fatalf("directive.openTool = %q, want %q — %s", resp.Directive.OpenTool, "forming", rr.Body)
	}
	if strings.TrimSpace(resp.Narrate) == "" {
		t.Fatalf("expected non-empty narrate on start — %s", rr.Body)
	}

	// GET /studio-state reflects started=true durably.
	rrState := httptest.NewRecorder()
	startH.ServeHTTP(rrState, withCookie(httptest.NewRequest("GET", base+"/studio-state", nil), cookie))
	if rrState.Code != http.StatusOK {
		t.Fatalf("studio-state = %d — %s", rrState.Code, rrState.Body)
	}
	var state struct {
		Started  bool   `json:"started"`
		OpenTool string `json:"openTool"`
	}
	if err := json.Unmarshal(rrState.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode studio-state: %v — %s", err, rrState.Body)
	}
	if !state.Started {
		t.Fatalf("studio-state.started = false, want true — %s", rrState.Body)
	}

	// The synthetic student turn "我准备好了，开始吧" was persisted (studio
	// surface now carries: opening's assistant turn + start's user+assistant).
	surface := "studio"
	rows, err := sqlc.New(pool).ListChatMessagesByProjectSurface(context.Background(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: mustUUID(seedProjectID), Valid: true}, Surface: &surface,
	})
	if err != nil {
		t.Fatalf("ListChatMessagesByProjectSurface: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("studio surface turns after start = %d, want 3 (opening-assistant + start-user + start-assistant) — %+v", len(rows), rows)
	}
	var sawSynthetic bool
	for _, m := range rows {
		if m.Role == "user" && m.Content == "我准备好了，开始吧" {
			sawSynthetic = true
		}
	}
	if !sawSynthetic {
		t.Fatalf("synthetic student turn not found among studio turns: %+v", rows)
	}

	// Idempotent: a second start call must not re-run the orchestrator or add
	// another synthetic turn.
	rr2 := httptest.NewRecorder()
	startH.ServeHTTP(rr2, withCookie(httptest.NewRequest("POST", base+"/coach/start", nil), cookie))
	if rr2.Code != http.StatusOK {
		t.Fatalf("second coach/start = %d — %s", rr2.Code, rr2.Body)
	}
	var resp2 coachOnboardingReplyDTO
	if err := json.Unmarshal(rr2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("decode 2nd: %v — %s", err, rr2.Body)
	}
	if resp2.Narrate != "" {
		t.Fatalf("second start narrate = %q, want empty (idempotent, no spend) — %s", resp2.Narrate, rr2.Body)
	}
	rows2, err := sqlc.New(pool).ListChatMessagesByProjectSurface(context.Background(), sqlc.ListChatMessagesByProjectSurfaceParams{
		SeededProjectID: pgtype.UUID{Bytes: mustUUID(seedProjectID), Valid: true}, Surface: &surface,
	})
	if err != nil {
		t.Fatalf("ListChatMessagesByProjectSurface (2nd): %v", err)
	}
	if len(rows2) != 3 {
		t.Fatalf("studio surface turns after 2nd start = %d, want still 3 (no new turn)", len(rows2))
	}
}
