package api_test

// project_finish_test.go — A3 Task 4: POST /api/v1/projects/{id}/finish, the
// project's one-time terminal. Order: ownership -> entitlement -> one-time
// guard (already finished/evaluating -> 409) -> gate guards (writing finished,
// reflection done, both server-enforced) -> claim 'evaluating' + 202 response
// -> a detached goroutine generates the report and, on success, marks
// 'finished' + appends project_finished; on any error it reverts to 'active'
// (retryable). As of Task 6 (evaluation-report-pipeline), the goroutine's
// artifact is the new EvaluationReport (generateAndStoreEvaluationReport,
// currently evalreport.Placeholder) — the old dual-axis evaluation + mirror
// pipeline has been retired entirely (2026-08-14).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

// markReflectionDone upserts a done=true reflection so the finish gate (Slice
// 5: 完成回顾 before archive) is satisfied. The old draft_polish gate is gone.
func markReflectionDone(t *testing.T, pool *pgxpool.Pool, projectID string) {
	t.Helper()
	if _, err := sqlc.New(pool).UpsertProjectReflection(context.Background(), sqlc.UpsertProjectReflectionParams{
		ProjectID: mustUUID(projectID),
		Answers:   []byte(`["这次我把论点收窄到国内新能源投资"]`),
		Done:      true,
	}); err != nil {
		t.Fatalf("mark reflection done: %v", err)
	}
}

// markWritingFinished sets the 完成写作 milestone (Slice 5 · #20) so the finish
// gate (writing must be finished before finalize) is satisfied.
func markWritingFinished(t *testing.T, pool *pgxpool.Pool, projectID string) {
	t.Helper()
	if err := sqlc.New(pool).SetWritingFinish(context.Background(), sqlc.SetWritingFinishParams{ProjectID: mustUUID(projectID), DocKind: "essay"}); err != nil {
		t.Fatalf("mark writing finished: %v", err)
	}
}

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

// waitProjectStatus polls the project row until its status equals want, or
// fails after ~5s. Finish is now async (BE5): it returns 202 and a detached
// goroutine drives status evaluating→finished (success) or evaluating→active
// (reject/failure), so tests wait for the terminal state rather than reading it
// off the finish response.
func waitProjectStatus(t *testing.T, pool *pgxpool.Pool, projectID, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		if err := pool.QueryRow(context.Background(),
			`SELECT status FROM project WHERE id = $1`, projectID).Scan(&got); err != nil {
			t.Fatalf("read project status: %v", err)
		}
		if got == want {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("project status = %q after 5s, want %q", got, want)
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

// countEvaluationReports counts evaluation_report rows scoped to projectID —
// the finish worker now writes the new EvaluationReport artifact (via
// generateAndStoreEvaluationReport) instead of the old dual-axis evaluation +
// mirror; a successful finish writes exactly one.
func countEvaluationReports(t *testing.T, pool *pgxpool.Pool, projectID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM evaluation_report WHERE project_id = $1`, projectID).Scan(&n); err != nil {
		t.Fatalf("count evaluation_report rows: %v", err)
	}
	return n
}

// TestFinishProject_ReflectionNotDone — the gate is enforced server-side:
// without a done=true reflection, finish 422s with reflection_not_done, the
// project stays 'active', and no evaluation row is written.
func TestFinishProject_ReflectionNotDone(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(assessReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	// Satisfy the writing gate so the reflection gate is the one that fires.
	markWritingFinished(t, pool, projectID)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/finish", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("finish (reflection not done) = %d, want 422; body=%s", rec.Code, rec.Body)
	}
	var perr struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &perr); err != nil || perr.Error.Code != "reflection_not_done" {
		t.Fatalf("finish (reflection not done) code = %+v (err=%v), want reflection_not_done; body=%s", perr, err, rec.Body)
	}
	assertProjectStatus(t, pool, projectID, "active")
	if n := countLLMCalls(t, pool, projectID); n != 0 {
		t.Fatalf("llm_call rows after reflection-not-done finish = %d, want 0 (never reached generation)", n)
	}
}

// TestFinishProject_SuccessMarksFinishedAndPersistsEvaluationReport — the
// happy path: gate solid -> 202 evaluating -> the detached goroutine calls
// generateAndStoreEvaluationReport (Task 6); project.status becomes
// 'finished'; exactly one evaluation_report row persisted; GET
// /evaluation-report returns it; a project_finished event exists. (Task 6
// retired the old dual-axis evaluation + mirror from the finish path — the
// finish artifact is now the new EvaluationReport.)
func TestFinishProject_SuccessMarksFinishedAndPersistsEvaluationReport(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	markReflectionDone(t, pool, projectID)
	markWritingFinished(t, pool, projectID)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/finish", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	// Finish is now async (BE5): 202 evaluating, then a goroutine finishes it.
	if rec.Code != http.StatusAccepted {
		t.Fatalf("finish (success) = %d, want 202; body=%s", rec.Code, rec.Body)
	}
	var acc struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &acc); err != nil || acc.Status != "evaluating" {
		t.Fatalf("finish 202 body = %s, want {status:evaluating}", rec.Body)
	}

	// Drive the goroutine to completion.
	waitProjectStatus(t, pool, projectID, "finished")

	// The finish artifact now comes from GET /evaluation-report (the read path).
	recRep := httptest.NewRecorder()
	h.ServeHTTP(recRep, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/evaluation-report", nil), cookie))
	if recRep.Code != http.StatusOK {
		t.Fatalf("GET evaluation-report = %d, want 200; body=%s", recRep.Code, recRep.Body)
	}
	rep := decodeReadyEnvelope(t, recRep.Body.Bytes())
	if rep.ProjectID != projectID {
		t.Fatalf("report.ProjectID = %q, want %q", rep.ProjectID, projectID)
	}
	if rep.GeneratedAt == "" {
		t.Fatalf("report = %+v, want non-empty generatedAt", rep)
	}
	if len(rep.Depth) != 6 {
		t.Fatalf("report.Depth len = %d, want 6 (D1-D6 always present)", len(rep.Depth))
	}

	assertProjectStatus(t, pool, projectID, "finished")

	// The workspace projection now derives status="done" for a finished project.
	recProj := httptest.NewRecorder()
	h.ServeHTTP(recProj, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID, nil), cookie))
	if !strings.Contains(recProj.Body.String(), `"status":"done"`) {
		t.Fatalf("projection status after finish = %s, want done", recProj.Body)
	}

	if n := countEvaluationReports(t, pool, projectID); n != 1 {
		t.Fatalf("evaluation_report rows = %d, want exactly 1", n)
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

	markReflectionDone(t, pool, projectID)
	markWritingFinished(t, pool, projectID)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/finish", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("first finish = %d, want 202; body=%s", rec.Code, rec.Body)
	}
	// Let the goroutine drive it to finished before the second finish.
	waitProjectStatus(t, pool, projectID, "finished")

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
	if n := countEvaluationReports(t, pool, projectID); n != 1 {
		t.Fatalf("evaluation_report rows after 2nd finish = %d, want still 1 (no regeneration)", n)
	}
}

// NOTE (Task 6): TestFinishProject_RejectedAssessmentKeepsProjectActive was
// removed here. It exercised the old dual-axis assessment-generation reject
// path (a provider output failing enforcement, e.g. banned_phrasing). The
// finish worker no longer calls that pipeline — it calls
// generateAndStoreEvaluationReport (evalreport.Placeholder), which has no
// reject path (a deterministic fixture, no model call to reject). The
// rollback-to-active-on-error branch in runProjectReport is still preserved
// in code (see project_finish.go) but is currently unreachable from this
// test file without a fault-injection seam into Queries/InsertEvaluationReport,
// which does not exist today. A later task adding real report generation
// (with an actual reject/error path) should add a replacement test here.
