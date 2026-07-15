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
	"mindimprint/api/internal/gateway"
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

// reviewStubProvider returns a canned model reply for ProposeReview's own
// gateway.Collect call — same scripted-provider shape as studioturn_test.go's
// fakeProvider, just carrying a review work-order JSON array as the reply.
func reviewStubProvider(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 42, OutputTokens: 17}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// countReviewItems counts the project's persisted review_item interventions.
func countReviewItems(t *testing.T, pool *pgxpool.Pool, projectID string) int {
	t.Helper()
	ivs, err := sqlc.New(pool).ListInterventionsByProject(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListInterventionsByProject: %v", err)
	}
	n := 0
	for _, iv := range ivs {
		if iv.Type == "review_item" {
			n++
		}
	}
	return n
}

// countLLMCalls counts the project's metered llm_call rows (same query
// studioturn_test.go's TestProjectTurn_PersistsAnchorsUsage uses) — the
// idempotent-replay test asserts this does NOT move on a second order, and
// the rejected-proposal test asserts it DOES move by exactly one (a rejected
// call still cost money).
func countLLMCalls(t *testing.T, pool *pgxpool.Pool, projectID string) int {
	t.Helper()
	calls, err := sqlc.New(pool).ListLLMCallsByProject(context.Background(), pgUUID(mustUUID(projectID)))
	if err != nil {
		t.Fatalf("ListLLMCallsByProject: %v", err)
	}
	return len(calls)
}

// TestOrderReview_PersistsWorkOrderAndIsIdempotent — Task 6: ordering a
// review over a committed snapshot streams the work-order over SSE,
// persists each item as a review_item intervention anchored to the
// snapshot, and records the S5 human gate item whole_draft_review. A second
// order on the SAME snapshot must replay the same rows with NO second model
// call (spec §12: one snapshot = one review).
func TestOrderReview_PersistsWorkOrderAndIsIdempotent(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"第2段接住反方","missing":"跳步没补","fix":"补上定义"}]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     reviewStubProvider(reply),
		ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool) // Phoebe, owns materialsTestProjectID
	projectID := materialsTestProjectID

	// Commit a snapshot first — an in-band draft, same shape as the commit
	// tests above.
	content := strings.Repeat("字", 1600)
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(content)+`}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit snapshot = %d, want 201; body=%s", rec.Code, rec.Body)
	}
	var snap struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	// Order the review — SSE stream carries one `review` event with 1 item.
	rec2 := httptest.NewRecorder()
	req2 := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots/"+snap.ID+"/review",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("order review = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}
	body := rec2.Body.String()
	if !strings.Contains(body, `"criterion_code":"表E"`) {
		t.Fatalf("review stream missing work order: %s", body)
	}
	if !strings.Contains(body, "event: review") {
		t.Fatalf("review stream missing the review event: %s", body)
	}

	// Persisted as exactly one review_item intervention.
	if n := countReviewItems(t, pool, projectID); n != 1 {
		t.Fatalf("review_item interventions = %d, want 1", n)
	}
	callsAfterFirst := countLLMCalls(t, pool, projectID)

	// Second review on the SAME snapshot returns the SAME rows, no new
	// model call (the stub would still satisfy a second call, so what this
	// actually proves is the row count staying at 1 — a second model call
	// would either duplicate the row or the reply would be unmarshalled
	// again; either way idempotency means the count must not move).
	rec3 := httptest.NewRecorder()
	req3 := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots/"+snap.ID+"/review",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("second order review = %d, want 200; body=%s", rec3.Code, rec3.Body)
	}
	body3 := rec3.Body.String()
	if !strings.Contains(body3, `"criterion_code":"表E"`) {
		t.Fatalf("second review stream missing work order: %s", body3)
	}
	if n := countReviewItems(t, pool, projectID); n != 1 {
		t.Fatalf("after 2nd review, review_item interventions = %d, want 1 (idempotent)", n)
	}
	// The replay must not have made a second model call — llm_call row
	// count must be unchanged from after the first (real) review.
	if n := countLLMCalls(t, pool, projectID); n != callsAfterFirst {
		t.Fatalf("llm_call rows after 2nd (replayed) review = %d, want %d (no second model call)", n, callsAfterFirst)
	}

	// Ordering the review recorded the S5 human gate item.
	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	rec4, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates: %v", err)
	}
	if rec4["draft_polish"].Items["whole_draft_review"] != "solid" {
		t.Fatalf("whole_draft_review = %q, want solid", rec4["draft_polish"].Items["whole_draft_review"])
	}
}

// TestOrderReview_RejectedProposalPersistsNothing — a banned-phrasing
// rejection from ProposeReview must persist NOTHING (spec RL-1: the review
// writes only typed advice, and a rejected proposal is not advice at all)
// and stream an error envelope instead of a review event.
func TestOrderReview_RejectedProposalPersistsNothing(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"e","missing":"m","fix":"你应该这样写：中国的转型是叠加式的。"}]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     reviewStubProvider(reply),
		ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	content := strings.Repeat("字", 1600)
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(content)+`}`)), cookie)
	h.ServeHTTP(rec, req)
	var snap struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	rec2 := httptest.NewRecorder()
	req2 := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots/"+snap.ID+"/review",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("order review = %d, want 200 (SSE error, not HTTP error); body=%s", rec2.Code, rec2.Body)
	}
	if !strings.Contains(rec2.Body.String(), "event: error") {
		t.Fatalf("expected an SSE error event: %s", rec2.Body.String())
	}
	if n := countReviewItems(t, pool, projectID); n != 0 {
		t.Fatalf("rejected proposal persisted %d review_item interventions, want 0", n)
	}
	// A rejected proposal still cost money — the llm_call row must exist
	// even though nothing was persisted as advice.
	if n := countLLMCalls(t, pool, projectID); n != 1 {
		t.Fatalf("llm_call rows after rejected review = %d, want 1 (cost-on-rejection)", n)
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
