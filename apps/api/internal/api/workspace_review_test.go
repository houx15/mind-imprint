package api_test

// workspace_review_test.go — Slice 5 (Review room): the student's own
// five-dimension reflection doc (plain REST, no model call) and the 你的思维印记
// mirror (POST composes once via the flagship EvalResolver, first-open-wins; a
// second POST does not re-spend). Also covers the rewired finish gate:
// reflection.done must be true before a project archives, after which
// GET /assessment returns the generated growth report.

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
	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/store/sqlc"
)

// mirrorReply is a valid ComposeMirror JSON reply (agent.Mirror wire shape).
const mirrorReply = `{"sections":[
  {"title":"论点是怎么长出来的","body":"你一开始想笼统地谈可持续，后来把它收窄到国内新能源投资——这一步收窄是你自己完成的。"},
  {"title":"阅读怎么喂给写作","body":"NASA 与 Nature Sustainability 两条一手源真的进了你的论证，而不是堆在参考文献里。"}],
  "carryForwards":["下一次动笔前，先把「我到底想论证什么」写成一句话。","读到关键材料时，随手记下它能接上你论证的哪一步。"]}`

type reflectionWire struct {
	Answers []string `json:"answers"`
	Done    bool     `json:"done"`
}

type mirrorWire struct {
	Sections []struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"sections"`
	CarryForwards []string `json:"carryForwards"`
}

func reviewTestHandler(t *testing.T) (http.Handler, *http.Cookie, *pgxpool.Pool) {
	t.Helper()
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(mirrorReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
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

// TestMirror_FirstOpenWins — the first POST composes once via the flagship
// provider and stores it; GET returns that stored mirror; a SECOND POST returns
// the same stored mirror WITHOUT a second model call (exactly one llm_call
// purpose=mirror across both POSTs).
func TestMirror_FirstOpenWins(t *testing.T) {
	h, cookie, pool := reviewTestHandler(t)
	pid := materialsTestProjectID

	// GET before any compose → JSON null.
	recNull := httptest.NewRecorder()
	h.ServeHTTP(recNull, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+pid+"/mirror", nil), cookie))
	if recNull.Code != http.StatusOK {
		t.Fatalf("GET mirror (none) = %d, want 200; body=%s", recNull.Code, recNull.Body)
	}
	if strings.TrimSpace(recNull.Body.String()) != "null" {
		t.Fatalf("GET mirror (none) body = %q, want JSON null", recNull.Body.String())
	}

	// First POST → composes + stores.
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/mirror", strings.NewReader("")), cookie))
	if rec1.Code != http.StatusOK {
		t.Fatalf("POST mirror (first) = %d, want 200; body=%s", rec1.Code, rec1.Body)
	}
	var m1 mirrorWire
	if err := json.Unmarshal(rec1.Body.Bytes(), &m1); err != nil {
		t.Fatalf("decode first mirror: %v — %s", err, rec1.Body)
	}
	if len(m1.Sections) != 2 || m1.Sections[0].Title == "" || m1.Sections[0].Body == "" {
		t.Fatalf("first mirror sections = %+v, want 2 titled sections", m1.Sections)
	}
	if len(m1.CarryForwards) != 2 {
		t.Fatalf("first mirror carryForwards = %+v, want 2", m1.CarryForwards)
	}
	if n := countLLMCallsByPurpose(t, pool, pid, "mirror"); n != 1 {
		t.Fatalf("mirror llm_call rows after first POST = %d, want 1", n)
	}

	// GET → returns the stored mirror.
	recGet := httptest.NewRecorder()
	h.ServeHTTP(recGet, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+pid+"/mirror", nil), cookie))
	if recGet.Code != http.StatusOK {
		t.Fatalf("GET mirror = %d, want 200; body=%s", recGet.Code, recGet.Body)
	}
	var mg mirrorWire
	if err := json.Unmarshal(recGet.Body.Bytes(), &mg); err != nil {
		t.Fatalf("decode GET mirror: %v — %s", err, recGet.Body)
	}
	if len(mg.Sections) != 2 || mg.Sections[0].Title != m1.Sections[0].Title {
		t.Fatalf("GET mirror = %+v, want the stored first-composed mirror", mg.Sections)
	}

	// Second POST → no re-spend (first-open-wins).
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/mirror", strings.NewReader("")), cookie))
	if rec2.Code != http.StatusOK {
		t.Fatalf("POST mirror (second) = %d, want 200; body=%s", rec2.Code, rec2.Body)
	}
	if n := countLLMCallsByPurpose(t, pool, pid, "mirror"); n != 1 {
		t.Fatalf("mirror llm_call rows after second POST = %d, want still 1 (first-open-wins)", n)
	}
}

// TestMirror_FailedComposeNotPersisted — BE4: when the composer fails (model
// returns non-JSON), POST /mirror returns a graceful minimal mirror but stores
// NOTHING (GET is still null), so a later POST — once the model is usable —
// composes and stores the real mirror instead of being stuck on canned text.
func TestMirror_FailedComposeNotPersisted(t *testing.T) {
	pool := newAPITestPool(t)
	cookie := signInSeed(t, pool)
	pid := materialsTestProjectID

	// First: a provider whose reply is NOT valid mirror JSON → compose fails.
	hBad := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider("这不是 JSON，只是随口一句。"), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()

	recBad := httptest.NewRecorder()
	hBad.ServeHTTP(recBad, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/mirror", strings.NewReader("")), cookie))
	if recBad.Code != http.StatusOK {
		t.Fatalf("POST mirror (failed compose) = %d, want 200 (graceful minimal); body=%s", recBad.Code, recBad.Body)
	}
	// It returned a minimal mirror but persisted NOTHING — GET is still null.
	recGet := httptest.NewRecorder()
	hBad.ServeHTTP(recGet, withCookie(httptest.NewRequest("GET", "/api/v1/projects/"+pid+"/mirror", nil), cookie))
	if strings.TrimSpace(recGet.Body.String()) != "null" {
		t.Fatalf("GET mirror after failed compose = %q, want null (nothing persisted)", recGet.Body.String())
	}
	var stored int
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM project_mirror_prose WHERE project_id = $1`, pid).Scan(&stored)
	if stored != 0 {
		t.Fatalf("project_mirror rows after failed compose = %d, want 0 (BE4: never persist the fallback)", stored)
	}

	// Now a working provider: the retry composes and stores the real mirror.
	hGood := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(mirrorReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	recGood := httptest.NewRecorder()
	hGood.ServeHTTP(recGood, withCookie(httptest.NewRequest("POST", "/api/v1/projects/"+pid+"/mirror", strings.NewReader("")), cookie))
	if recGood.Code != http.StatusOK {
		t.Fatalf("POST mirror (retry) = %d, want 200; body=%s", recGood.Code, recGood.Body)
	}
	var m mirrorWire
	if err := json.Unmarshal(recGood.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode retry mirror: %v — %s", err, recGood.Body)
	}
	if len(m.Sections) != 2 {
		t.Fatalf("retry mirror sections = %d, want 2 (real compose)", len(m.Sections))
	}
	_ = pool.QueryRow(context.Background(), `SELECT count(*) FROM project_mirror_prose WHERE project_id = $1`, pid).Scan(&stored)
	if stored != 1 {
		t.Fatalf("project_mirror rows after successful retry = %d, want 1", stored)
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
	var rep evalreport.Report
	if err := json.Unmarshal(recRep.Body.Bytes(), &rep); err != nil {
		t.Fatalf("decode evaluation report: %v — %s", err, recRep.Body)
	}
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
		{"GET", foreign + "/mirror", ""},
		{"POST", foreign + "/mirror", ""},
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
