package api_test

// project_finish_test.go — A3 Task 4: POST /api/v1/projects/{id}/finish, the
// project's one-time terminal. Order: ownership -> entitlement -> one-time
// guard (already finished -> 409) -> gate guard (draft_polish.whole_draft_review
// must be "solid", server-enforced) -> generate the flagship report -> on
// reject 422 assessment_rejected AND the project stays 'active' (retryable)
// -> on success, mark 'finished' + append project_finished, return the DTO.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// assertProjectStatus reads the project row directly and fails the test if
// its status doesn't match want — used to confirm the one-time guard (a
// rejected report leaves the project 'active'; a successful one flips it to
// 'finished').
func assertProjectStatus(t *testing.T, pool *pgxpool.Pool, projectID, want string) {
	t.Helper()
	var got string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM project WHERE id = $1`, projectID).Scan(&got); err != nil {
		t.Fatalf("read project status: %v", err)
	}
	if got != want {
		t.Fatalf("project status = %q, want %q", got, want)
	}
}

// countProjectEvaluations counts evaluation rows scoped to projectID —
// finishProject writes at most one (InsertProjectEvaluation), and the
// already-finished/rejected paths must write none.
func countProjectEvaluations(t *testing.T, pool *pgxpool.Pool, projectID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM evaluations WHERE project_id = $1`, projectID).Scan(&n); err != nil {
		t.Fatalf("count project evaluations: %v", err)
	}
	return n
}

// TestFinishProject_GateNotMet — the gate is enforced server-side: without a
// solid whole_draft_review, finish 422s with gate_not_met, the project stays
// 'active', and no evaluation row is written.
func TestFinishProject_GateNotMet(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(assessReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/finish", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("finish (gate unmet) = %d, want 422; body=%s", rec.Code, rec.Body)
	}
	var perr struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &perr); err != nil || perr.Error.Code != "gate_not_met" {
		t.Fatalf("finish (gate unmet) code = %+v (err=%v), want gate_not_met; body=%s", perr, err, rec.Body)
	}
	assertProjectStatus(t, pool, projectID, "active")
	if n := countLLMCalls(t, pool, projectID); n != 0 {
		t.Fatalf("llm_call rows after gate-unmet finish = %d, want 0 (never reached generation)", n)
	}
}

// TestFinishProject_SuccessMarksFinishedAndPersistsFlagshipReport — the happy
// path: gate solid, provider returns a valid report -> 200 with the
// DualAxis ReportDTO; project.status becomes 'finished'; exactly one flagship
// evaluation persisted; a project_finished event exists.
func TestFinishProject_SuccessMarksFinishedAndPersistsFlagshipReport(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	if err := store.UpsertGateState(context.Background(), mustUUID(projectID), "draft_polish", agent.RecordedGate{
		Confirmed: true,
		Items:     map[string]string{"whole_draft_review": "solid"},
	}); err != nil {
		t.Fatalf("UpsertGateState(whole_draft_review): %v", err)
	}

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/finish", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("finish (success) = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var dto struct {
		DepthAxis struct {
			Subtotal int `json:"subtotal"`
		} `json:"depthAxis"`
		Narrative   string `json:"narrative"`
		GeneratedAt string `json:"generatedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode finish DTO: %v — body=%s", err, rec.Body)
	}
	if dto.Narrative == "" || dto.GeneratedAt == "" {
		t.Fatalf("dto = %+v, want narrative + generatedAt", dto)
	}
	if dto.DepthAxis.Subtotal != 11 {
		t.Fatalf("depthAxis.subtotal = %d, want 11", dto.DepthAxis.Subtotal)
	}

	assertProjectStatus(t, pool, projectID, "finished")

	row, err := sqlc.New(pool).GetLatestProjectEvaluation(context.Background(), pgUUID(mustUUID(projectID)))
	if err != nil {
		t.Fatalf("GetLatestProjectEvaluation: %v", err)
	}
	if row.Tier != "flagship" {
		t.Fatalf("persisted evaluation tier = %q, want flagship (评估走旗舰模型绝不降级)", row.Tier)
	}
	if n := countProjectEvaluations(t, pool, projectID); n != 1 {
		t.Fatalf("project evaluation rows = %d, want exactly 1", n)
	}

	events, err := sqlc.New(pool).ListEventsByProject(context.Background(), pgUUID(mustUUID(projectID)))
	if err != nil {
		t.Fatalf("ListEventsByProject: %v", err)
	}
	found := false
	for _, e := range events {
		if e.Type == "project_finished" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no project_finished event found among %d events", len(events))
	}
}

// TestFinishProject_AlreadyFinished — a second finish call on an
// already-finished project 409s with no second evaluation row (one-time;
// DEC-A3.5, no regeneration).
func TestFinishProject_AlreadyFinished(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	if err := store.UpsertGateState(context.Background(), mustUUID(projectID), "draft_polish", agent.RecordedGate{
		Confirmed: true,
		Items:     map[string]string{"whole_draft_review": "solid"},
	}); err != nil {
		t.Fatalf("UpsertGateState(whole_draft_review): %v", err)
	}

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/finish", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first finish = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	rec2 := httptest.NewRecorder()
	req2 := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/finish", strings.NewReader("")), cookie)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("second finish = %d, want 409; body=%s", rec2.Code, rec2.Body)
	}
	var perr struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &perr); err != nil || perr.Error.Code != "already_finished" {
		t.Fatalf("second finish code = %+v (err=%v), want already_finished; body=%s", perr, err, rec2.Body)
	}
	if n := countProjectEvaluations(t, pool, projectID); n != 1 {
		t.Fatalf("project evaluation rows after 2nd finish = %d, want still 1 (no regeneration)", n)
	}
}

// TestFinishProject_RejectedAssessmentKeepsProjectActive — gate solid but the
// provider's output fails enforcement (banned_phrasing's "你应该这样写" rule,
// same fixture chat_assessment_test.go uses to force the reject path): 422
// assessment_rejected, the project stays 'active' (retryable), and an
// llm_call cost row was still recorded (a rejected call still cost money).
func TestFinishProject_RejectedAssessmentKeepsProjectActive(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     assessStubProvider(`{"depthAxis":{"dims":[{"code":"D1","score":2,"evidence":"你应该这样写：先摆结论"}]},"autonomyAxis":{"observation":"o"},"crossAxis":{"depthLevel":"L2"},"narrative":"n"}`),
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	if err := store.UpsertGateState(context.Background(), mustUUID(projectID), "draft_polish", agent.RecordedGate{
		Confirmed: true,
		Items:     map[string]string{"whole_draft_review": "solid"},
	}); err != nil {
		t.Fatalf("UpsertGateState(whole_draft_review): %v", err)
	}

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/finish", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("finish (rejected) = %d, want 422; body=%s", rec.Code, rec.Body)
	}
	var perr struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &perr); err != nil || perr.Error.Code != "assessment_rejected" {
		t.Fatalf("finish (rejected) code = %+v (err=%v), want assessment_rejected; body=%s", perr, err, rec.Body)
	}
	assertProjectStatus(t, pool, projectID, "active")
	if n := countProjectEvaluations(t, pool, projectID); n != 0 {
		t.Fatalf("project evaluation rows after rejection = %d, want 0 (nothing persisted)", n)
	}
	if n := countLLMCalls(t, pool, projectID); n != 1 {
		t.Fatalf("llm_call rows after rejection = %d, want 1 — a rejected call still cost money", n)
	}
}
