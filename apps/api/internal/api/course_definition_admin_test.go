package api_test

// course_definition_admin_test.go — Task 3 of the course authoring & publish
// lifecycle: PUT /api/v1/admin/courses/{slug}/definition, the course
// generator's create/modify endpoint. Mirrors course_admin_test.go's
// admin-key bearer pattern and course_definition_test.go's definition-shaped
// fixtures.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

// putCourseDefinitionBody builds the PUT envelope: definition/blurb/cardIds.
func putCourseDefinitionBody(definition string, cardIDs []string, blurb string) string {
	cardIDsJSON, _ := json.Marshal(cardIDs)
	blurbJSON, _ := json.Marshal(blurb)
	return `{"definition":` + definition + `,"blurb":` + string(blurbJSON) + `,"cardIds":` + string(cardIDsJSON) + `}`
}

// courseDefinitionDoc returns a border-valid CourseDefinition 2.0 document
// for the given course id, mirroring testCourseDefinitionJSON's shape.
func courseDefinitionDoc(id string) string {
	return `{"schemaVersion":"2.0","course":{"id":"` + id + `","title":"Generated Course","language":"en","estimatedMinutes":7,"objectives":[],"parts":[]}}`
}

func putDefinition(h http.Handler, slug, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("PUT", "/api/v1/admin/courses/"+slug+"/definition", strings.NewReader(body))
	bearer(req, testAdminKey)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestPutCourseDefinitionCreatesPreview asserts a valid PUT on a brand-new
// slug returns 200 {slug, status:"preview"} and that the definition is then
// readable through GET /courses/{slug}/definition (admin-visible even in
// preview, per requireVisibleCourse).
func TestCourseDefinitionAdminCreatesPreview(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	body := putCourseDefinitionBody(courseDefinitionDoc("gen-course"), []string{"craap"}, "a generated course")
	rec := putDefinition(h, "gen-course", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid put: want 200 got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Slug   string `json:"slug"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Slug != "gen-course" || resp.Status != "preview" {
		t.Fatalf("resp = %+v, want slug=gen-course status=preview", resp)
	}

	// Readable back via the admin-visible GET (a fresh course is 'preview',
	// invisible to students but visible to admins per requireVisibleCourse).
	adminCookie := signInAdmin(t, pool)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/gen-course/definition", nil), adminCookie))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get definition: want 200 got %d %s", getRec.Code, getRec.Body)
	}
	var getResp struct {
		Definition struct {
			SchemaVersion string `json:"schemaVersion"`
			Course        struct {
				ID string `json:"id"`
			} `json:"course"`
		} `json:"definition"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("get definition decode: %v", err)
	}
	if getResp.Definition.SchemaVersion != "2.0" || getResp.Definition.Course.ID != "gen-course" {
		t.Fatalf("get definition payload = %+v", getResp.Definition)
	}
}

// TestPutCourseDefinitionUnauthorized asserts a missing or blank admin key
// 401s before any parsing/DB work.
func TestCourseDefinitionAdminUnauthorized(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	body := putCourseDefinitionBody(courseDefinitionDoc("gen-course-unauth"), []string{"craap"}, "b")

	t.Run("missing bearer", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/v1/admin/courses/gen-course-unauth/definition", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("missing bearer: want 401 got %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("blank bearer", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/v1/admin/courses/gen-course-unauth/definition", strings.NewReader(body))
		bearer(req, "")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("blank bearer: want 401 got %d %s", rec.Code, rec.Body)
		}
	})
}

// TestPutCourseDefinitionValidation covers the border-validation rejections:
// wrong schemaVersion (422), course.id != slug (400), unknown cardId (400).
func TestCourseDefinitionAdminValidation(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	t.Run("schemaVersion != 2.0", func(t *testing.T) {
		badDoc := `{"schemaVersion":"1.0","course":{"id":"bad-schema","title":"Bad"}}`
		rec := putDefinition(h, "bad-schema", putCourseDefinitionBody(badDoc, []string{"craap"}, "b"))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("bad schemaVersion: want 422 got %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("course.id != slug", func(t *testing.T) {
		rec := putDefinition(h, "wrong-slug", putCourseDefinitionBody(courseDefinitionDoc("mismatched-id"), []string{"craap"}, "b"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("id/slug mismatch: want 400 got %d %s", rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "validation_failed") {
			t.Fatalf("id/slug mismatch: want validation_failed code, got %s", rec.Body)
		}
	})

	t.Run("unknown cardId", func(t *testing.T) {
		rec := putDefinition(h, "gen-course-badcard", putCourseDefinitionBody(courseDefinitionDoc("gen-course-badcard"), []string{"not-a-card"}, "b"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unknown card id: want 400 got %d %s", rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "validation_failed") {
			t.Fatalf("unknown card id: want validation_failed code, got %s", rec.Body)
		}
	})

	// None of the rejected puts should have written a course row.
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM course WHERE slug IN ('bad-schema','wrong-slug','mismatched-id','gen-course-badcard')`).Scan(&count); err != nil {
		t.Fatalf("count courses: %v", err)
	}
	if count != 0 {
		t.Fatalf("rejected puts wrote %d course rows, want 0", count)
	}
}

// TestPutCourseDefinitionRePutPreservesPublishedStatus asserts the
// UpsertCourseDefinition query's ON CONFLICT contract end-to-end through the
// handler: a course flipped to 'published' (via SetCourseStatusAndCover, the
// only way status changes today) stays 'published' after a re-PUT of its
// definition — the handler never touches status itself.
func TestCourseDefinitionAdminRePutPreservesPublishedStatus(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	slug := "gen-course-republish"
	firstBody := putCourseDefinitionBody(courseDefinitionDoc(slug), []string{"craap"}, "v1 blurb")
	rec := putDefinition(h, slug, firstBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("initial put: want 200 got %d %s", rec.Code, rec.Body)
	}

	q := sqlc.New(pool)
	if err := q.SetCourseStatusAndCover(context.Background(), sqlc.SetCourseStatusAndCoverParams{
		Slug: slug, Status: "published", Cover: "",
	}); err != nil {
		t.Fatalf("flip to published: %v", err)
	}
	status, err := agent.NewSqlcAgentStore(q, pool).CourseStatus(context.Background(), slug)
	if err != nil || status != "published" {
		t.Fatalf("precondition: course status = %q, err %v, want published", status, err)
	}

	secondBody := putCourseDefinitionBody(courseDefinitionDoc(slug), []string{"craap"}, "v2 blurb — content update")
	rec = putDefinition(h, slug, secondBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("re-put: want 200 got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Slug   string `json:"slug"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "published" {
		t.Fatalf("re-put response status = %q, want published (re-PUT must not un-publish)", resp.Status)
	}

	status, err = agent.NewSqlcAgentStore(q, pool).CourseStatus(context.Background(), slug)
	if err != nil {
		t.Fatalf("CourseStatus after re-put: %v", err)
	}
	if status != "published" {
		t.Fatalf("stored status after re-put = %q, want published", status)
	}
}
