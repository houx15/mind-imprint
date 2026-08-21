package api_test

// course_list_progress_test.go — GET /api/v1/courses carries each card's own
// progress, so the 课程 list is ONE round trip instead of an N+1 of per-course
// /progress calls. Covers both storages (legacy course_progress and the 2.0
// course_session), the untouched-course null, and the per-student scoping.

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
	"mindimprint/api/internal/store/sqlc"
)

type listedCourse struct {
	Slug      string `json:"slug"`
	StepCount int    `json:"step_count"`
	Progress  *struct {
		Status         string `json:"status"`
		CompletedSteps int    `json:"completedSteps"`
		UpdatedAt      string `json:"updatedAt"`
	} `json:"progress"`
}

func listCoursesAs(t *testing.T, h http.Handler, cookie *http.Cookie) map[string]listedCourse {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("list courses: want 200, got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Courses []listedCourse `json:"courses"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode course list: %v", err)
	}
	out := make(map[string]listedCourse, len(resp.Courses))
	for _, c := range resp.Courses {
		out[c.Slug] = c
	}
	return out
}

// seedLegacyCourse upserts a legacy (pre-2.0) course with `steps` authored
// steps, so SaveProgress has a real denominator to complete against.
func seedLegacyCourse(t *testing.T, pool *pgxpool.Pool, slug string, steps int) {
	t.Helper()
	structure := `{"steps":[` + strings.Repeat(`{},`, steps)
	structure = strings.TrimSuffix(structure, ",") + `]}`
	st := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	if err := st.UpsertCourse(context.Background(), agent.UpsertCourseInput{
		Slug: slug, Branch: "Legacy", Title: "Legacy Course", Blurb: "b", TimeLabel: "约 5 分钟",
		CardIDs: []string{}, Structure: []byte(structure), RenderCache: []byte("{}"), StepCount: steps,
	}); err != nil {
		t.Fatalf("upsert legacy course %s: %v", slug, err)
	}
}

func putProgress(t *testing.T, h http.Handler, cookie *http.Cookie, slug, body string) {
	t.Helper()
	req := httptest.NewRequest("PUT", "/api/v1/courses/"+slug+"/progress", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(req, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("put progress %s: want 200, got %d %s", slug, rec.Code, rec.Body)
	}
}

// The list reports a legacy course's progress, and leaves an untouched course's
// `progress` null — the distinction the catalog draws 未开始 from.
func TestCourseListCarriesLegacyProgressAndNullForUntouched(t *testing.T) {
	pool := newAPITestPool(t)
	seedLegacyCourse(t, pool, "listprog-touched", 4)
	seedLegacyCourse(t, pool, "listprog-untouched", 4)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	putProgress(t, h, cookie, "listprog-touched", `{"current_ordinal":1,"completed_ordinal":0}`)

	got := listCoursesAs(t, h, cookie)
	touched, ok := got["listprog-touched"]
	if !ok {
		t.Fatalf("touched course missing from the list: %+v", got)
	}
	if touched.Progress == nil {
		t.Fatalf("touched course has null progress — the catalog would read it as 未开始")
	}
	if touched.Progress.CompletedSteps != 1 || touched.Progress.Status != "in-progress" {
		t.Fatalf("progress = %+v, want 1 step / in-progress", *touched.Progress)
	}
	if touched.Progress.UpdatedAt == "" {
		t.Fatalf("progress.updatedAt is empty — the list could not order by recency")
	}
	if untouched := got["listprog-untouched"]; untouched.Progress != nil {
		t.Fatalf("untouched course carries progress %+v, want null", *untouched.Progress)
	}
}

// A finished legacy course reports 'completed'.
func TestCourseListReportsCompletion(t *testing.T) {
	pool := newAPITestPool(t)
	seedLegacyCourse(t, pool, "listprog-done", 2)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	putProgress(t, h, cookie, "listprog-done", `{"current_ordinal":0,"completed_ordinal":0}`)
	putProgress(t, h, cookie, "listprog-done", `{"current_ordinal":1,"completed_ordinal":1}`)

	got := listCoursesAs(t, h, cookie)["listprog-done"]
	if got.Progress == nil || got.Progress.Status != "completed" {
		t.Fatalf("progress = %+v, want completed", got.Progress)
	}
	if got.Progress.CompletedSteps != got.StepCount {
		t.Fatalf("completedSteps = %d, want step_count %d", got.Progress.CompletedSteps, got.StepCount)
	}
}

// A 2.0 course's progress lives in course_session, not course_progress — the
// list must read it from there (the same preference GetProgress applies), or
// every runtime course shows a stuck 0.
func TestCourseListReadsRuntimeSessionProgress(t *testing.T) {
	pool := newAPITestPool(t)
	slug := "listprog-runtime"
	seedCourseWithDefinition(t, pool, slug, testCourseDefinitionJSON)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	// Create the session through the API (get-or-create), then snapshot a
	// session blob with two completed slices — the shape the runtime persists.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+slug+"/session", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("create session: want 200, got %d %s", rec.Code, rec.Body)
	}
	var created struct {
		Session map[string]any `json:"session"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	created.Session["sliceStates"] = map[string]any{
		"s1": map[string]any{"status": "completed"},
		"s2": map[string]any{"status": "completed"},
		"s3": map[string]any{"status": "in-progress"},
	}
	body, _ := json.Marshal(map[string]any{"session": created.Session})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/courses/"+slug+"/session", strings.NewReader(string(body))), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("save session: want 200, got %d %s", rec.Code, rec.Body)
	}

	// step_count is 0 for this bare seed and the query CLAMPS the completed
	// count to it, so raise it to a realistic authored count first.
	if _, err := pool.Exec(context.Background(), "UPDATE course SET step_count = 5 WHERE slug = $1", slug); err != nil {
		t.Fatalf("set step count: %v", err)
	}

	got := listCoursesAs(t, h, cookie)[slug]
	if got.Progress == nil {
		t.Fatalf("runtime course has null progress — session progress was not read")
	}
	if got.Progress.CompletedSteps != 2 || got.Progress.Status != "in-progress" {
		t.Fatalf("progress = %+v, want 2 steps / in-progress", *got.Progress)
	}
}

// Progress is the STUDENT's, never the course's: a second student sees their
// own state on the same catalog.
func TestCourseListProgressIsPerStudent(t *testing.T) {
	pool := newAPITestPool(t)
	seedLegacyCourse(t, pool, "listprog-scoped", 3)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	mine := signInSeed(t, pool)
	putProgress(t, h, mine, "listprog-scoped", `{"current_ordinal":1,"completed_ordinal":0}`)

	theirs := signInAdmin(t, pool) // a different account on the same catalog
	if got := listCoursesAs(t, h, theirs)["listprog-scoped"]; got.Progress != nil {
		t.Fatalf("another account sees my progress: %+v", *got.Progress)
	}
	if got := listCoursesAs(t, h, mine)["listprog-scoped"]; got.Progress == nil {
		t.Fatalf("my own progress went missing")
	}
}
