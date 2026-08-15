package api_test

// course_definition_test.go — Course Runtime Slice 8: GET /api/v1/courses/{slug}
// /definition. A course WITH a stored 2.0 definition returns it (border-checked
// schemaVersion=="2.0"); a legacy course with none is 404; unauth is 401.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// A minimal, border-valid CourseDefinition 2.0 document. The endpoint only
// checks schemaVersion; full structural validation is the frontend's job.
const testCourseDefinitionJSON = `{"schemaVersion":"2.0","course":{"id":"test-runtime-course","title":"Test Course","language":"en","estimatedMinutes":5,"objectives":[],"parts":[]}}`

// seedCourseWithDefinition upserts a bare course row under slug and attaches a
// CourseDefinition 2.0 document to it. Structure/render_cache are empty ({}) —
// a 2.0 course plays through the runtime, not the pre-rendered player.
func seedCourseWithDefinition(t *testing.T, pool *pgxpool.Pool, slug, definitionJSON string) {
	t.Helper()
	st := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	if err := st.UpsertCourse(context.Background(), agent.UpsertCourseInput{
		Slug: slug, Branch: "Runtime", Title: "Test Course", Blurb: "b", TimeLabel: "约 5 分钟",
		CardIDs: []string{}, Structure: []byte("{}"), RenderCache: []byte("{}"), StepCount: 0,
	}); err != nil {
		t.Fatalf("upsert course %s: %v", slug, err)
	}
	if err := st.SetCourseDefinition(context.Background(), slug, []byte(definitionJSON)); err != nil {
		t.Fatalf("set definition %s: %v", slug, err)
	}
}

func TestCourseDefinitionServed(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "def-course", testCourseDefinitionJSON)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/def-course/definition", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("definition: want 200, got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Definition struct {
			SchemaVersion string `json:"schemaVersion"`
			Course        struct {
				ID string `json:"id"`
			} `json:"course"`
		} `json:"definition"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v — body %s", err, rec.Body)
	}
	if resp.Definition.SchemaVersion != "2.0" {
		t.Fatalf("schemaVersion = %q, want 2.0", resp.Definition.SchemaVersion)
	}
	if resp.Definition.Course.ID != "test-runtime-course" {
		t.Fatalf("course.id = %q, want test-runtime-course", resp.Definition.Course.ID)
	}
}

// TestCourseDefinitionMissing404 — a legacy course (seeded with no 2.0
// definition) returns 404 from the definition endpoint, so the frontend routes
// it to the legacy player.
func TestCourseDefinitionMissing404(t *testing.T) {
	pool := newAPITestPool(t)
	seedAMidCourse(t, pool) // legacy course, no course_definition
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()
	cookie := signInSeed(t, pool)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/a-mid/definition", nil), cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("legacy course definition: want 404, got %d %s", rec.Code, rec.Body)
	}
}

func TestCourseDefinitionRequiresAuth(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "def-course", testCourseDefinitionJSON)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/courses/def-course/definition", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: want 401, got %d", rec.Code)
	}
}
