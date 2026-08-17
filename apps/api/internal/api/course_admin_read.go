package api

// course_admin_read.go — the read counterpart of the OSS_ADMIN_KEY-gated
// authoring routes (course_definition_admin.go / course_ship.go). The course
// generator writes with the admin key but, until now, could only read a course
// back through the session-gated /api/v1/courses/... surface, which HIDES
// preview (draft) courses from any non-admin session. These two endpoints let
// the generator do a headless round-trip readback of the drafts it created
// using nothing but the same bearer key it writes with — no admin login/session:
//
//	GET /api/v1/admin/courses                     → every course incl. preview, + status
//	GET /api/v1/admin/courses/{slug}/definition   → { definition, hash, status } for any status
//
// Both bypass the preview-visibility gate on purpose (that gate protects
// students, not the key holder). Neither mutates anything.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// adminCourseSummaryDTO is the student-facing courseSummaryDTO plus the publish
// status, which the student list deliberately omits. Embedding flattens the
// shared fields into the same JSON object.
type adminCourseSummaryDTO struct {
	courseSummaryDTO
	Status string `json:"status"`
}

// listCoursesAdmin returns EVERY course (preview + published) behind the
// OSS_ADMIN_KEY bearer, so the generator can discover/read back the drafts it
// created without an admin session. Mirrors listCourses but forces
// includePreview=true and exposes each course's publish status.
func (a *API) listCoursesAdmin(w http.ResponseWriter, r *http.Request) {
	if !a.ossAdminAuthed(r) { // same constant-time gate as the authoring routes
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥")) // 401, no key echoed
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	rows, err := store.ListCourses(r.Context(), true) // includePreview: the key holder sees drafts
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]adminCourseSummaryDTO, 0, len(rows))
	for _, c := range rows {
		out = append(out, adminCourseSummaryDTO{courseSummaryDTO: a.toCourseSummaryDTO(c), Status: c.Status})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"courses": out})
}

// getCourseDefinitionAdmin is the read counterpart of putCourseDefinition:
// returns a course's stored CourseDefinition 2.0 document regardless of publish
// status (unlike the session-gated getCourseDefinition, which 404s a preview
// course for non-admins). The `hash` is the same sha256-of-stored-bytes
// revision signal getCourseDefinition emits; `status` lets the generator tell a
// draft from a published course. Border-validates schemaVersion=="2.0" — a
// malformed blob is 422, never shipped as if valid.
func (a *API) getCourseDefinitionAdmin(w http.ResponseWriter, r *http.Request) {
	if !a.ossAdminAuthed(r) {
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥"))
		return
	}
	slug := r.PathValue("slug")
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	def, status, err := store.GetCourseDefinition(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("课程不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	if len(def) == 0 { // course exists but has no 2.0 definition (legacy)
		httpx.WriteError(w, r, httpx.ErrNotFound("该课程没有 2.0 定义"))
		return
	}
	var head struct {
		SchemaVersion string `json:"schemaVersion"`
	}
	if json.Unmarshal(def, &head) != nil || head.SchemaVersion != "2.0" {
		httpx.WriteError(w, r, &httpx.APIError{
			Status:  http.StatusUnprocessableEntity,
			Code:    "invalid_course_definition",
			Message: "课程定义格式无效",
		})
		return
	}
	sum := sha256.Sum256(def)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"definition": json.RawMessage(def),
		"hash":       hex.EncodeToString(sum[:]),
		"status":     status,
	})
}
