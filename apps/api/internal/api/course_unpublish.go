package api

// course_unpublish.go — 下线: the inverse of course_ship.go's publish.
// POST /api/v1/admin/courses/{slug}/unpublish (OSS_ADMIN_KEY bearer) flips
// status published -> preview, which is exactly what makes a course invisible
// to students: requireVisibleCourse (course_visibility.go) 404s a 'preview'
// course for a non-admin session, and ListCourseRows only returns 'published'
// unless the caller is an admin.
//
// Deliberately NOT a delete. Retiring a course must never orphan the sessions,
// progress rows and events students already produced under it (铁律④: the
// record is the product). Unpublishing hides the course from the catalog while
// every row that references it stays intact and readable by the teacher
// surfaces — and re-publishing is one ship call away.
//
// The cover is preserved: SetCourseStatusAndCover treats an empty cover as
// "leave it alone", so a re-ship without a cover argument still finds the art
// that was there before.

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

func (a *API) postCourseUnpublish(w http.ResponseWriter, r *http.Request) {
	if !a.ossAdminAuthed(r) { // constant-time compare (oss.go) — same gate as ship
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥")) // 401, no key echoed
		return
	}
	slug := r.PathValue("slug")
	// Same defensive slug shape check as ship/asset-upload: a traversal-shaped
	// slug has no business reaching a store lookup, even though this handler
	// derives no object keys from it.
	if slug == "" || strings.Contains(slug, "..") || strings.Contains(slug, "/") {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_slug", "无效的课程标识。", nil))
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	status, err := store.CourseStatus(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("课程不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	// Idempotent: unpublishing an already-preview course is a no-op success,
	// so a retried/duplicated 下线 call never turns into an error the operator
	// has to reason about.
	if status == "preview" {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"slug": slug, "status": "preview", "changed": false})
		return
	}
	if err := store.SetCourseStatusAndCover(r.Context(), slug, "preview", ""); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"slug": slug, "status": "preview", "changed": true})
}
