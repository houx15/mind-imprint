package api_test

// ai_use_test.go — S5 · the AI-interaction retrospective endpoints. Draft
// assembles the objective record + seeds the two authored fields (metered mid-
// tier, only when there's a record and nothing saved); a saved statement is
// returned verbatim with no new spend; POST persists (no spend) + logs.

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

// aiUseSeedProvider returns valid seed JSON (also the coach reply when it serves
// a coach turn — an ugly-but-valid 200 that creates the coach_turn event/llm_call
// the record needs).
func aiUseSeedProvider() gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"used_for":"用 AI 澄清检索词与核对来源功能","not_used_for":"没有让 AI 代写正文或预测分数"}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func aiUseHandler(t *testing.T) (http.Handler, *http.Cookie, *pgxpool.Pool) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: aiUseSeedProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool), pool
}

func TestGetAIUseDraft_SeedsFromRecordAndMeters(t *testing.T) {
	h, cookie, pool := aiUseHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	// A coach turn gives the project an interaction record (coach_turn + coach call).
	rrC := httptest.NewRecorder()
	h.ServeHTTP(rrC, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"scope":"writing","user_input":"我想聊聊这段论证怎么收尾"}`)), cookie))
	if rrC.Code != http.StatusOK {
		t.Fatalf("coach = %d — %s", rrC.Code, rrC.Body)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/ai-use-draft", nil), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("ai-use-draft = %d — %s", rr.Code, rr.Body)
	}
	var d struct {
		Record struct {
			CoachTurns      int  `json:"coachTurns"`
			GhostwroteEssay bool `json:"ghostwroteEssay"`
		} `json:"record"`
		Draft struct {
			UsedFor    string `json:"usedFor"`
			NotUsedFor string `json:"notUsedFor"`
		} `json:"draft"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if d.Record.CoachTurns < 1 {
		t.Fatalf("record.coachTurns = %d, want >= 1", d.Record.CoachTurns)
	}
	if d.Record.GhostwroteEssay {
		t.Fatalf("ghostwroteEssay must be false")
	}
	if d.Draft.UsedFor == "" || d.Draft.NotUsedFor == "" {
		t.Fatalf("expected a seeded draft, got %+v", d.Draft)
	}
	if got := countLLMCallsByPurpose(t, pool, seedProjectID, "ai_use_retrospective"); got != 1 {
		t.Fatalf("ai_use_retrospective calls = %d, want 1", got)
	}
}

// TestGetAIUseDraft_CountsAcceptedCoachCard — the objective record must count a
// coach-proposed card the student accepted via the REAL persist path (which
// emits card_logged, NOT card_activated). Regression for the whole-branch
// CRITICAL: card_activated-only counting reported accepted cards as 0.
func TestGetAIUseDraft_CountsAcceptedCoachCard(t *testing.T) {
	h, cookie, _ := aiUseHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	rrP := httptest.NewRecorder()
	h.ServeHTTP(rrP, withCookie(httptest.NewRequest("POST", base+"/cards/persist",
		strings.NewReader(`{"card_id":"certainty-spectrum","field_values":{"claim":"有限肯定"},"event_trace":[]}`)), cookie))
	if rrP.Code != http.StatusOK {
		t.Fatalf("persist = %d — %s", rrP.Code, rrP.Body)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/ai-use-draft", nil), cookie))
	var d struct {
		Record struct {
			CardsAccepted int `json:"cardsAccepted"`
		} `json:"record"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body)
	}
	if d.Record.CardsAccepted < 1 {
		t.Fatalf("an accepted coach card (card_logged) must count, got cardsAccepted=%d", d.Record.CardsAccepted)
	}
}

func TestGetAIUseDraft_ReturnsSavedStatementNoSpend(t *testing.T) {
	h, cookie, pool := aiUseHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	rrP := httptest.NewRecorder()
	h.ServeHTTP(rrP, withCookie(httptest.NewRequest("POST", base+"/ai-use",
		strings.NewReader(`{"usedFor":"我自己写的自述","notUsedFor":"没让AI代写"}`)), cookie))
	if rrP.Code != http.StatusOK {
		t.Fatalf("post ai-use = %d — %s", rrP.Code, rrP.Body)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", base+"/ai-use-draft", nil), cookie))
	if !strings.Contains(rr.Body.String(), "我自己写的自述") {
		t.Fatalf("draft should return the saved statement: %s", rr.Body)
	}
	if got := countLLMCallsByPurpose(t, pool, seedProjectID, "ai_use_retrospective"); got != 0 {
		t.Fatalf("saved statement must not seed (0 spend), got %d", got)
	}
}

func TestPostAIUse_PersistsAndLogsNoSpend(t *testing.T) {
	h, cookie, pool := aiUseHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	rrP := httptest.NewRecorder()
	h.ServeHTTP(rrP, withCookie(httptest.NewRequest("POST", base+"/ai-use",
		strings.NewReader(`{"usedFor":"溯源提问","notUsedFor":"代写正文、预测分数"}`)), cookie))
	if rrP.Code != http.StatusOK {
		t.Fatalf("post ai-use = %d — %s", rrP.Code, rrP.Body)
	}
	rrG := httptest.NewRecorder()
	h.ServeHTTP(rrG, withCookie(httptest.NewRequest("GET", base+"/ai-use", nil), cookie))
	if !strings.Contains(rrG.Body.String(), "溯源提问") {
		t.Fatalf("get ai-use should return the saved statement: %s", rrG.Body)
	}
	if got := countEventsByType(t, pool, seedProjectID, "ai_use_written"); got != 1 {
		t.Fatalf("ai_use_written events = %d, want 1", got)
	}
	if got := countLLMCallsByPurpose(t, pool, seedProjectID, "ai_use_retrospective"); got != 0 {
		t.Fatalf("POST must not spend, got %d ai_use_retrospective calls", got)
	}
}

func TestPostAIUse_NonOwnedIs404(t *testing.T) {
	h, cookie, _ := aiUseHandler(t)
	base := "/api/v1/projects/00000000-0000-0000-0000-0000000009ff"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/ai-use",
		strings.NewReader(`{"usedFor":"x","notUsedFor":"y"}`)), cookie))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("non-owned = %d, want 404 — %s", rr.Code, rr.Body)
	}
}
