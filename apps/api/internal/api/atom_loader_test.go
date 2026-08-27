package api_test

// atom_loader_test.go — Task 1.5: the generalised loader (loadOwnedAtom /
// loadOwnedAtomRow, readings.go) takes an explicit kind instead of hardcoding
// "reading". These tests exercise that generalisation directly.
//
// No /writings/* route is registered yet — mounting one is a later task's
// job — so the "writing" direction of every test here calls the loader via
// LoadOwnedAtomForTest (export_test.go), not through the mux. That shim
// exists ONLY in the test build (export_test.go is package api, a _test.go
// file) and is never reachable from production code.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// createWritingAtom mints a bare writing atom + writing row owned by
// SeedUserID, mirroring createReadingAtom (readings_test.go) but going
// straight to the store — there is no POST /api/v1/writings route yet.
func createWritingAtom(t *testing.T, q *sqlc.Queries) sqlc.Atom {
	t.Helper()
	at, err := q.CreateAtom(context.Background(), sqlc.CreateAtomParams{Kind: "writing", UserID: SeedUserID})
	if err != nil {
		t.Fatalf("create writing atom: %v", err)
	}
	if _, err := q.CreateWriting(context.Background(), sqlc.CreateWritingParams{
		AtomID: at.ID, Title: "一篇作文", Lang: "zh",
	}); err != nil {
		t.Fatalf("create writing row: %v", err)
	}
	return at
}

// studentCtx builds a request context carrying SeedUserID as the signed-in
// student — what SessionAuth+RequireUser would have put there, constructed
// directly since these tests call the loader without going through the mux.
func studentCtx() context.Context {
	return WithUser(context.Background(), User{ID: SeedUserID, SchoolID: SeedSchoolID, Role: "student"})
}

// atomReq builds a GET-or-other-method request carrying the signed-in
// student context and {id} set the way http.ServeMux would after routing.
func atomReq(method, id string) *http.Request {
	r := httptest.NewRequest(method, "/api/v1/x/"+id, nil).WithContext(studentCtx())
	r.SetPathValue("id", id)
	return r
}

// TestLoadOwnedAtom_CrossKindIsolationBothDirections is the mirror of
// TestLoadOwnedReadingAtom_WrongKindIs404 (readings_test.go), which already
// covers direction 1 (a writing atom requested through a /readings/{id}/...
// route → 404) end-to-end over HTTP and needs no change — it still passes
// unchanged after this refactor because loadOwnedReadingAtom is now a thin
// wrapper over loadOwnedAtom(..., "reading").
//
// This test adds direction 2 — the one no existing route can exercise: a
// READING atom requested through the WRITING kind must ALSO 404, never 403
// and never 200. This is the entire reason the loader takes an explicit
// kind parameter instead of being told "reading" everywhere: the isolation
// has to hold in both directions once a second kind exists, not just the
// one direction the pre-refactor code happened to hardcode.
func TestLoadOwnedAtom_CrossKindIsolationBothDirections(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	a := New(Deps{Queries: q, Pool: pool})

	readingAtom := createReadingAtomRow(t, q)
	writingAtom := createWritingAtom(t, q)

	// Direction 1 (already covered end-to-end over HTTP by
	// TestLoadOwnedReadingAtom_WrongKindIs404) — repeated here at the loader
	// level for symmetry with direction 2, and because it's now the SAME
	// generalised function under test, just curried differently.
	w1 := httptest.NewRecorder()
	if _, ok := a.LoadOwnedAtomForTest(w1, atomReq(http.MethodGet, writingAtom.ID.String()), "reading"); ok {
		t.Fatal("loadOwnedAtom(\"reading\") accepted a writing atom")
	}
	if w1.Code != http.StatusNotFound {
		t.Fatalf("writing atom via reading kind = %d, want 404; body=%s", w1.Code, w1.Body)
	}
	if !strings.Contains(w1.Body.String(), `"code":"not_found"`) {
		t.Fatalf("body missing not_found code — got %s", w1.Body)
	}

	// Direction 2 (the mirror) — a reading atom requested through the
	// writing kind must 404 exactly the same way.
	w2 := httptest.NewRecorder()
	if _, ok := a.LoadOwnedAtomForTest(w2, atomReq(http.MethodGet, readingAtom.ID.String()), "writing"); ok {
		t.Fatal("loadOwnedAtom(\"writing\") accepted a reading atom")
	}
	if w2.Code != http.StatusNotFound {
		t.Fatalf("reading atom via writing kind = %d, want 404; body=%s", w2.Code, w2.Body)
	}
	if !strings.Contains(w2.Body.String(), `"code":"not_found"`) {
		t.Fatalf("body missing not_found code — got %s", w2.Body)
	}
}

// TestLoadOwnedAtom_WritingFinishedGateIsIndependentOfReading — Task 1.5's
// atomIsFinished dispatch must read the WRITING table (not reading) for a
// writing atom, and refuse with the WRITING-specific 403 code. A bug that
// accidentally left the dispatch calling GetReading for every kind would
// either 500 (no reading row for a writing atom_id) or — worse — silently
// treat every writing atom as never-finished; this pins the correct
// behaviour directly.
func TestLoadOwnedAtom_WritingFinishedGateIsIndependentOfReading(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	a := New(Deps{Queries: q, Pool: pool})

	writingAtom := createWritingAtom(t, q)
	if err := q.SetWritingFinished(context.Background(), writingAtom.ID); err != nil {
		t.Fatalf("SetWritingFinished: %v", err)
	}

	// non-GET on a finished writing atom → 403 writing_finished, NOT
	// reading_finished and NOT a 500 from misreading the wrong table.
	wPost := httptest.NewRecorder()
	if _, ok := a.LoadOwnedAtomForTest(wPost, atomReq(http.MethodPost, writingAtom.ID.String()), "writing"); ok {
		t.Fatal("loadOwnedAtom(\"writing\") allowed a write against a finished writing atom")
	}
	if wPost.Code != http.StatusForbidden {
		t.Fatalf("finished writing non-GET = %d, want 403; body=%s", wPost.Code, wPost.Body)
	}
	if !strings.Contains(wPost.Body.String(), `"code":"writing_finished"`) {
		t.Fatalf("body missing writing_finished code — got %s", wPost.Body)
	}

	// GET on the same finished writing atom still works — the gate is
	// method-based (non-GET only), mirroring
	// TestFinishedReading_ReadsStillWork (reading_finished_gate_test.go).
	wGet := httptest.NewRecorder()
	if _, ok := a.LoadOwnedAtomForTest(wGet, atomReq(http.MethodGet, writingAtom.ID.String()), "writing"); !ok {
		t.Fatalf("GET on a finished writing atom was rejected; body=%s", wGet.Body)
	}
}

// createReadingAtomRow mints a bare reading atom + reading row owned by
// SeedUserID directly against the store — the loader-level tests above need
// a reading atom but not the HTTP surface createReadingAtom
// (readings_test.go) drives.
func createReadingAtomRow(t *testing.T, q *sqlc.Queries) sqlc.Atom {
	t.Helper()
	at, err := q.CreateAtom(context.Background(), sqlc.CreateAtomParams{Kind: "reading", UserID: SeedUserID})
	if err != nil {
		t.Fatalf("create reading atom: %v", err)
	}
	if _, err := q.CreateReading(context.Background(), sqlc.CreateReadingParams{
		AtomID: at.ID, Title: "一篇文章", Lang: "zh",
	}); err != nil {
		t.Fatalf("create reading row: %v", err)
	}
	return at
}
