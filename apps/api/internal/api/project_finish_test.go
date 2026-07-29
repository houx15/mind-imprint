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

	markReflectionDone(t, pool, projectID)

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

	// The report DTO now comes from GET /assessment (the read path).
	recAssess := httptest.NewRecorder()
	h.ServeHTTP(recAssess, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/assessment", nil), cookie))
	if recAssess.Code != http.StatusOK {
		t.Fatalf("GET assessment = %d, want 200; body=%s", recAssess.Code, recAssess.Body)
	}
	var dto struct {
		DepthAxis []struct {
			Code  string `json:"code"`
			Level string `json:"level"`
		} `json:"depthAxis"`
		OfficialProjection *struct {
			Readiness struct {
				Score int `json:"score"`
			} `json:"readiness"`
		} `json:"officialProjection"`
		Narrative   string `json:"narrative"`
		GeneratedAt string `json:"generatedAt"`
	}
	if err := json.Unmarshal(recAssess.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode assessment DTO: %v — body=%s", err, recAssess.Body)
	}
	if dto.Narrative == "" || dto.GeneratedAt == "" {
		t.Fatalf("dto = %+v, want narrative + generatedAt", dto)
	}
	if len(dto.DepthAxis) != 6 {
		t.Fatalf("depthAxis len = %d, want 6 (D1-D6 always present)", len(dto.DepthAxis))
	}
	// The project surface is the one ProjectProjection=true surface — the
	// finish DTO must carry the officialProjection superset (chat/course never
	// do, see chat_assessment_test.go/course_assessment_test.go).
	if dto.OfficialProjection == nil {
		t.Fatalf("officialProjection missing on the project finish DTO, want it present (ProjectProjection=true)")
	}
	if dto.OfficialProjection.Readiness.Score != 72 {
		t.Fatalf("officialProjection.readiness.score = %d, want 72 (fixture value, clamped-through)", dto.OfficialProjection.Readiness.Score)
	}

	assertProjectStatus(t, pool, projectID, "finished")

	// The workspace projection now derives status="done" for a finished project.
	recProj := httptest.NewRecorder()
	h.ServeHTTP(recProj, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID, nil), cookie))
	if !strings.Contains(recProj.Body.String(), `"status":"done"`) {
		t.Fatalf("projection status after finish = %s, want done", recProj.Body)
	}

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

	markReflectionDone(t, pool, projectID)

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
		Provider:     assessStubProvider(`{"depthAxis":[{"code":"D1","level":"L2","evidence":"你应该这样写：先摆结论"}],"narrative":"n"}`),
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	projectID := materialsTestProjectID

	markReflectionDone(t, pool, projectID)

	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+projectID+"/finish", strings.NewReader("")), cookie)
	h.ServeHTTP(rec, req)
	// Async: 202 up front; the goroutine rejects the report and reverts status.
	if rec.Code != http.StatusAccepted {
		t.Fatalf("finish (rejected) = %d, want 202; body=%s", rec.Code, rec.Body)
	}
	// The reject path rolls status back to 'active' (retryable).
	waitProjectStatus(t, pool, projectID, "active")
	if n := countProjectEvaluations(t, pool, projectID); n != 0 {
		t.Fatalf("project evaluation rows after rejection = %d, want 0 (nothing persisted)", n)
	}
	if n := countLLMCalls(t, pool, projectID); n != 1 {
		t.Fatalf("llm_call rows after rejection = %d, want 1 — a rejected call still cost money", n)
	}
}
