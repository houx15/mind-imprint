package api_test

// exploration_card_test.go — S4 · the rabbit-hole card's persistence path.
// A completed envelope becomes a durable card_instance (status=completed) plus a
// rabbit_hole_logged process event; a malformed envelope is rejected and
// persists nothing; a non-owned project is hidden as 404.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func rabbitHoleHandler(t *testing.T) (http.Handler, *http.Cookie, *pgxpool.Pool) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	return h, signInSeed(t, pool), pool
}

func countRabbitHoleCards(t *testing.T, pool *pgxpool.Pool, projectID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM card_instances WHERE project_id=$1 AND card_id='rabbit-hole' AND status='completed'`,
		mustUUID(projectID)).Scan(&n); err != nil {
		t.Fatalf("countRabbitHoleCards: %v", err)
	}
	return n
}

func countEventsByType(t *testing.T, pool *pgxpool.Pool, projectID, typ string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE project_id=$1 AND type=$2`, mustUUID(projectID), typ).Scan(&n); err != nil {
		t.Fatalf("countEventsByType: %v", err)
	}
	return n
}

func TestPostExplorationRabbitHole_PersistsAndLogs(t *testing.T) {
	h, cookie, pool := rabbitHoleHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	envelope := `{"field_values":{"interest":"Exxon 的气候传播","exit_reason":"跑题+篇幅限制，放进 rabbit hole log"},"event_trace":[]}`
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/exploration/rabbit-hole", strings.NewReader(envelope)), cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("rabbit-hole = %d — %s", rr.Code, rr.Body)
	}
	var resp struct {
		CardInstanceID string `json:"cardInstanceId"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil || resp.CardInstanceID == "" {
		t.Fatalf("decode (err=%v): %s", err, rr.Body)
	}
	if got := countRabbitHoleCards(t, pool, seedProjectID); got != 1 {
		t.Fatalf("completed rabbit-hole card_instances = %d, want 1", got)
	}
	if got := countEventsByType(t, pool, seedProjectID, "rabbit_hole_logged"); got != 1 {
		t.Fatalf("rabbit_hole_logged events = %d, want 1", got)
	}
}

func TestPostExplorationRabbitHole_RejectsBadEnvelope(t *testing.T) {
	h, cookie, pool := rabbitHoleHandler(t)
	base := "/api/v1/projects/" + seedProjectID

	// field_values is an array, not an object → 400, nothing persisted.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/exploration/rabbit-hole",
		strings.NewReader(`{"field_values":[],"event_trace":[]}`)), cookie))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad envelope = %d, want 400 — %s", rr.Code, rr.Body)
	}
	if got := countRabbitHoleCards(t, pool, seedProjectID); got != 0 {
		t.Fatalf("bad envelope must persist nothing, got %d", got)
	}
}

func TestPostExplorationRabbitHole_NonOwnedIs404(t *testing.T) {
	h, cookie, _ := rabbitHoleHandler(t)
	// A syntactically valid but non-owned/nonexistent project id → 404-no-leak.
	base := "/api/v1/projects/00000000-0000-0000-0000-0000000009ff"

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base+"/exploration/rabbit-hole",
		strings.NewReader(`{"field_values":{"interest":"x"},"event_trace":[]}`)), cookie))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("non-owned project = %d, want 404 — %s", rr.Code, rr.Body)
	}
}
