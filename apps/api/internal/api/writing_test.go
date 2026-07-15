package api_test

// writing_test.go — Task 3: PUT /projects/{id}/buffer (silent edit buffer
// upsert) and POST /projects/{id}/snapshots (immutable draft commit). An
// in-band commit (word count within the skill's word_budget) mints the
// word_budget_ok graph node that satisfies the S5 machine gate; an
// out-of-band commit must not mint it (and removes it if a prior in-band
// commit had minted it).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// nodeOfTypeExists lists the project's graph nodes directly via the sqlc
// queries and reports whether one of the given type exists.
func nodeOfTypeExists(t *testing.T, pool *pgxpool.Pool, projectID, nodeType string) bool {
	t.Helper()
	nodes, err := sqlc.New(pool).ListGraphNodesByProject(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGraphNodesByProject: %v", err)
	}
	for _, n := range nodes {
		if n.Type == nodeType {
			return true
		}
	}
	return false
}

func TestPutEditBuffer_Upserts(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("PUT", "/api/v1/projects/"+materialsTestProjectID+"/buffer",
		strings.NewReader(`{"content":"我在这里安静地写"}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PUT buffer = %d, want 204; body=%s", rec.Code, rec.Body)
	}

	// Second PUT overwrites.
	rec2 := httptest.NewRecorder()
	req2 := withCookie(httptest.NewRequest("PUT", "/api/v1/projects/"+materialsTestProjectID+"/buffer",
		strings.NewReader(`{"content":"改了"}`)), cookie)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNoContent {
		t.Fatalf("second PUT = %d, want 204", rec2.Code)
	}

	content, err := sqlc.New(pool).GetEditBuffer(req.Context(), mustUUID(materialsTestProjectID))
	if err != nil {
		t.Fatalf("GetEditBuffer: %v", err)
	}
	if content != "改了" {
		t.Errorf("buffer content = %q, want the second PUT's content (overwrite, not append)", content)
	}
}

func TestCommitSnapshot_MintsWordBudgetOkInBand(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	// An in-band draft (1500..2000 words). Build 1600 CJK chars.
	inBand := strings.Repeat("字", 1600)
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(inBand)+`}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var snap struct {
		ID        string `json:"id"`
		Seq       int    `json:"seq"`
		WordCount int    `json:"word_count"`
		InBand    bool   `json:"in_band"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode response: %v — %s", err, rec.Body.String())
	}
	if snap.Seq != 1 || snap.WordCount != 1600 || !snap.InBand {
		t.Fatalf("snapshot = %+v; want seq1 wc1600 inBand", snap)
	}

	if !nodeOfTypeExists(t, pool, materialsTestProjectID, "word_budget_ok") {
		t.Fatal("word_budget_ok node not minted for in-band commit")
	}
}

func TestCommitSnapshot_NoMintOutOfBand(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	short := strings.Repeat("字", 50) // below min
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(short)+`}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit = %d, want 201", rec.Code)
	}
	if nodeOfTypeExists(t, pool, materialsTestProjectID, "word_budget_ok") {
		t.Fatal("word_budget_ok minted for out-of-band commit")
	}
}

// TestCommitSnapshot_RemovesWordBudgetOkWhenLatestGoesOutOfBand — the node
// must track the LATEST snapshot honestly: a later out-of-band commit must
// remove a node an earlier in-band commit minted.
func TestCommitSnapshot_RemovesWordBudgetOkWhenLatestGoesOutOfBand(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	inBand := strings.Repeat("字", 1600)
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(inBand)+`}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first commit = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	if !nodeOfTypeExists(t, pool, materialsTestProjectID, "word_budget_ok") {
		t.Fatal("word_budget_ok node not minted for in-band commit")
	}

	short := strings.Repeat("字", 50)
	rec2 := httptest.NewRecorder()
	req2 := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(short)+`}`)), cookie)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("second commit = %d, want 201; body=%s", rec2.Code, rec2.Body)
	}
	if nodeOfTypeExists(t, pool, materialsTestProjectID, "word_budget_ok") {
		t.Fatal("word_budget_ok node should have been removed once the latest snapshot went out of band")
	}
}

func TestCommitSnapshot_EmptyContentRejected(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/snapshots",
		strings.NewReader(`{"content":""}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("commit empty content = %d, want 400: %s", rec.Code, rec.Body)
	}
}

// TestAttestGate_RecordsStudentWrittenItem — draft_polish's student_written
// item is "citations_matched" (writing-project.json). Attesting it records
// the gate_state node as solid — the write path nothing else exercises,
// since the planner's Advance deliberately never marks non-machine items.
func TestAttestGate_RecordsStudentWrittenItem(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/gate/draft_polish/attest",
		strings.NewReader(`{"item":"citations_matched","confirmed":true}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("attest = %d, want 204; body=%s", rec.Code, rec.Body)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	states, err := store.ListGateStates(context.Background(), mustUUID(materialsTestProjectID))
	if err != nil {
		t.Fatalf("ListGateStates: %v", err)
	}
	if states["draft_polish"].Items["citations_matched"] != "solid" {
		t.Fatalf("citations_matched = %q, want solid", states["draft_polish"].Items["citations_matched"])
	}
}

// TestAttestGate_RejectsUnknownItem — word_budget_ok is draft_polish's
// MACHINE gate item (computed, not student-attested); a caller must not be
// able to forge it as solid through this endpoint.
func TestAttestGate_RejectsUnknownItem(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+materialsTestProjectID+"/gate/draft_polish/attest",
		strings.NewReader(`{"item":"word_budget_ok","confirmed":true}`)), cookie) // machine item, not student_written
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("attest machine item = %d, want 400; body=%s", rec.Code, rec.Body)
	}
}
