package api_test

// workspace_review_test.go — Slice 5 (Review room): the student's own
// five-dimension reflection doc (plain REST, no model call). Also covers the
// rewired finish gate: reflection.done must be true before a project
// archives, after which GET /evaluation-report returns the generated report.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/store/sqlc"
)

type reflectionWire struct {
	Answers []string `json:"answers"`
	Done    bool     `json:"done"`
}

func reviewTestHandler(t *testing.T) (http.Handler, *http.Cookie, *pgxpool.Pool) {
	t.Helper()
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool) // Phoebe, owns materialsTestProjectID
	return h, cookie, pool
}

func putReflection(t *testing.T, h http.Handler, cookie *http.Cookie, projectID, body string) reflectionWire {
	t.Helper()
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("PUT", "/api/v1/projects/"+projectID+"/reflection-doc", strings.NewReader(body)), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT reflection-doc = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out reflectionWire
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode PUT reflection: %v — %s", err, rec.Body)
	}
	return out
}

func getReflection(t *testing.T, h http.Handler, cookie *http.Cookie, projectID string) reflectionWire {
	t.Helper()
	rec := httptest.NewRecorder()
	req := withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+projectID+"/reflection-doc", nil), cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET reflection-doc = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out reflectionWire
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode GET reflection: %v — %s", err, rec.Body)
	}
	return out
}

func countLogByText(t *testing.T, pool *pgxpool.Pool, projectID, text string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM activity_log_entry WHERE project_id = $1 AND text = $2`, projectID, text).Scan(&n); err != nil {
		t.Fatalf("count activity log: %v", err)
	}
	return n
}

// TestReflection_RoundTripAndDoneFlip — PUT persists answers and GET reads them
// back; done defaults false; an answers-only PUT preserves done; flipping done
// true drops exactly one 完成回顾 auto-log line (fired once, on the transition).
func TestReflection_RoundTripAndDoneFlip(t *testing.T) {
	h, cookie, pool := reviewTestHandler(t)
	pid := materialsTestProjectID

	// GET on a fresh project → empty state.
	if got := getReflection(t, h, cookie, pid); len(got.Answers) != 0 || got.Done {
		t.Fatalf("fresh reflection = %+v, want empty answers + done=false", got)
	}

	// PUT answers only → persisted, done stays false.
	put := putReflection(t, h, cookie, pid, `{"answers":["目标达成了","读得最深的是碳排放数据"]}`)
	if len(put.Answers) != 2 || put.Done {
		t.Fatalf("PUT answers-only = %+v, want 2 answers + done=false", put)
	}
	if got := getReflection(t, h, cookie, pid); len(got.Answers) != 2 || got.Answers[0] != "目标达成了" || got.Done {
		t.Fatalf("GET after answers PUT = %+v, want the 2 answers + done=false", got)
	}

	// Flip done → true: answers preserved, done true, one auto-log line.
	put2 := putReflection(t, h, cookie, pid, `{"answers":["目标达成了","读得最深的是碳排放数据","下次先收窄论点"],"done":true}`)
	if len(put2.Answers) != 3 || !put2.Done {
		t.Fatalf("PUT done=true = %+v, want 3 answers + done=true", put2)
	}
	if n := countLogByText(t, pool, pid, "完成回顾"); n != 1 {
		t.Fatalf("完成回顾 auto-log rows = %d, want exactly 1", n)
	}

	// An answers-only PUT after done=true must NOT un-finish, and must NOT
	// re-fire the auto-log.
	put3 := putReflection(t, h, cookie, pid, `{"answers":["改了一个字"]}`)
	if !put3.Done {
		t.Fatalf("answers-only PUT after done=true flipped done to %v, want preserved true", put3.Done)
	}
	if n := countLogByText(t, pool, pid, "完成回顾"); n != 1 {
		t.Fatalf("完成回顾 auto-log rows after re-PUT = %d, want still 1 (fired once)", n)
	}
}

// TestFinishGate_ReflectionDoneThenAssessment — the rewired finish gate: finish
// is blocked (422 reflection_not_done) until the reflection is marked done via
// PUT /reflection-doc, after which finish succeeds, an evaluation_report row is
// persisted (Task 6: the finish worker now generates the new EvaluationReport,
// not the old dual-axis evaluation), and GET /evaluation-report returns it.
func TestFinishGate_ReflectionDoneThenAssessment(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(dualAxisReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	pid := materialsTestProjectID

	// Satisfy the #20 writing gate so the reflection gate is the one under test.
	markWritingFinished(t, pool, pid)

	// Finish before reflection done → 422 reflection_not_done.
	recBlocked := httptest.NewRecorder()
	h.ServeHTTP(recBlocked, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/finish", strings.NewReader("")), cookie))
	if recBlocked.Code != http.StatusUnprocessableEntity {
		t.Fatalf("finish (reflection not done) = %d, want 422; body=%s", recBlocked.Code, recBlocked.Body)
	}
	var perr struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recBlocked.Body.Bytes(), &perr); err != nil || perr.Error.Code != "reflection_not_done" {
		t.Fatalf("blocked finish code = %+v (err=%v), want reflection_not_done", perr, err)
	}

	// Mark reflection done through the real handler.
	putReflection(t, h, cookie, pid, `{"answers":["把论点收窄到国内新能源投资"],"done":true}`)

	// Finish now succeeds — async: 202 then a goroutine finishes it.
	recOK := httptest.NewRecorder()
	h.ServeHTTP(recOK, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/finish", strings.NewReader("")), cookie))
	if recOK.Code != http.StatusAccepted {
		t.Fatalf("finish (reflection done) = %d, want 202; body=%s", recOK.Code, recOK.Body)
	}
	waitProjectStatus(t, pool, pid, "finished")
	if n := countEvaluationReports(t, pool, pid); n != 1 {
		t.Fatalf("evaluation_report rows after finish = %d, want 1", n)
	}

	// GET /evaluation-report now returns the generated report.
	recRep := httptest.NewRecorder()
	h.ServeHTTP(recRep, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+pid+"/evaluation-report", nil), cookie))
	if recRep.Code != http.StatusOK {
		t.Fatalf("GET evaluation-report = %d, want 200; body=%s", recRep.Code, recRep.Body)
	}
	rep := decodeReadyEnvelope(t, recRep.Body.Bytes())
	if rep.ProjectID != pid || rep.GeneratedAt == "" {
		t.Fatalf("evaluation report = %+v, want projectId=%s + non-empty generatedAt", rep, pid)
	}
}

// TestWorkspaceReview_Ownership404 — the Slice 5 routes hide a foreign (here
// non-existent) project as not-found rather than leaking its existence.
func TestWorkspaceReview_Ownership404(t *testing.T) {
	h, cookie, _ := reviewTestHandler(t)
	foreign := "/api/v1/projects/00000000-0000-0000-0000-0000000009ff"

	cases := []struct {
		method, path, body string
	}{
		{"GET", foreign + "/reflection-doc", ""},
		{"PUT", foreign + "/reflection-doc", `{"answers":["x"],"done":true}`},
	}
	for _, c := range cases {
		var r *http.Request
		if c.body == "" {
			r = httptest.NewRequest(c.method, c.path, nil)
		} else {
			r = httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, withCookie(r, cookie))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("%s %s: want 404, got %d — %s", c.method, c.path, rr.Code, rr.Body.String())
		}
	}
}
