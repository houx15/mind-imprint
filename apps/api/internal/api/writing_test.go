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

	content, err := sqlc.New(pool).GetEditBuffer(req.Context(), sqlc.GetEditBufferParams{ProjectID: mustUUID(materialsTestProjectID), DocKind: "essay"})
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

// countReviewItemsForVoice counts review_item interventions whose anchor
// voice matches (a missing anchor voice reads as board — the keystone
// back-compat case).
func countReviewItemsForVoice(t *testing.T, pool *pgxpool.Pool, projectID, voice string) int {
	t.Helper()
	rows, err := sqlc.New(pool).ListInterventionsByProject(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListInterventionsByProject: %v", err)
	}
	n := 0
	for _, iv := range rows {
		if iv.Type != "review_item" {
			continue
		}
		var a struct{ Voice string }
		_ = json.Unmarshal(iv.Anchor, &a)
		if a.Voice == "" {
			a.Voice = "board"
		}
		if a.Voice == voice {
			n++
		}
	}
	return n
}

// Two voices on the SAME snapshot are independent caches: each does one model
// call and persists its own disjoint row set; re-running a voice replays with
// no new call. A keystone row (anchor with no voice) is served as board.
func TestOrderReview_PerVoiceIndependentCaches(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"e","missing":"m","fix":"补定义"}]`
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: reviewStubProvider(reply), ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	content := strings.Repeat("字", 1600)
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(content)+`}`)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("commit = %d; body=%s", rec.Code, rec.Body)
	}
	var snap struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &snap)

	order := func(voice string) string {
		r := httptest.NewRecorder()
		url := "/api/v1/projects/" + projectID + "/snapshots/" + snap.ID + "/review"
		if voice != "" {
			url += "?voice=" + voice
		}
		req := withCookie(httptest.NewRequest("POST", url, strings.NewReader("")), cookie)
		h.ServeHTTP(r, req)
		if r.Code != http.StatusOK {
			t.Fatalf("order review voice=%q = %d; body=%s", voice, r.Code, r.Body)
		}
		return r.Body.String()
	}

	order("board")
	callsAfterBoard := countLLMCalls(t, pool, projectID)
	if callsAfterBoard != 1 {
		t.Fatalf("after board review, llm_calls = %d, want 1", callsAfterBoard)
	}
	if n := countReviewItems(t, pool, projectID); n != 1 {
		t.Fatalf("after board review, review_items = %d, want 1", n)
	}

	order("sceptic") // distinct voice → a second, independent model call + row
	if n := countLLMCalls(t, pool, projectID); n != 2 {
		t.Fatalf("after sceptic review, llm_calls = %d, want 2 (independent cache)", n)
	}
	if n := countReviewItems(t, pool, projectID); n != 2 {
		t.Fatalf("after sceptic review, review_items = %d, want 2 (disjoint set)", n)
	}

	order("sceptic") // replay — no new call, no new row
	if n := countLLMCalls(t, pool, projectID); n != 2 {
		t.Fatalf("after sceptic replay, llm_calls = %d, want 2 (replay, no new call)", n)
	}
	if n := countReviewItems(t, pool, projectID); n != 2 {
		t.Fatalf("after sceptic replay, review_items = %d, want 2 (replay)", n)
	}

	// Per-voice row counts: exactly one board row, one sceptic row.
	if n := countReviewItemsForVoice(t, pool, projectID, "board"); n != 1 {
		t.Fatalf("board rows = %d, want 1", n)
	}
	if n := countReviewItemsForVoice(t, pool, projectID, "sceptic"); n != 1 {
		t.Fatalf("sceptic rows = %d, want 1", n)
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

// confirmSolid marks a contract's gate_state as confirmed_solid directly
// through the store — test-only scaffolding used by the two tests below to
// satisfy a prerequisite contract the seed fixture deliberately leaves
// un-confirmed, so AdvanceAll will even consider the contract under test.
func confirmSolid(t *testing.T, store agent.AgentStore, projectID, contract string) {
	t.Helper()
	recorded, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates (confirmSolid %s): %v", contract, err)
	}
	rec := recorded[contract]
	rec.Confirmed = true
	if err := store.UpsertGateState(context.Background(), mustUUID(projectID), contract, rec); err != nil {
		t.Fatalf("UpsertGateState (confirmSolid %s): %v", contract, err)
	}
}

// TestCommitSnapshot_AdvancesGateInTheSameRequest — I3 (whole-branch review
// IMPORTANT): commitSnapshot mints/removes word_budget_ok, draft_polish's
// only MACHINE item, so a commit can complete the gate entirely on its own.
// Before this fix, advanceGates was never called here, so Confirmed stayed
// stale until some OTHER write happened to call it. This pre-solidifies the
// other two items directly, then asserts Confirmed flips true from the
// commit request ALONE — no second request in between.
func TestCommitSnapshot_AdvancesGateInTheSameRequest(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)

	// The seeded demo project deliberately leaves build_argument (S4)
	// current, not confirmed (0018_seed_demo_project.sql) — draft_polish's
	// own prerequisite. AdvanceAll only ever considers a contract once its
	// requires are solid, so without this the test would fail regardless of
	// the fix under test. Pre-confirming it here is test-only scaffolding,
	// not part of what's being asserted.
	confirmSolid(t, store, projectID, "build_argument")

	// Pre-solidify whole_draft_review (human item) directly through the
	// store — its real producer is orderReview, exercised separately by
	// TestOrderReview_AdvancesGateInTheSameRequest below; this test is about
	// commitSnapshot's own advance.
	recorded, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates: %v", err)
	}
	rec := recorded["draft_polish"]
	if rec.Items == nil {
		rec.Items = map[string]string{}
	}
	rec.Items["whole_draft_review"] = "solid"
	if err := store.UpsertGateState(context.Background(), mustUUID(projectID), "draft_polish", rec); err != nil {
		t.Fatalf("UpsertGateState: %v", err)
	}

	// citations_matched (student_written item) through its real endpoint.
	recAttest := httptest.NewRecorder()
	h.ServeHTTP(recAttest, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/gate/draft_polish/attest",
		strings.NewReader(`{"item":"citations_matched","confirmed":true}`)), cookie))
	if recAttest.Code != http.StatusNoContent {
		t.Fatalf("attest citations_matched = %d, want 204; body=%s", recAttest.Code, recAttest.Body)
	}

	// Confirm the gate is NOT solid yet — word_budget_ok (the last item) is
	// still missing. This is asserting the test's own setup, not the fix.
	before, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates (before commit): %v", err)
	}
	if before["draft_polish"].Confirmed {
		t.Fatal("draft_polish already Confirmed before the commit — test setup is wrong")
	}

	// The ONE request under test: committing an in-band snapshot mints
	// word_budget_ok, completing all three draft_polish items.
	content := strings.Repeat("字", 1600)
	recCommit := httptest.NewRecorder()
	h.ServeHTTP(recCommit, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(content)+`}`)), cookie))
	if recCommit.Code != http.StatusCreated {
		t.Fatalf("commit snapshot = %d, want 201; body=%s", recCommit.Code, recCommit.Body)
	}

	after, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates (after commit): %v", err)
	}
	if !after["draft_polish"].Confirmed {
		t.Fatal("draft_polish.Confirmed = false after the commit that completed its last item — advanceGates missing from commitSnapshot")
	}
}

// TestOrderReview_AdvancesGateInTheSameRequest — I3 (whole-branch review
// IMPORTANT): orderReview writes whole_draft_review, draft_polish's only
// HUMAN item, so ordering a review can complete the gate entirely on its
// own. Pre-solidifies the other two items, then asserts Confirmed flips
// true from the review request ALONE — no second request in between.
func TestOrderReview_AdvancesGateInTheSameRequest(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"第2段接住反方","missing":"跳步没补","fix":"补上定义"}]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     reviewStubProvider(reply),
		ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	// See TestCommitSnapshot_AdvancesGateInTheSameRequest's own comment:
	// build_argument (S4) is deliberately left un-confirmed by the seed
	// fixture — pre-confirm it here so draft_polish is even considered by
	// AdvanceAll. Test-only scaffolding, not part of what's being asserted.
	confirmSolid(t, agent.NewSqlcAgentStore(sqlc.New(pool), pool), projectID, "build_argument")

	// Commit an in-band snapshot first — mints word_budget_ok (the machine
	// item) and is also the prerequisite snapshot a review is ordered over.
	content := strings.Repeat("字", 1600)
	recCommit := httptest.NewRecorder()
	h.ServeHTTP(recCommit, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots",
		strings.NewReader(`{"content":`+strconv.Quote(content)+`}`)), cookie))
	if recCommit.Code != http.StatusCreated {
		t.Fatalf("commit snapshot = %d, want 201; body=%s", recCommit.Code, recCommit.Body)
	}
	var snap struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(recCommit.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	// citations_matched (student_written item) through its real endpoint.
	recAttest := httptest.NewRecorder()
	h.ServeHTTP(recAttest, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/gate/draft_polish/attest",
		strings.NewReader(`{"item":"citations_matched","confirmed":true}`)), cookie))
	if recAttest.Code != http.StatusNoContent {
		t.Fatalf("attest citations_matched = %d, want 204; body=%s", recAttest.Code, recAttest.Body)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	before, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates (before review): %v", err)
	}
	if before["draft_polish"].Confirmed {
		t.Fatal("draft_polish already Confirmed before the review — test setup is wrong")
	}

	// The ONE request under test: ordering the review writes
	// whole_draft_review, completing all three draft_polish items.
	recReview := httptest.NewRecorder()
	h.ServeHTTP(recReview, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/snapshots/"+snap.ID+"/review",
		strings.NewReader("")), cookie))
	if recReview.Code != http.StatusOK {
		t.Fatalf("order review = %d, want 200; body=%s", recReview.Code, recReview.Body)
	}

	after, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates (after review): %v", err)
	}
	if !after["draft_polish"].Confirmed {
		t.Fatal("draft_polish.Confirmed = false after the review that completed its last item — advanceGates missing from orderReview")
	}
}
