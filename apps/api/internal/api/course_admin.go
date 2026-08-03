package api

// course_admin.go — Task 7: the developer publish path for course v2 content.
// A course is authored OFFLINE (structure JSON + a rendered content cache +
// the tool-card ids the course summons) and pushed here behind the
// OSS_ADMIN_KEY bearer (the same gate oss.go's admin routes use — see
// ossBearer/a.d.OSSAdminKey). This handler only border-validates the outer
// envelope (ids/titles/steps present, the two payloads agree on courseId,
// every cardId is a real registry entry); it never interprets the deep
// structure/render_cache shape beyond that — the class-agent repo that
// authors these owns the inner shape, and agent.UpsertCourse stores both
// blobs verbatim as []byte (course.go's other handlers already treat
// Structure/RenderCache this way; see coursestore.go's package doc).

import (
	"encoding/json"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/httpx"
)

// postAdminUploadCourseReq is the upload envelope: Course/RenderCache travel
// as json.RawMessage (stored verbatim), CardIDs is checked against the
// registry, Branch/Blurb/TimeLabel are the catalog-card fields UpsertCourse
// needs that neither payload carries.
type postAdminUploadCourseReq struct {
	Course      json.RawMessage `json:"course"`
	RenderCache json.RawMessage `json:"renderCache"`
	CardIDs     []string        `json:"cardIds"`
	Branch      string          `json:"branch"`
	Blurb       string          `json:"blurb"`
	TimeLabel   string          `json:"time_label"`
}

// postAdminUploadCourseCourse is the narrow slice of `course` this handler
// border-validates: id/title must be non-empty and there must be ≥1 step.
// The step's own shape is not this handler's concern beyond counting it.
type postAdminUploadCourseCourse struct {
	ID    string            `json:"id"`
	Title string            `json:"title"`
	Steps []json.RawMessage `json:"steps"`
}

// postAdminUploadCourseRenderCache is the narrow slice of `renderCache` this
// handler border-validates: courseId must match course.id, and there must be
// ≥1 rendered step.
type postAdminUploadCourseRenderCache struct {
	CourseID string            `json:"courseId"`
	Steps    []json.RawMessage `json:"steps"`
}

// postAdminUploadCourse lets a developer push a pre-built course (structure
// JSON + published render cache + attached tool cards) behind the
// OSS_ADMIN_KEY bearer. Border-validates the envelope, then upserts by slug
// (course.id) — a re-upload of the same id REPLACES the course's content,
// per agent.UpsertCourse's own upsert-by-slug contract.
func (a *API) postAdminUploadCourse(w http.ResponseWriter, r *http.Request) {
	if !a.ossAdminAuthed(r) { // constant-time compare (oss.go) — same gate as the OSS admin routes
		httpx.WriteError(w, r, httpx.ErrUnauthorized("需要有效的管理密钥")) // 401, no key echoed
		return
	}

	var body postAdminUploadCourseReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	var course postAdminUploadCourseCourse
	if err := json.Unmarshal(body.Course, &course); err != nil || course.ID == "" || course.Title == "" || len(course.Steps) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "course 结构缺少 id/title/steps", nil))
		return
	}
	var renderCache postAdminUploadCourseRenderCache
	if err := json.Unmarshal(body.RenderCache, &renderCache); err != nil || len(renderCache.Steps) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "renderCache 缺少 steps", nil))
		return
	}
	if renderCache.CourseID != course.ID {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "renderCache.courseId 与 course.id 不一致", nil))
		return
	}
	// Completion (putCourseProgress) is computed server-side from
	// len(structure.steps), but the player pages by len(renderCache.steps).
	// A mismatch would let the player either dead-end before "完成课程" ever
	// fires, or mark the course finished before the player has shown every
	// step — reject at the border instead of letting the two counts drift.
	if len(course.Steps) != len(renderCache.Steps) {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "course.steps 与 renderCache.steps 数量不一致", nil))
		return
	}
	for _, id := range body.CardIDs {
		if _, ok := cards.ByID(id); !ok {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "未知的工具卡: "+id, nil))
			return
		}
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.UpsertCourse(r.Context(), agent.UpsertCourseInput{
		Slug:        course.ID,
		Branch:      body.Branch,
		Title:       course.Title,
		Blurb:       body.Blurb,
		TimeLabel:   body.TimeLabel,
		CardIDs:     body.CardIDs,
		Structure:   body.Course,
		RenderCache: body.RenderCache,
		StepCount:   len(course.Steps),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"slug": course.ID, "step_count": len(course.Steps)})
}
