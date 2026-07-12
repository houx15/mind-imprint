package api_test

// disposition_test.go — Task 7: POST
// /api/v1/projects/{id}/interventions/{iid}/disposition -> RecordDisposition.
// Uses the real testcontainers Postgres + the seeded demo project
// (00000000-0000-0000-0000-000000000101, owned by Phoebe) and its seeded
// intervention (00000000-0000-0000-0000-000000000121).

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestInterventionDisposition(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	base := "/api/v1/projects/00000000-0000-0000-0000-000000000101/interventions/00000000-0000-0000-0000-000000000121/disposition"

	// happy path (>=15 runes) -> 204.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base, strings.NewReader(`{"action":"accept","reason":"这条我接受，因为它把证据连回了主张"}`)), cookie))
	if rr.Code != 204 {
		t.Fatalf("happy: want 204, got %d — %s", rr.Code, rr.Body.String())
	}

	// short reason -> 400.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", base, strings.NewReader(`{"action":"reject","reason":"太短"}`)), cookie))
	if rr.Code != 400 {
		t.Fatalf("short: want 400, got %d", rr.Code)
	}

	// non-owned project -> 404.
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-0000000009ff/interventions/00000000-0000-0000-0000-000000000121/disposition", strings.NewReader(`{"action":"accept","reason":"这条我接受因为理由足够长了"}`)), cookie))
	if rr.Code != 404 {
		t.Fatalf("foreign: want 404, got %d", rr.Code)
	}

	// owned project, but intervention id doesn't belong to it (cross-tenant /
	// nonexistent intervention) -> 404, not a silent 204 (IDOR regression guard).
	// Reason is >=15 runes so it's the scoping check rejecting this, not the
	// reason guard.
	foreignIID := uuid.NewString()
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, withCookie(httptest.NewRequest("POST", "/api/v1/projects/00000000-0000-0000-0000-000000000101/interventions/"+foreignIID+"/disposition", strings.NewReader(`{"action":"accept","reason":"这条我接受因为理由足够长了"}`)), cookie))
	if rr.Code != 404 {
		t.Fatalf("foreign intervention scoped to owned project: want 404, got %d — %s", rr.Code, rr.Body.String())
	}
}
