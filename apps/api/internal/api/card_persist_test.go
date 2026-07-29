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

	// certainty-spectrum is proposable and NOT pre-seeded (0018 seeds steelman).
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/cards/persist",
		strings.NewReader(`{"card_id":"certainty-spectrum","field_values":{"claim":"中国让地球更可持续","certainty":"有限肯定"},"event_trace":[]}`)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("persist = %d — %s", rr.Code, rr.Body)
	}
	if got := countCompletedCard(t, pool, seedProjectID, "certainty-spectrum"); got != 1 {
		t.Fatalf("completed certainty-spectrum = %d, want 1", got)
	}
	if got := countEventsByType(t, pool, seedProjectID, "card_logged"); got != 1 {
		t.Fatalf("card_logged events = %d, want 1", got)
	}
}

func TestPostPersistProjectCard_RejectsNonProposable(t *testing.T) {
	h, cookie, pool := persistHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/cards/persist",
		strings.NewReader(`{"card_id":"toulmin","field_values":{"x":"y"},"event_trace":[]}`)), cookie))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("non-proposable card = %d, want 400 — %s", rr.Code, rr.Body)
	}
	if got := countCompletedCard(t, pool, seedProjectID, "toulmin"); got != 0 {
		t.Fatalf("non-proposable must persist nothing, got %d", got)
	}
}
