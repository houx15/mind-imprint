package api_test

// course_assessment_test.go — A1: GET/POST /api/v1/courses/{id}/session/assessment.
// The session-scoped sibling of assessment_test.go. GET never calls a model;
// POST runs the isolated flagship assessor once over the course session's own
// evidence and persists it at session scope.

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

// seededCourseID names the same fixture as course_session_test.go's
// courseSeededCourseID (migration 0012's one seeded course); this file reuses
// that constant and its startSession(t, h, cookie, courseID) helper directly
// rather than redeclaring either under these names.
const seededCourseID = courseSeededCourseID

// countCourseLLMCalls counts course-surface llm_call rows. The existing
// countLLMCalls filters by project_id, which is NULL for every course call.
func countCourseLLMCalls(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM llm_call WHERE surface = 'course' AND purpose = 'assessment'`).Scan(&n); err != nil {
		t.Fatalf("count course llm_call: %v", err)
	}
	return n
}

// TestGetCourseAssessment_EmptyBeforeGenerate — "not yet assessed" is a normal
// state, never a 404: 200 + literal JSON null, and ZERO llm_call rows.
func TestGetCourseAssessment_EmptyBeforeGenerate(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie, seededCourseID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+seededCourseID+"/session/assessment", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if strings.TrimSpace(rec.Body.String()) != "null" {
		t.Fatalf("GET body = %s, want literal null", rec.Body)
	}
	if n := countCourseLLMCalls(t, pool); n != 0 {
		t.Fatalf("llm_call rows after GET = %d, want 0 — the read path never calls a model", n)
	}
}

// TestGenerateCourseAssessment_PersistsAtSessionScope — one flagship call,
// metered surface=course/purpose=assessment, persisted so GET replays it.
func TestGenerateCourseAssessment_PersistsAtSessionScope(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider: assessStubProvider(assessReply), ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie, seededCourseID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+seededCourseID+"/session/assessment", strings.NewReader("")), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST = %d, want 200; body=%s", rec.Code, rec.Body)
	}

	var dto struct {
		Dimensions []struct {
			Code, Name, Level, Evidence string
		} `json:"dimensions"`
		Narrative   string `json:"narrative"`
		GeneratedAt string `json:"generatedAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("decode DTO: %v — body=%s", err, rec.Body)
	}
	if len(dto.Dimensions) != 10 {
		t.Fatalf("dimensions len = %d, want 10 (every rubric dimension, unevidenced ones NA)", len(dto.Dimensions))
	}
	if dto.Narrative == "" || dto.GeneratedAt == "" {
		t.Fatalf("dto = %+v, want narrative + generatedAt", dto)
	}

	var surface, purpose string
	if err := pool.QueryRow(context.Background(),
		`SELECT surface, purpose FROM llm_call ORDER BY created_at DESC LIMIT 1`).Scan(&surface, &purpose); err != nil {
		t.Fatalf("read llm_call: %v", err)
	}
	if surface != "course" || purpose != "assessment" {
		t.Fatalf("llm_call = (%s,%s), want (course,assessment)", surface, purpose)
	}

	// 评估走旗舰模型绝不降级 — the assessor must resolve the flagship tier, never
	// the chaperone tier the coach turn uses. Regression guard for the bug
	// where both assessors called a.d.ChatResolver instead of a.d.EvalResolver.
	var tier string
	if err := pool.QueryRow(context.Background(),
		`SELECT tier FROM llm_call WHERE surface = 'course' AND purpose = 'assessment'
		 ORDER BY created_at DESC LIMIT 1`).Scan(&tier); err != nil {
		t.Fatalf("read llm_call tier: %v", err)
	}
	if tier != "flagship" {
		t.Fatalf("assessment ran on tier %q, want flagship — 评估走旗舰模型绝不降级", tier)
	}

	// Persisted at session scope: GET now replays it with no second model call.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+seededCourseID+"/session/assessment", nil), cookie))
	if strings.TrimSpace(rec2.Body.String()) == "null" {
		t.Fatal("GET after POST returned null — the report was not persisted at session scope")
	}
	if n := countCourseLLMCalls(t, pool); n != 1 {
		t.Fatalf("llm_call rows after POST+GET = %d, want exactly 1", n)
	}
}

// TestGenerateCourseAssessment_RecordsCostOnRejection — a rejected call still
// cost money, so the llm_call row must exist even though nothing is persisted.
//
// The narrative below deliberately contains "你应该这样写" — verified against
// apps/api/internal/agent/enforcement/banned_phrasing.go's "rewritten-sentence-zh"
// rule, which matches that substring regardless of what follows. (The brief's
// original candidate, "作为一个 AI 语言模型，我认为你做得很好。", matches NONE of
// the corpus's regexes and would have made this test pass for the wrong
// reason — the Slice 12 mock-infidelity lesson.)
func TestGenerateCourseAssessment_RecordsCostOnRejection(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{
		Queries: sqlc.New(pool), Pool: pool,
		Provider:     assessStubProvider(`{"dimensions":[],"narrative":"你应该这样写：先摆结论，再给证据。"}`),
		ChatResolver: fakeResolver(), EvalResolver: fakeEvalResolver(), SpecByID: cards.ByID,
	}).Handler()
	cookie := signInSeed(t, pool)
	startSession(t, h, cookie, seededCourseID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("POST", "/api/v1/courses/"+seededCourseID+"/session/assessment", strings.NewReader("")), cookie))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST (banned phrasing) = %d, want 422; body=%s", rec.Code, rec.Body)
	}
	if n := countCourseLLMCalls(t, pool); n != 1 {
		t.Fatalf("llm_call rows after rejection = %d, want 1 — a rejected call still cost money", n)
	}
}

// TestCourseAssessment_NoSession404s — a caller with no session on this course
// gets 404 and leaks nothing (loadOwnedSession resolves by (caller, course_id),
// so another user's session is unreachable by construction).
func TestCourseAssessment_NoSession404s(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)
	// No startSession call.

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+seededCourseID+"/session/assessment", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET without a session = %d, want 404", rec.Code)
	}
}
