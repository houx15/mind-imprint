package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "mindimprint/api/internal/api"
)

// The old task surface is retired (Slice 5d). This is a regression fence: a
// half-deleted router that still registers a task route would silently keep
// the dead path reachable.
//
// Built directly with a zero-value Deps rather than newTestAPI (which spins
// up testcontainers Postgres) — route *registration* is what's under test,
// and an unregistered route 404s before any handler or middleware touches
// Deps. None of these requests carry a session cookie, so SessionAuth never
// dereferences Deps.Queries either.
func TestOldTaskRoutesAreGone(t *testing.T) {
	t.Parallel()
	h := New(Deps{}).Handler()
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/tasks"},
		{"POST", "/api/v1/tasks"},
		{"GET", "/api/v1/tasks/00000000-0000-0000-0000-000000000001"},
		{"POST", "/api/v1/tasks/00000000-0000-0000-0000-000000000001/turn"},
		{"PUT", "/api/v1/tasks/00000000-0000-0000-0000-000000000001/cards/c1"},
		{"GET", "/api/v1/tasks/00000000-0000-0000-0000-000000000001/materials"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: got %d, want 404 (route should be retired)", tc.method, tc.path, rec.Code)
		}
	}
}
