package api_test

// course_test.go — Task 5's httptest suite for the course v2 handlers
// (list/payload/progress/quiz-answer/report), exercised against a course
// seeded from the real embedded a-mid content (courses.FS) — the same
// fixture Task 4's agent/coursestore_test.go uses. Replaces the pre-course-v2
// suite (course-id + step-table era, retired by migration 0050 / Task 3-4).

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	courses "mindimprint/api/internal/store/seed/courses"
	"mindimprint/api/internal/store/sqlc"
)

// seedAMidCourse upserts the real a-mid course (structure + render cache),
// slug "a-mid", 4 steps / 9 authored quiz interactions, card_ids=[craap] —
// so these handler tests exercise real content, not a synthetic fixture.
func seedAMidCourse(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	q := sqlc.New(pool)
	st := agent.NewSqlcAgentStore(q, pool)
	raw, err := courses.FS.ReadFile("a-mid.json")
	if err != nil {
		t.Fatalf("read a-mid.json: %v", err)
	}
	rc, err := courses.FS.ReadFile("a-mid-render-cache.json")
	if err != nil {
		t.Fatalf("read a-mid-render-cache.json: %v", err)
	}
	if err := st.UpsertCourse(context.Background(), agent.UpsertCourseInput{
		Slug: "a-mid", Branch: "A", Title: "CRRAAB 信源评估：从机构到亲历者到专家",
		Blurb: "从机构到亲历者到专家的信源评估之旅", TimeLabel: "20 分钟",
		CardIDs: []string{"craap"}, Structure: raw, RenderCache: rc, StepCount: 4,
	}); err != nil {
		t.Fatalf("seed a-mid: %v", err)
	}
}

// TestCourseV2EndToEnd walks the full student path through one course:
// list → payload → set progress → read progress back → answer a quiz item
// (incorrectly — never gates) → the finished-course report.
func TestCourseV2EndToEnd(t *testing.T) {
	pool := newAPITestPool(t)
	seedAMidCourse(t, pool)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	// list
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	var listResp struct {
		Courses []struct {
			Slug      string   `json:"slug"`
			CardIDs   []string `json:"card_ids"`
			StepCount int      `json:"step_count"`
		} `json:"courses"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	found := false
	for _, c := range listResp.Courses {
		if c.Slug == "a-mid" {
			found = true
			if c.StepCount != 4 {
				t.Fatalf("a-mid step_count = %d, want 4", c.StepCount)
			}
			if len(c.CardIDs) != 1 || c.CardIDs[0] != "craap" {
				t.Fatalf("a-mid card_ids = %v, want [craap]", c.CardIDs)
			}
		}
	}
	if !found {
		t.Fatalf("a-mid missing from list: %+v", listResp.Courses)
	}

	// get payload
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/a-mid", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("get course: %d %s", rec.Code, rec.Body)
	}
	var courseResp struct {
		Course struct {
			Slug        string          `json:"slug"`
			Title       string          `json:"title"`
			Branch      string          `json:"branch"`
			CardIDs     []string        `json:"cardIds"`
			Structure   json.RawMessage `json:"structure"`
			RenderCache struct {
				Steps []json.RawMessage `json:"steps"`
			} `json:"renderCache"`
		} `json:"course"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &courseResp); err != nil {
		t.Fatalf("get course decode: %v", err)
	}
	if courseResp.Course.Slug != "a-mid" || courseResp.Course.Title == "" {
		t.Fatalf("course payload missing slug/title: %+v", courseResp.Course)
	}
	if len(courseResp.Course.RenderCache.Steps) == 0 {
		t.Fatalf("course payload renderCache.steps is empty")
	}

	// progress default (no row yet) → current_ordinal 0
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/a-mid/progress", nil), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"current_ordinal":0`)) {
		t.Fatalf("progress default: %d %s", rec.Code, rec.Body)
	}

	// put progress: current_ordinal ONLY — a client-sent completed_ordinals
	// must be ignored (铁律②: the floor is never a client assertion).
	body, _ := json.Marshal(map[string]any{"current_ordinal": 1, "completed_ordinals": []int{0, 1, 2, 3}})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/courses/a-mid/progress", bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"current_ordinal":1`)) {
		t.Fatalf("put progress: %d %s", rec.Code, rec.Body)
	}

	// progress read-back: 1 ∈ completed_ordinals (the union, not the client's
	// asserted [0,1,2,3]), started_at non-null.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/a-mid/progress", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("progress read-back: %d %s", rec.Code, rec.Body)
	}
	var progResp struct {
		Progress struct {
			CourseSlug        string  `json:"course_slug"`
			CurrentOrdinal    int     `json:"current_ordinal"`
			CompletedOrdinals []int   `json:"completed_ordinals"`
			StartedAt         *string `json:"started_at"`
		} `json:"progress"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &progResp); err != nil {
		t.Fatalf("progress read-back decode: %v", err)
	}
	if progResp.Progress.CourseSlug != "a-mid" {
		t.Fatalf("progress course_slug = %q, want a-mid", progResp.Progress.CourseSlug)
	}
	if progResp.Progress.StartedAt == nil {
		t.Fatalf("progress started_at is null, want set")
	}
	oneSeen := false
	for _, o := range progResp.Progress.CompletedOrdinals {
		if o == 1 {
			oneSeen = true
		}
	}
	if !oneSeen {
		t.Fatalf("completed_ordinals = %v, want to contain 1", progResp.Progress.CompletedOrdinals)
	}
	if len(progResp.Progress.CompletedOrdinals) != 1 {
		t.Fatalf("completed_ordinals = %v, want exactly [1] — client-sent [0,1,2,3] must be ignored", progResp.Progress.CompletedOrdinals)
	}

	// quiz answer — WRONG, and it still returns 200: quiz answers never gate.
	quizBody, _ := json.Marshal(map[string]any{
		"stepId": "step_01", "interactionId": "intro_q1", "selected": []string{"A"}, "correct": false,
	})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/a-mid/quiz-answer", bytes.NewReader(quizBody)), cookie))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"ok":true`)) {
		t.Fatalf("quiz answer (incorrect): %d %s", rec.Code, rec.Body)
	}

	// report
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/a-mid/report", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("report: %d %s", rec.Code, rec.Body)
	}
	var reportResp struct {
		Report struct {
			Title               string   `json:"title"`
			Goal                string   `json:"goal"`
			TeachingThread      string   `json:"teaching_thread"`
			CompletedStepTitles []string `json:"completedStepTitles"`
			CardIDs             []string `json:"cardIds"`
			Quiz                struct {
				Total   int `json:"total"`
				Correct int `json:"correct"`
			} `json:"quiz"`
		} `json:"report"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &reportResp); err != nil {
		t.Fatalf("report decode: %v", err)
	}
	if reportResp.Report.Title == "" || reportResp.Report.Goal == "" || reportResp.Report.TeachingThread == "" {
		t.Fatalf("report header fields empty: %+v", reportResp.Report)
	}
	if reportResp.Report.Quiz.Total <= 0 {
		t.Fatalf("report quiz.total = %d, want > 0", reportResp.Report.Quiz.Total)
	}
	if len(reportResp.Report.CardIDs) != 1 || reportResp.Report.CardIDs[0] != "craap" {
		t.Fatalf("report cardIds = %v, want [craap]", reportResp.Report.CardIDs)
	}
}

// TestCourseV2UnknownSlug404 asserts every slug-keyed course read 404s on a
// slug that was never published — the pre-v2 suite's own guarantee, ported.
func TestCourseV2UnknownSlug404(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/does-not-exist", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown slug get: want 404 got %d %s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/does-not-exist/report", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown slug report: want 404 got %d %s", rec.Code, rec.Body)
	}
}

// TestCourseAskMissingInputBadRequest asserts postCourseAsk 400s on an empty
// input BEFORE committing to the SSE stream — the same JSON-error-before-
// stream-commit rule postChatTurn follows (chat.go's own doc comment).
func TestCourseAskMissingInputBadRequest(t *testing.T) {
	pool := newAPITestPool(t)
	seedAMidCourse(t, pool)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)

	body, _ := json.Marshal(map[string]any{"input": "", "ordinal": 0})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/a-mid/ask", bytes.NewReader(body)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty input: want 400 got %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Content-Type") == "text/event-stream" {
		t.Fatalf("empty input must not commit to the SSE stream")
	}
}

// TestCourseAskStreamsReplyAndMeters — a free-Q&A ask turn streams the coach's
// reply as an SSE text frame followed by done, records exactly one llm_call
// row (surface=course, purpose=coach, project_id NULL — a course ask is never
// project-scoped), and logs one course_asked event carrying the student's
// question. Mirrors chat_test.go's TestChatTurn_TextOnly assertion shape.
func TestCourseAskStreamsReplyAndMeters(t *testing.T) {
	pool := newAPITestPool(t)
	seedAMidCourse(t, pool)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: fakeProvider(), ChatResolver: fakeResolver(),
	}).Handler()
	cookie := signInSeed(t, pool)

	// 401 unauth.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/courses/a-mid/ask",
		strings.NewReader(`{"input":"这一步在说什么？","ordinal":0}`)))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: want 401, got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/courses/a-mid/ask",
		strings.NewReader(`{"input":"这一步在说什么？","ordinal":0}`))
	h.ServeHTTP(rr, withCookie(req, cookie))
	if rr.Code != http.StatusOK {
		t.Fatalf("ask: %d — %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: text") {
		t.Fatalf("expected a text frame:\n%s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("expected a done frame:\n%s", body)
	}
	if strings.Contains(body, "event: error") {
		t.Fatalf("did not expect an error frame:\n%s", body)
	}

	// One llm_call row recorded, surface=course, purpose=coach, project_id NULL.
	var surface, purpose string
	var projectIDNull bool
	if err := pool.QueryRow(context.Background(),
		`SELECT surface, purpose, project_id IS NULL FROM llm_call WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, SeedUserID,
	).Scan(&surface, &purpose, &projectIDNull); err != nil {
		t.Fatalf("query llm_call: %v", err)
	}
	if surface != "course" || purpose != "coach" || !projectIDNull {
		t.Fatalf("llm_call surface=%q purpose=%q projectIDNull=%v, want course/coach/true", surface, purpose, projectIDNull)
	}

	// One course_asked event logged, carrying the student's question.
	var payload []byte
	if err := pool.QueryRow(context.Background(),
		`SELECT payload FROM event WHERE user_id = $1 AND type = 'course_asked' ORDER BY created_at DESC LIMIT 1`, SeedUserID,
	).Scan(&payload); err != nil {
		t.Fatalf("query course_asked event: %v", err)
	}
	if !bytes.Contains(payload, []byte("这一步在说什么？")) {
		t.Fatalf("course_asked payload = %s, want to contain the student's question", payload)
	}
}
