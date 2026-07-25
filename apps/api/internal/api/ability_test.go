package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestGetAbilityModel_EmptyStateNoModelCall — a fresh student with no evaluations
// gets a 200 empty model (length-6 depth all insufficient), and NO llm_call row is
// written (deterministic projection, no model call).
func TestGetAbilityModel_EmptyStateNoModelCall(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/growth/ability", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /growth/ability = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var m struct {
		TotalSessions int `json:"totalSessions"`
		Depth         []struct {
			Code  string `json:"code"`
			Level int    `json:"level"`
		} `json:"depth"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode ability model: %v — body=%s", err, rec.Body)
	}
	if m.TotalSessions != 0 || len(m.Depth) != 6 {
		t.Fatalf("empty model = sessions %d depth %d, want 0 / 6", m.TotalSessions, len(m.Depth))
	}
	for _, d := range m.Depth {
		if d.Level != -1 {
			t.Fatalf("empty depth %s level %d, want -1", d.Code, d.Level)
		}
	}
	// deterministic: no model call happened.
	if n := countAllLLMCalls(t, pool); n != 0 {
		t.Fatalf("llm_call rows = %d, want 0 (ability is a pure projection)", n)
	}
}

// countAllLLMCalls counts every llm_call row (the ability endpoint must add
// none) — same *pgxpool.Pool raw-query style as assertProjectStatus in
// project_finish_test.go, but unscoped since this endpoint has no project id.
func countAllLLMCalls(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM llm_call").Scan(&n); err != nil {
		t.Fatalf("count all llm_call rows: %v", err)
	}
	return n
}
