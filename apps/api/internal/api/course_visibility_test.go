package api_test

// course_visibility_test.go — Task 2 of the course authoring & publish
// lifecycle: the preview-visibility gate. A 'preview' course must be
// invisible to students (404, indistinguishable from an unknown slug) and
// fully visible to admins. isAdmin is pure (table test); requireVisibleCourse
// is exercised end-to-end through getCourseDefinition, mirroring
// course_definition_test.go's seed/request pattern.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestIsAdmin(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{"admin user in ctx", WithUser(ctx, User{ID: uuid.New(), Role: "admin"}), true},
		{"student user in ctx", WithUser(ctx, User{ID: uuid.New(), Role: "student"}), false},
		{"no user in ctx", ctx, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsAdminForTest(tc.ctx); got != tc.want {
				t.Fatalf("isAdmin() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPreviewCourseHiddenFromStudents404sVisibleToAdmin — a course flipped to
// 'preview' status 404s getCourseDefinition for a student (same response as
// an unknown slug — never leaks that a draft exists) but still 200s for an
// admin.
func TestPreviewCourseHiddenFromStudents404sVisibleToAdmin(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "preview-course", testCourseDefinitionJSON)

	q := sqlc.New(pool)
	if err := q.SetCourseStatusAndCover(context.Background(), sqlc.SetCourseStatusAndCoverParams{
		Slug: "preview-course", Status: "preview", Cover: "",
	}); err != nil {
		t.Fatalf("flip to preview: %v", err)
	}

	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	studentCookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/preview-course/definition", nil), studentCookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("student on preview course: want 404, got %d %s", rec.Code, rec.Body)
	}

	adminCookie := signInAdmin(t, pool)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses/preview-course/definition", nil), adminCookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("admin on preview course: want 200, got %d %s", rec.Code, rec.Body)
	}
}

// TestPreviewCourseAbsentFromStudentCatalogListedForAdmin — listCourses omits
// a 'preview' course for a student (isAdmin(ctx)==false → includePreview
// false) but includes it for an admin.
func TestPreviewCourseAbsentFromStudentCatalogListedForAdmin(t *testing.T) {
	pool := newAPITestPool(t)
	seedCourseWithDefinition(t, pool, "preview-catalog-course", testCourseDefinitionJSON)

	q := sqlc.New(pool)
	if err := q.SetCourseStatusAndCover(context.Background(), sqlc.SetCourseStatusAndCoverParams{
		Slug: "preview-catalog-course", Status: "preview", Cover: "",
	}); err != nil {
		t.Fatalf("flip to preview: %v", err)
	}

	h := New(Deps{Queries: sqlc.New(pool), Pool: pool}).Handler()

	studentCookie := signInSeed(t, pool)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses", nil), studentCookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("student list courses: want 200, got %d %s", rec.Code, rec.Body)
	}
	if containsCourseSlug(rec.Body.String(), "preview-catalog-course") {
		t.Fatalf("student catalog leaked preview course: %s", rec.Body)
	}

	adminCookie := signInAdmin(t, pool)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/courses", nil), adminCookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("admin list courses: want 200, got %d %s", rec.Code, rec.Body)
	}
	if !containsCourseSlug(rec.Body.String(), "preview-catalog-course") {
		t.Fatalf("admin catalog missing preview course: %s", rec.Body)
	}
}

func containsCourseSlug(body, slug string) bool {
	return strings.Contains(body, `"slug":"`+slug+`"`)
}
