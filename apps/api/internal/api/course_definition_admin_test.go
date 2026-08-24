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
	"strconv"
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

// putCourseDefinitionBodyWithCategory extends putCourseDefinitionBody with
// the optional category + introduction fields (Task 5).
func putCourseDefinitionBodyWithCategory(definition string, cardIDs []string, blurb, category, introduction string) string {
	cardIDsJSON, _ := json.Marshal(cardIDs)
	blurbJSON, _ := json.Marshal(blurb)
	categoryJSON, _ := json.Marshal(category)
	body := `{"definition":` + definition + `,"blurb":` + string(blurbJSON) + `,"cardIds":` + string(cardIDsJSON) + `,"category":` + string(categoryJSON)
	if introduction != "" {
		body += `,"introduction":` + introduction
	}
	body += `}`
	return body
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

// TestCourseDefinitionAdminRejectsUnknownCategory asserts an out-of-vocab
// category (not one of the 7 controlled slugs) 4xxs before any write.
func TestCourseDefinitionAdminRejectsUnknownCategory(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	body := putCourseDefinitionBodyWithCategory(courseDefinitionDoc("cat-x"), []string{"craap"}, "b", "totally-made-up", "")
	rec := putDefinition(h, "cat-x", body)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 4xx for bad category, body %s", rec.Code, rec.Body)
	}
}

// TestCourseDefinitionAdminAcceptsCategoryAndIntroduction asserts a valid
// category (one of the 7 slugs) + a JSON-object introduction are accepted
// AND actually threaded through to storage — not just a 200 that would pass
// even if the handler silently dropped both fields. Round-trips through
// agent.NewSqlcAgentStore.ListCourses (includePreview=true — a freshly
// upserted course is 'preview'), mirroring
// TestCourseDefinitionAdminRePutPreservesPublishedStatus's store-access
// pattern.
func TestCourseDefinitionAdminAcceptsCategoryAndIntroduction(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	body := putCourseDefinitionBodyWithCategory(courseDefinitionDoc("cat-ok"), []string{"craap"}, "b", "source-check", `{"hook":"h","takeaways":["a"]}`)
	rec := putDefinition(h, "cat-ok", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body %s", rec.Code, rec.Body)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	rows, err := store.ListCourses(context.Background(), true)
	if err != nil {
		t.Fatalf("ListCourses: %v", err)
	}
	var found *agent.CourseSummaryRow
	for i := range rows {
		if rows[i].Slug == "cat-ok" {
			found = &rows[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("course cat-ok not found in ListCourses(includePreview=true)")
	}
	if found.Category == nil || *found.Category != "source-check" {
		t.Fatalf("stored Category = %v, want source-check", found.Category)
	}
	if len(found.Introduction) == 0 {
		t.Fatalf("stored Introduction is empty, want the round-tripped JSON object")
	}
}

// TestCourseDefinitionAdminIntroductionNullTreatedAsUnset asserts a literal
// JSON `null` for introduction is treated the same as absent — leaving it
// unset (nil) — rather than being stored as the literal null value.
func TestCourseDefinitionAdminIntroductionNullTreatedAsUnset(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	body := putCourseDefinitionBodyWithCategory(courseDefinitionDoc("cat-null-intro"), []string{"craap"}, "b", "source-check", `null`)
	rec := putDefinition(h, "cat-null-intro", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body %s", rec.Code, rec.Body)
	}

	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)
	rows, err := store.ListCourses(context.Background(), true)
	if err != nil {
		t.Fatalf("ListCourses: %v", err)
	}
	var found *agent.CourseSummaryRow
	for i := range rows {
		if rows[i].Slug == "cat-null-intro" {
			found = &rows[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("course cat-null-intro not found in ListCourses(includePreview=true)")
	}
	if len(found.Introduction) != 0 {
		t.Fatalf("stored Introduction = %s, want unset (nil) for a JSON null input", found.Introduction)
	}
}

// courseDefinitionDocWithSlices builds a border-valid 2.0 doc whose parts carry
// the given per-part slice counts (each slice a minimal {id} stub). Used to
// prove the catalog step_count = total slices across parts.
func courseDefinitionDocWithSlices(id string, perPart ...int) string {
	parts := make([]string, 0, len(perPart))
	sliceN := 0
	for _, count := range perPart {
		slices := make([]string, 0, count)
		for i := 0; i < count; i++ {
			sliceN++
			slices = append(slices, `{"id":"s`+strconv.Itoa(sliceN)+`","title":"S","blocks":[],"layout":{"preset":"full","slots":[{"id":"main","blockIds":[]}]},"workflow":{"steps":[]},"navigation":{}}`)
		}
		parts = append(parts, `{"id":"p`+strconv.Itoa(len(parts)+1)+`","title":"P","slices":[`+strings.Join(slices, ",")+`]}`)
	}
	return `{"schemaVersion":"2.0","course":{"id":"` + id + `","title":"Generated Course","language":"en","estimatedMinutes":7,"objectives":[],"parts":[` + strings.Join(parts, ",") + `]}}`
}

// TestCourseDefinitionAdminStepCountFromSlices proves the fix: publishing a 2.0
// definition sets step_count = total slices across parts (not the old hardcoded
// 0), the PUT response echoes it, the catalog row carries it, and a re-PUT with
// a different slice count UPDATES the stored value (ON CONFLICT ... step_count).
func TestCourseDefinitionAdminStepCountFromSlices(t *testing.T) {
	pool := newAPITestPool(t)
	h := New(Deps{Queries: sqlc.New(pool), Pool: pool, OSSAdminKey: testAdminKey}).Handler()
	store := agent.NewSqlcAgentStore(sqlc.New(pool), pool)

	// 2 parts, 2 + 1 slices → 3 steps.
	body := putCourseDefinitionBody(courseDefinitionDocWithSlices("stepc", 2, 1), []string{}, "steps")
	rec := putDefinition(h, "stepc", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("put: want 200 got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		StepCount int `json:"step_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.StepCount != 3 {
		t.Fatalf("PUT response step_count = %d, want 3", resp.StepCount)
	}

	rows, err := store.ListCourses(context.Background(), true)
	if err != nil {
		t.Fatalf("ListCourses: %v", err)
	}
	if sc := stepCountOf(rows, "stepc"); sc != 3 {
		t.Fatalf("catalog step_count = %d, want 3", sc)
	}

	// Re-PUT with 5 slices (3 + 2) → the stored count must UPDATE, not stick at 3.
	body2 := putCourseDefinitionBody(courseDefinitionDocWithSlices("stepc", 3, 2), []string{}, "steps")
	if rec2 := putDefinition(h, "stepc", body2); rec2.Code != http.StatusOK {
		t.Fatalf("re-put: want 200 got %d %s", rec2.Code, rec2.Body)
	}
	rows2, err := store.ListCourses(context.Background(), true)
	if err != nil {
		t.Fatalf("ListCourses 2: %v", err)
	}
	if sc := stepCountOf(rows2, "stepc"); sc != 5 {
		t.Fatalf("catalog step_count after re-put = %d, want 5 (ON CONFLICT must update)", sc)
	}
}

func stepCountOf(rows []agent.CourseSummaryRow, slug string) int {
	for _, r := range rows {
		if r.Slug == slug {
			return r.StepCount
		}
	}
	return -1
}
