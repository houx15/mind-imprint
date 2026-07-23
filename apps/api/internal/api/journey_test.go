package api_test

// journey_test.go — Task 5 (N6-E): POST
// /api/v1/projects/{id}/journey/reopen/{code} — the student's escape hatch
// (铁律 2) for a composed journey. A waived station is never a permanent
// wall: re-opening un-waives it, re-plans, and appends journey_reopened.

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// TestReopenStationUnwaives drives a freshly created project through
// SetWaived([decode_task, frame_question]) — simulating a compose-time
// waive of S0 and S1 — then re-opens S1 over HTTP and checks the projection
// and event stream both reflect it, while S0 remains waived.
func TestReopenStationUnwaives(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	pid := createProjectForTest(t, h, cookie)
	pidUUID, err := uuid.Parse(pid)
	if err != nil {
		t.Fatalf("parse project id: %v", err)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	if err := store.SetWaived(context.Background(), pidUUID, []string{"decode_task", "frame_question"}); err != nil {
		t.Fatalf("SetWaived: %v", err)
	}

	snap := fetchStations(t, h, pid, cookie)
	if got := stationState(t, snap, "S0"); got != "waived" {
		t.Fatalf("S0 state before reopen = %q, want waived", got)
	}
	if got := stationState(t, snap, "S1"); got != "waived" {
		t.Fatalf("S1 state before reopen = %q, want waived", got)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/journey/reopen/S1", nil), cookie))
	if rec.Code != 200 {
		t.Fatalf("reopen S1 = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var body struct {
		Reopened bool `json:"reopened"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal reopen response: %v; raw=%s", err, rec.Body.Bytes())
	}
	if !body.Reopened {
		t.Fatalf("reopen S1 response = %+v, want reopened:true", body)
	}

	snap = fetchStations(t, h, pid, cookie)
	if got := stationState(t, snap, "S1"); got == "waived" {
		t.Fatalf("S1 state after reopen = %q, want no longer waived", got)
	}
	if got := stationState(t, snap, "S0"); got != "waived" {
		t.Fatalf("S0 state after reopening S1 = %q, want still waived", got)
	}

	var eventCount int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM event WHERE project_id=$1 AND type='journey_reopened'`, pid).
		Scan(&eventCount); err != nil {
		t.Fatalf("count journey_reopened events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("journey_reopened event count = %d, want 1", eventCount)
	}
}

// TestReopenStationRejectsBadCodeAndForeignProject covers the guard rails: a
// malformed/out-of-range code is 400, a foreign project is 404 (via
// loadOwnedProject), and re-opening a station that is NOT waived is an
// idempotent 200 no-op rather than an error.
func TestReopenStationRejectsBadCodeAndForeignProject(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, SpecByID: cards.ByID}).Handler()
	cookie := signInSeed(t, pool)

	pid := createProjectForTest(t, h, cookie)

	// Bad code (out of range: skill only has 7 stations, S0..S6).
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/journey/reopen/S9", nil), cookie))
	if rec.Code != 400 {
		t.Fatalf("reopen S9 = %d, want 400; body=%s", rec.Code, rec.Body)
	}

	// Foreign project (not owned by the signed-in seed user) -> 404.
	foreignID := uuid.NewString()
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+foreignID+"/journey/reopen/S0", nil), cookie))
	if rec.Code != 404 {
		t.Fatalf("reopen on foreign project = %d, want 404; body=%s", rec.Code, rec.Body)
	}

	// S0 is not waived on this project (nothing was composed away) -> 200 no-op.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/journey/reopen/S0", nil), cookie))
	if rec.Code != 200 {
		t.Fatalf("reopen non-waived S0 = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var body struct {
		Reopened bool `json:"reopened"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal reopen response: %v; raw=%s", err, rec.Body.Bytes())
	}
	if body.Reopened {
		t.Fatalf("reopen non-waived S0 response = %+v, want reopened:false", body)
	}
	snap := fetchStations(t, h, pid, cookie)
	if got := stationState(t, snap, "S0"); got == "waived" {
		t.Fatalf("S0 state after no-op reopen = %q, want not waived", got)
	}
}
