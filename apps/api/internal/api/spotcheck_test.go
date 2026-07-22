package api_test

// spotcheck_test.go — N3f Task 4: POST /projects/{id}/contracts/{contractId}/
// spot-check. Mirrors writing_test.go's orderReview harness (testcontainers
// Postgres + seeded demo project, gateway.NewStubProvider for the model,
// httptest.ResponseRecorder read directly as the SSE body) — no second
// harness invented.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// spotCheckStubProvider returns a canned model reply for ProposeSpotCheck's
// own gateway.Collect call — same scripted-provider shape as
// writing_test.go's reviewStubProvider.
func spotCheckStubProvider(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 12}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

// countSpotCheckItems counts the project's persisted spot_check_item
// interventions.
func countSpotCheckItems(t *testing.T, pool *pgxpool.Pool, projectID string) int {
	t.Helper()
	ivs, err := sqlc.New(pool).ListInterventionsByProject(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListInterventionsByProject: %v", err)
	}
	n := 0
	for _, iv := range ivs {
		if iv.Type == "spot_check_item" {
			n++
		}
	}
	return n
}

// countSpotCheckOrderedEvents counts the project's spot_check_ordered events.
func countSpotCheckOrderedEvents(t *testing.T, pool *pgxpool.Pool, projectID string) int {
	t.Helper()
	rows, err := sqlc.New(pool).ListEventsByProject(context.Background(), pgtype.UUID{Bytes: mustUUID(projectID), Valid: true})
	if err != nil {
		t.Fatalf("ListEventsByProject: %v", err)
	}
	n := 0
	for _, r := range rows {
		if r.Type == "spot_check_ordered" {
			n++
		}
	}
	return n
}

// errRow is a pgx.Row whose Scan always fails — used by
// failingInterventionDBTX to force every intervention insert to error.
type errRow struct{ err error }

func (r errRow) Scan(dest ...any) error { return r.err }

// failingInterventionDBTX wraps the real pool but forces every
// "INSERT INTO intervention" QueryRow to fail — the only way to exercise
// "every insert fails" against a real Postgres backend without corrupting
// any of the OTHER queries the same request makes (ListGateStates/
// UpsertGateState/AppendEvent/ListInterventionsByProject all still go
// through the real pool via the embedded methods below).
type failingInterventionDBTX struct {
	*pgxpool.Pool
}

func (f failingInterventionDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql, "INSERT INTO intervention") {
		return errRow{err: errors.New("forced insert failure (test)")}
	}
	return f.Pool.QueryRow(ctx, sql, args...)
}

// TestOrderSpotCheck_UnknownContractIs404 — (a) an unknown contractId must
// 404 and make no model call: it must never reach ProposeSpotCheck.
func TestOrderSpotCheck_UnknownContractIs404(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: spotCheckStubProvider(`[]`), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/contracts/not_a_real_contract/spot-check",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("order spot-check unknown contract = %d, want 404; body=%s", rec.Code, rec.Body)
	}
	if n := countLLMCalls(t, pool, projectID); n != 0 {
		t.Fatalf("llm_call rows after unknown-contract order = %d, want 0", n)
	}
}

// TestOrderSpotCheck_EmptyStationIsBadRequest — (b) a station with nothing
// to check answers nothing_to_check BEFORE opening the stream, so an empty
// station never costs a model call. Uses a freshly created project (no
// materials, no graph nodes at all) rather than the shared seeded fixture,
// which already has article materials/claim-evidence nodes.
func TestOrderSpotCheck_EmptyStationIsBadRequest(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: spotCheckStubProvider(`[]`), ChatResolver: fakeResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)

	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, withCookie(httptest.NewRequest("POST", "/api/v1/projects",
		strings.NewReader(`{"title":"空项目","prompt":"随便什么题目"}`)), cookie))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create project = %d, want 201; body=%s", createRec.Code, createRec.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("decode created project id: %v — %s", err, createRec.Body)
	}

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+created.ID+"/contracts/evaluate_sources/spot-check",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("order spot-check on empty station = %d, want 400; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"code":"nothing_to_check"`) {
		t.Fatalf("expected nothing_to_check code: %s", rec.Body.String())
	}
	if n := countLLMCalls(t, pool, created.ID); n != 0 {
		t.Fatalf("llm_call rows after empty-station order = %d, want 0", n)
	}
}

// TestOrderSpotCheck_PersistsAndMarksGateSolid — (c)+(d): a successful order
// on evaluate_sources (the seeded demo project has two article materials,
// neither yet evaluated) persists one spot_check_item per target, marks
// source_quality_spot_check solid, and appends exactly one
// spot_check_ordered event. Re-ordering with an UNCHANGED fingerprint then
// returns the same items and makes no second model call (idempotent replay).
func TestOrderSpotCheck_PersistsAndMarksGateSolid(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[` +
		`{"target_id":"00000000-0000-0000-0000-000000000110","evidence":"讲清了这条来源能回答的问题","missing":"没写清它不能回答什么","fix":"补充这条来源的边界"},` +
		`{"target_id":"00000000-0000-0000-0000-000000000111","evidence":"讲清了它在论证里的角色","missing":"缺一次横向核查","fix":"找一条独立信源核对"}` +
		`]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     spotCheckStubProvider(reply),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/contracts/evaluate_sources/spot-check",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("order spot-check = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: review") {
		t.Fatalf("spot-check stream missing the review event: %s", body)
	}
	if !strings.Contains(body, `"target_id":"00000000-0000-0000-0000-000000000110"`) {
		t.Fatalf("spot-check stream missing work order: %s", body)
	}

	if n := countSpotCheckItems(t, pool, projectID); n != 2 {
		t.Fatalf("spot_check_item interventions = %d, want 2", n)
	}
	if n := countSpotCheckOrderedEvents(t, pool, projectID); n != 1 {
		t.Fatalf("spot_check_ordered events = %d, want 1", n)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	states, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates: %v", err)
	}
	if states["evaluate_sources"].Items["source_quality_spot_check"] != "solid" {
		t.Fatalf("source_quality_spot_check = %q, want solid", states["evaluate_sources"].Items["source_quality_spot_check"])
	}
	callsAfterFirst := countLLMCalls(t, pool, projectID)
	if callsAfterFirst == 0 {
		t.Fatal("expected at least one llm_call row after the real order")
	}

	// (d) Re-order with an UNCHANGED fingerprint (nothing about the two
	// materials changed) — same items, no second model call.
	rec2 := httptest.NewRecorder()
	req2 := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/contracts/evaluate_sources/spot-check",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second order spot-check = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}
	body2 := rec2.Body.String()
	if !strings.Contains(body2, `"target_id":"00000000-0000-0000-0000-000000000110"`) {
		t.Fatalf("replayed spot-check stream missing work order: %s", body2)
	}
	if n := countSpotCheckItems(t, pool, projectID); n != 2 {
		t.Fatalf("after replay, spot_check_item interventions = %d, want 2 (idempotent)", n)
	}
	if n := countSpotCheckOrderedEvents(t, pool, projectID); n != 1 {
		t.Fatalf("after replay, spot_check_ordered events = %d, want 1 (no second order)", n)
	}
	if n := countLLMCalls(t, pool, projectID); n != callsAfterFirst {
		t.Fatalf("llm_call rows after replay = %d, want %d (no second model call)", n, callsAfterFirst)
	}
}

// TestOrderSpotCheck_EveryInsertFailsLeavesGateNotSolid — (e): if every
// persist attempt fails, the gate item must NOT be marked solid (a solid
// gate with zero visible items would make the gate and the UI disagree).
// Forces the failure at the real DB layer (failingInterventionDBTX) rather
// than via model rejection, so this exercises the persist loop's OWN
// all-failed branch, distinct from the proposal-rejected path.
func TestOrderSpotCheck_EveryInsertFailsLeavesGateNotSolid(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[` +
		`{"target_id":"00000000-0000-0000-0000-000000000110","evidence":"讲清了这条来源能回答的问题","missing":"没写清它不能回答什么","fix":"补充这条来源的边界"},` +
		`{"target_id":"00000000-0000-0000-0000-000000000111","evidence":"讲清了它在论证里的角色","missing":"缺一次横向核查","fix":"找一条独立信源核对"}` +
		`]`
	h := New(Deps{
		Queries:      sqlc.New(failingInterventionDBTX{pool}),
		Pool:         pool,
		Provider:     spotCheckStubProvider(reply),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/contracts/evaluate_sources/spot-check",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("order spot-check (forced insert failure) = %d, want 200 (SSE, not HTTP error); body=%s", rec.Code, rec.Body)
	}
	// The model call still happened (only the persist loop is forced to
	// fail) — the stream must carry the review event (empty work order, since
	// nothing persisted), NOT an error envelope. This is what M3 pins: a
	// request that failed BEFORE the persist loop (e.g. entitlement/parse
	// rejection) would also leave 0 rows in the DB, so the DB-state
	// assertions alone can't tell "every insert failed" apart from "the
	// request never got that far" — the event kind can.
	if strings.Contains(rec.Body.String(), "event: error") {
		t.Fatalf("forced insert failure streamed an error envelope, want the review event: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "event: review") {
		t.Fatalf("forced insert failure missing the review event: %s", rec.Body.String())
	}
	if n := countLLMCalls(t, pool, projectID); n != 1 {
		t.Fatalf("llm_call rows after forced insert failure = %d, want 1 (the model call happened)", n)
	}

	if n := countSpotCheckItems(t, pool, projectID); n != 0 {
		t.Fatalf("spot_check_item interventions after forced insert failure = %d, want 0", n)
	}
	if n := countSpotCheckOrderedEvents(t, pool, projectID); n != 0 {
		t.Fatalf("spot_check_ordered events after forced insert failure = %d, want 0", n)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	states, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates: %v", err)
	}
	if states["evaluate_sources"].Items["source_quality_spot_check"] == "solid" {
		t.Fatal("source_quality_spot_check marked solid despite every insert failing")
	}
}

// TestOrderSpotCheck_ArgumentPersistsAndMarksGateSolid — finding I1: every
// other test in this file orders evaluate_sources, so
// studio.argumentSpotCheckTargets and the
// agent.SpotCheckArgument -> "warrant_quality_spot_check" arm of the gate-item
// map (spotcheck.go) never ran in any test. The seeded demo project's S4
// graph (migration 0018) has a done "claim" node (...142, first by the
// (created_at, id) tiebreak over the two claim nodes) and an orphan "evidence"
// node (...144) — no warrant/counter/concession — so build_argument's target
// list is exactly [claim, evidence] in slot order. A target's ID is the
// Toulmin SLOT id ("claim"/"evidence"), not the graph node's UUID — that's
// the exact id/node-type coupling I1 warns could silently drift.
func TestOrderSpotCheck_ArgumentPersistsAndMarksGateSolid(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[` +
		`{"target_id":"claim","evidence":"主张写得完整","missing":"还没接到可核查的证据上","fix":"把这条主张连到一条来源"},` +
		`{"target_id":"evidence","evidence":"证据本身具体","missing":"没写清它如何支撑哪条主张","fix":"把这条证据接回一条主张"}` +
		`]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     spotCheckStubProvider(reply),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/contracts/build_argument/spot-check",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("order spot-check (build_argument) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: review") {
		t.Fatalf("spot-check stream missing the review event: %s", body)
	}
	if !strings.Contains(body, `"target_id":"claim"`) ||
		!strings.Contains(body, `"target_id":"evidence"`) {
		t.Fatalf("spot-check stream missing the claim/evidence work order: %s", body)
	}

	if n := countSpotCheckItems(t, pool, projectID); n != 2 {
		t.Fatalf("spot_check_item interventions = %d, want 2", n)
	}
	if n := countSpotCheckOrderedEvents(t, pool, projectID); n != 1 {
		t.Fatalf("spot_check_ordered events = %d, want 1", n)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	states, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates: %v", err)
	}
	if states["build_argument"].Items["warrant_quality_spot_check"] != "solid" {
		t.Fatalf("warrant_quality_spot_check = %q, want solid", states["build_argument"].Items["warrant_quality_spot_check"])
	}
}

// TestOrderSpotCheck_RejectedProposalPersistsNothingButStillMeters — finding
// I2, mirroring writing_test.go's TestOrderReview_RejectedProposalPersistsNothing:
// a banned-phrasing violation in the model's output must persist NOTHING (no
// spot_check_item rows, the gate item NOT marked solid) but the llm_call row
// must still exist — a rejected call still cost money.
func TestOrderSpotCheck_RejectedProposalPersistsNothingButStillMeters(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[{"target_id":"00000000-0000-0000-0000-000000000110","evidence":"e","missing":"m","fix":"你应该这样写：这条来源只能证明局部现象。"}]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     spotCheckStubProvider(reply),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/contracts/evaluate_sources/spot-check",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("order spot-check (rejected) = %d, want 200 (SSE error, not HTTP error); body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "event: error") {
		t.Fatalf("expected an SSE error event: %s", rec.Body.String())
	}
	if n := countSpotCheckItems(t, pool, projectID); n != 0 {
		t.Fatalf("rejected proposal persisted %d spot_check_item interventions, want 0", n)
	}
	if n := countSpotCheckOrderedEvents(t, pool, projectID); n != 0 {
		t.Fatalf("rejected proposal appended %d spot_check_ordered events, want 0", n)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	states, err := store.ListGateStates(context.Background(), mustUUID(projectID))
	if err != nil {
		t.Fatalf("ListGateStates: %v", err)
	}
	if states["evaluate_sources"].Items["source_quality_spot_check"] == "solid" {
		t.Fatal("source_quality_spot_check marked solid despite a rejected proposal")
	}

	// A rejected proposal still cost money: exactly one llm_call row, with
	// Purpose "spot_check" — the distinct-from-order_review purpose that
	// keeps per-station cost legible in llm_usage.
	calls, err := sqlc.New(pool).ListLLMCallsByProject(context.Background(), pgUUID(mustUUID(projectID)))
	if err != nil {
		t.Fatalf("ListLLMCallsByProject: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("llm_call rows after rejected spot-check = %d, want 1 (cost-on-rejection)", len(calls))
	}
	if calls[0].Purpose != "spot_check" {
		t.Fatalf("llm_call purpose = %q, want spot_check", calls[0].Purpose)
	}
}

// TestOrderSpotCheck_TouchesLastActiveAt — finding M1: ordering a spot-check
// is student activity that costs a model call, so it must advance
// project.last_active_at the same way orderReview/RunAgentStep/card submit
// already do (the roster's 最近活跃 column depends on it — see the "exact
// roster lie" comment at projectcards.go:317). Modeled on
// TestProjectCardSubmit_TouchesLastActiveAt.
func TestOrderSpotCheck_TouchesLastActiveAt(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[` +
		`{"target_id":"00000000-0000-0000-0000-000000000110","evidence":"e","missing":"m","fix":"f"},` +
		`{"target_id":"00000000-0000-0000-0000-000000000111","evidence":"e","missing":"m","fix":"f"}` +
		`]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     spotCheckStubProvider(reply),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	projectID := mustUUID(materialsTestProjectID)

	before, err := q.GetProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProject before: %v", err)
	}

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+materialsTestProjectID+"/contracts/evaluate_sources/spot-check",
		strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("order spot-check = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	after, err := q.GetProject(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProject after: %v", err)
	}
	if !after.LastActiveAt.After(before.LastActiveAt) {
		t.Fatalf("last_active_at did not advance: before=%v after=%v", before.LastActiveAt, after.LastActiveAt)
	}
}

// TestOrderSpotCheck_ChangedFingerprintReRuns — finding M2, the converse of
// the (d) unchanged-fingerprint-replay case: SpotCheckFingerprint replaces
// the snapshot id with a content hash specifically so that editing what the
// check reads makes it re-run. Orders evaluate_sources once, then changes
// material 110's risk_note by adding the evaluated-as edge attest.go's CRAAP
// mint would produce, then orders again — a second model call must happen
// and the new fingerprint's items must persist alongside (not instead of) the
// first fingerprint's.
func TestOrderSpotCheck_ChangedFingerprintReRuns(t *testing.T) {
	pool := newAPITestPool(t)
	reply := `[` +
		`{"target_id":"00000000-0000-0000-0000-000000000110","evidence":"e1","missing":"m1","fix":"f1"},` +
		`{"target_id":"00000000-0000-0000-0000-000000000111","evidence":"e2","missing":"m2","fix":"f2"}` +
		`]`
	h := New(Deps{
		Queries:      sqlc.New(pool),
		Pool:         pool,
		Provider:     spotCheckStubProvider(reply),
		ChatResolver: fakeResolver(),
		SpecByID:     cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	q := sqlc.New(pool)
	projectID := materialsTestProjectID

	// First order — baseline.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/contracts/evaluate_sources/spot-check",
		strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("first order = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	callsAfterFirst := countLLMCalls(t, pool, projectID)
	itemsAfterFirst := countSpotCheckItems(t, pool, projectID)
	if callsAfterFirst == 0 || itemsAfterFirst != 2 {
		t.Fatalf("after first order: calls=%d items=%d, want >0 and 2", callsAfterFirst, itemsAfterFirst)
	}

	// Edit what the check reads: material 110 gains a risk_note via the same
	// evaluated-as edge shape attest.go's CRAAP mint produces. This changes
	// sourceSpotCheckTargets' Detail for that material, which changes
	// SpotCheckFingerprint's hash.
	evidenceNode, err := q.InsertGraphNode(context.Background(), sqlc.InsertGraphNodeParams{
		ProjectID: mustUUID(projectID), Type: "evidence", Author: "student",
		Body: []byte(`{"source_quality":{"risk_note":"这条来源只能证明局部现象，不能推广到全国"}}`),
	})
	if err != nil {
		t.Fatalf("insert evidence node: %v", err)
	}
	if _, err := q.InsertGraphEdge(context.Background(), sqlc.InsertGraphEdgeParams{
		ProjectID: mustUUID(projectID), Type: "evaluated-as",
		FromKind: "material", FromID: uuid.MustParse("00000000-0000-0000-0000-000000000110"),
		ToKind: "graph_node", ToID: evidenceNode.ID,
	}); err != nil {
		t.Fatalf("insert evaluated-as edge: %v", err)
	}

	// Second order — the fingerprint changed, so this must call the model
	// again and persist a SECOND set of items (anchored to the new
	// fingerprint), leaving the first fingerprint's items untouched.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("POST",
		"/api/v1/projects/"+projectID+"/contracts/evaluate_sources/spot-check",
		strings.NewReader("")), cookie))
	if rec2.Code != http.StatusOK {
		t.Fatalf("second order = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}
	if n := countLLMCalls(t, pool, projectID); n <= callsAfterFirst {
		t.Fatalf("llm_call rows after changed-fingerprint reorder = %d, want > %d (a second model call)", n, callsAfterFirst)
	}
	if n := countSpotCheckItems(t, pool, projectID); n <= itemsAfterFirst {
		t.Fatalf("spot_check_item rows after changed-fingerprint reorder = %d, want > %d (new items, old ones kept)", n, itemsAfterFirst)
	}
}
