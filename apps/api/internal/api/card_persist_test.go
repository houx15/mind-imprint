package api_test

// card_persist_test.go — S4 · the generic persist path for a proposed card the
// student opened and filled. Only allowlisted (proposable) card ids persist; a
// non-proposable id is rejected so the endpoint can't forge arbitrary state.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func persistHandler(t *testing.T) (http.Handler, *http.Cookie, *pgxpool.Pool) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool), pool
}

func countCompletedCard(t *testing.T, pool *pgxpool.Pool, projectID, cardID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM card_instances WHERE project_id=$1 AND card_id=$2 AND status='completed'`,
		mustUUID(projectID), cardID).Scan(&n); err != nil {
		t.Fatalf("countCompletedCard: %v", err)
	}
	return n
}

func TestPostPersistProjectCard_PersistsProposable(t *testing.T) {
	h, cookie, pool := persistHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	// fact-opinion-value is proposable and NOT pre-seeded (0018 seeds steelman + concession).
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/cards/persist",
		strings.NewReader(`{"card_id":"fact-opinion-value","field_values":{"claim":"中国让地球更可持续","kind":"观点"},"event_trace":[]}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("persist = %d — %s", rr.Code, rr.Body)
	}
	if got := countCompletedCard(t, pool, seedProjectID, "fact-opinion-value"); got != 1 {
		t.Fatalf("completed fact-opinion-value = %d, want 1", got)
	}
	if got := countEventsByType(t, pool, seedProjectID, "card_logged"); got != 1 {
		t.Fatalf("card_logged events = %d, want 1", got)
	}
}

func countSkippedCard(t *testing.T, pool *pgxpool.Pool, projectID, cardID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM card_instances WHERE project_id=$1 AND card_id=$2 AND status='skipped'`,
		mustUUID(projectID), cardID).Scan(&n); err != nil {
		t.Fatalf("countSkippedCard: %v", err)
	}
	return n
}

// TestPostDismissProposal_RecordsSkipAndStopsReoffer — dismissing a proposal
// marks the card skipped, so a subsequent qualifying coach turn no longer offers
// it (铁律 · 不操纵 — once she says no, we don't ask again).
func TestPostDismissProposal_RecordsSkipAndStopsReoffer(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: momentReplyProvider("fact_opinion"), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/" + seedProjectID

	// First writing turn → a fact-opinion-value proposal.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"scope":"writing","user_input":"我觉得中国显然让地球更可持续了，这就是事实"}`)), cookie))
	if !strings.Contains(rr.Body.String(), "fact-opinion-value") {
		t.Fatalf("expected first turn to propose fact-opinion-value: %s", rr.Body)
	}

	// Dismiss it.
	rrD := httptest.NewRecorder()
	h.ServeHTTP(rrD, withCookie(httptest.NewRequest("POST", base+"/cards/dismiss-proposal",
		strings.NewReader(`{"card_id":"fact-opinion-value"}`)), cookie))
	if rrD.Code != http.StatusNoContent {
		t.Fatalf("dismiss = %d, want 204 — %s", rrD.Code, rrD.Body)
	}
	if got := countSkippedCard(t, pool, seedProjectID, "fact-opinion-value"); got != 1 {
		t.Fatalf("skipped fact-opinion-value = %d, want 1", got)
	}

	// A second qualifying writing turn must NOT re-offer fact-opinion-value.
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, withCookie(httptest.NewRequest("POST", base+"/coach",
		strings.NewReader(`{"scope":"writing","user_input":"我还是觉得中国显然让地球更可持续了，这就是事实"}`)), cookie))
	if strings.Contains(rr2.Body.String(), "fact-opinion-value") {
		t.Fatalf("dismissed card must not be re-offered: %s", rr2.Body)
	}
}

func TestPostPersistProjectCard_RejectsNonPersistable(t *testing.T) {
	h, cookie, pool := persistHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	// craap is a reading card in neither allowlist (not AI-proposable, not in the
	// writing deck) → must be refused.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/cards/persist",
		strings.NewReader(`{"card_id":"craap","field_values":{"x":"y"},"event_trace":[]}`)), cookie))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("non-persistable card = %d, want 400 — %s", rr.Code, rr.Body)
	}
	if got := countCompletedCard(t, pool, seedProjectID, "craap"); got != 0 {
		t.Fatalf("non-persistable must persist nothing, got %d", got)
	}
}

// WC · a student-summoned writing-deck card persists through the same path.
func TestPostPersistProjectCard_PersistsWritingDeckCard(t *testing.T) {
	h, cookie, pool := persistHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	// toulmin is the ONLY deck card with completion + graph_effects — the persist
	// path must NOT invoke CompleteCard (nil material would otherwise error), so
	// it exercises the graph-card risk the other deck cards can't. Not pre-seeded.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/cards/persist",
		strings.NewReader(`{"card_id":"toulmin","field_values":{"claim":"qualified yes","grounds":"greening 强、排放限制强"},"event_trace":[]}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("writing-deck (toulmin, graph card) persist = %d — %s", rr.Code, rr.Body)
	}
	if got := countCompletedCard(t, pool, seedProjectID, "toulmin"); got != 1 {
		t.Fatalf("completed toulmin = %d, want 1", got)
	}
}

// A writing-deck-only card (not AI-proposable) must NOT be dismissable via the
// proposal-dismiss path — dismiss stays scoped to coachProposableCards, so a
// future widening to persistableCard would fail this guard.
func TestPostDismissProposal_RejectsWritingDeckOnlyCard(t *testing.T) {
	h, cookie, _ := persistHandler(t)
	base := "/api/v1/projects/" + seedProjectID
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/cards/dismiss-proposal",
		strings.NewReader(`{"card_id":"toulmin"}`)), cookie))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("dismiss of a writing-deck-only card = %d, want 400 — %s", rr.Code, rr.Body)
	}
}
