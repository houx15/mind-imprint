package api

// course_visibility.go — the preview-visibility gate: a 'preview' course must
// be invisible to students (absent from the catalog, 404 on every by-slug
// read) while remaining fully visible/playable to admins. Task 2 of the
// course authoring & publish lifecycle (Task 1's store.CourseStatus /
// store.ListCourses(ctx, includePreview) already merged on this branch).

import (
	"context"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// isAdmin reports whether the session user in ctx is an admin. Preview courses
// are visible/playable only to admins; students see published only.
func isAdmin(ctx context.Context) bool {
	u, ok := UserFromContext(ctx)
	return ok && u.Role == "admin"
}

// requireVisibleCourse enforces the preview gate for a by-slug course read: if
// the course is 'preview' and the caller is not an admin it writes a 404 (a
// draft is indistinguishable from a nonexistent course, so it never leaks) and
// returns false. A published course, an admin caller, or an unknown slug (left
// to the downstream handler's own 404) returns true.
func (a *API) requireVisibleCourse(w http.ResponseWriter, r *http.Request, slug string) bool {
	if isAdmin(r.Context()) {
		return true
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	status, err := store.CourseStatus(r.Context(), slug)
	if err != nil {
		return true // unknown slug / db error → let the downstream handler map it (404 etc.)
	}
	if status == "preview" {
		httpx.WriteError(w, r, httpx.ErrNotFound("课程不存在"))
		return false
	}
	return true
}
