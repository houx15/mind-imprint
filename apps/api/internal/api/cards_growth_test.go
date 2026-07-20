package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestGetGrowthCards_EmptyStateNoModelCall — a fresh student with no completed
// cards gets a 200 {"cards":[]}, and NO llm_call row is written (pure read).
func TestGetGrowthCards_EmptyStateNoModelCall(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	// A truly fresh student — NOT signInSeed, whose user has seeded completed
	// cards from migration 0018 (see the note above).
	student := createStudent(t, pool, SeedSchoolID, "cards-empty@demo.local")
	cookie := signInAs(t, pool, student)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/growth/cards", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /growth/cards = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var body struct {
		Cards []struct {
			CardID   string   `json:"cardId"`
			Uses     int      `json:"uses"`
			Surfaces []string `json:"surfaces"`
			LastUsed string   `json:"lastUsed"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cards: %v — body=%s", err, rec.Body)
	}
	if len(body.Cards) != 0 {
		t.Fatalf("fresh student cards = %d, want 0", len(body.Cards))
	}
	if n := countAllLLMCalls(t, pool); n != 0 {
		t.Fatalf("llm_call rows = %d, want 0 (cards is a pure read)", n)
	}
}
