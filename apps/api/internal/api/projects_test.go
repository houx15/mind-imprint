package api_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

func TestProjectsEndpoints(t *testing.T) {
	pool := newAPITestPool(t) // applies migrations incl. the 0018 demo seed
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()

	// 401 unauthenticated.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/projects", nil))
	if rr.Code != 401 {
		t.Fatalf("list no cookie: want 401, got %d", rr.Code)
	}

	cookie := signInSeed(t, pool) // Phoebe (SeedUserID = …0003)

	// list includes the seeded project.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects", nil), cookie))
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "0457 个人报告") {
		t.Fatalf("list: %d — %s", rr.Code, rr.Body.String())
	}

	// detail projects the studio state.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects/00000000-0000-0000-0000-000000000101", nil), cookie))
	if rr.Code != 200 {
		t.Fatalf("detail: %d — %s", rr.Code, rr.Body.String())
	}
	for _, want := range []string{`"activeStation":"S4"`, `"论证构建"`, `"论证图 · 治理决心主张"`} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Fatalf("detail missing %s — %s", want, rr.Body.String())
		}
	}

	// 404 for another user's project id (use a random non-owned uuid).
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("GET", "/api/v1/projects/00000000-0000-0000-0000-0000000009ff", nil), cookie))
	if rr.Code != 404 {
		t.Fatalf("foreign project: want 404, got %d", rr.Code)
	}
}
