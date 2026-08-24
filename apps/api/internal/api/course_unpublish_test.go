package api_test

// course_unpublish_test.go — 下线: POST /api/v1/admin/courses/{slug}/unpublish,
// the inverse of the ship transition. Mirrors course_ship_test.go's admin-key +
// DB harness (no Voice/OSS configured — unpublish touches neither).

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

func postUnpublish(h http.Handler, slug, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/v1/admin/courses/"+slug+"/unpublish", nil)
	if key != "" {
		bearer(req, key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// A published course goes back to 'preview' and disappears from the student
// catalog — the whole point of 下线 — while its row (and everything that
// references it) survives.
func TestCourseUnpublishHidesItFromStudentsWithoutDeletingIt(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	slug := "unpublish-course"
	if rec := putDefinition(h, slug, putCourseDefinitionBody(shipCourseDefinitionDoc(slug), []string{"craap"}, "a retirable course")); rec.Code != http.StatusOK {
		t.Fatalf("precondition put: want 200 got %d %s", rec.Code, rec.Body)
	}
	if rec := postShip(h, slug, `{"cover":"img:3"}`, testAdminKey); rec.Code != http.StatusOK {
		t.Fatalf("precondition ship: want 200 got %d %s", rec.Code, rec.Body)
	}

	studentCookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses", nil), studentCookie))
	if !containsCourseSlug(rec.Body.String(), slug) {
		t.Fatalf("precondition: published course missing from the student catalog: %s", rec.Body)
	}

	rec = postUnpublish(h, slug, testAdminKey)
	if rec.Code != http.StatusOK {
		t.Fatalf("unpublish: want 200 got %d %s", rec.Code, rec.Body)
	}
	var resp struct {
		Status  string `json:"status"`
		Changed bool   `json:"changed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "preview" || !resp.Changed {
		t.Fatalf("response = %+v, want preview/changed", resp)
	}

	// Verified by a fresh DB read, not the response echo.
	store := agent.NewSqlcAgentStore(q, pool)
	if status, err := store.CourseStatus(context.Background(), slug); err != nil || status != "preview" {
		t.Fatalf("stored status = %q, err %v, want preview", status, err)
	}
	// The course row itself is still there — 下线 is not a delete.
	if _, _, err := store.GetCourseDefinition(context.Background(), slug); err != nil {
		t.Fatalf("course row gone after unpublish: %v", err)
	}
	// And the student can no longer see it.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses", nil), studentCookie))
	if containsCourseSlug(rec.Body.String(), slug) {
		t.Fatalf("student catalog still lists the unpublished course: %s", rec.Body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/"+slug+"/definition", nil), studentCookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("student on unpublished course: want 404, got %d %s", rec.Code, rec.Body)
	}
}

// Re-running 下线 on an already-preview course is a success, not an error —
// an operator retrying a batch must never have to distinguish the two.
func TestCourseUnpublishIsIdempotent(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	slug := "unpublish-idempotent"
	if rec := putDefinition(h, slug, putCourseDefinitionBody(shipCourseDefinitionDoc(slug), nil, "still a draft")); rec.Code != http.StatusOK {
		t.Fatalf("precondition put: want 200 got %d %s", rec.Code, rec.Body)
	}

	rec := postUnpublish(h, slug, testAdminKey) // never published in the first place
	if rec.Code != http.StatusOK {
		t.Fatalf("unpublish a draft: want 200 got %d %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"changed":false`) {
		t.Fatalf("want changed:false for an already-preview course, got %s", rec.Body)
	}
}

func TestCourseUnpublishRequiresTheAdminKeyAndAKnownSlug(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	h := New(Deps{Queries: q, Pool: pool, OSSAdminKey: testAdminKey}).Handler()

	if rec := postUnpublish(h, "anything", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no key: want 401 got %d %s", rec.Code, rec.Body)
	}
	if rec := postUnpublish(h, "anything", "wrong-key"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key: want 401 got %d %s", rec.Code, rec.Body)
	}
	if rec := postUnpublish(h, "no-such-course", testAdminKey); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown slug: want 404 got %d %s", rec.Code, rec.Body)
	}
}
