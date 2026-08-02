package api

// course.go — Task 5's course v2 HTTP surface: slug-keyed handlers over the
// Task 4 content/progress/event store (agent.NewSqlcAgentStore). Replaces the
// retired course-id + session/step-render surface (course_session.go,
// course_assessment*.go, course_render.go — deleted alongside this file, per
// migration 0050's course_session/course_step/course_step_render drop). Two
// routes in api.go's block — postCourseAsk (Task 6) and postAdminUploadCourse
// (Task 7) — are temporary 501 stubs here, kept only so the tree builds
// between tasks; their real implementations are NOT this task's concern.

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

func (a *API) listCourses(w http.ResponseWriter, r *http.Request) {
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	rows, err := store.ListCourses(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]courseSummaryDTO, 0, len(rows))
	for _, c := range rows {
		out = append(out, toCourseSummaryDTO(c))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"courses": out})
}

// getCourse loads one course's full player payload by its external slug.
// httpx.WriteError already maps pgx.ErrNoRows (an unknown slug) to 404.
func (a *API) getCourse(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	payload, _, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"course": toCoursePayloadDTO(payload)})
}

func (a *API) getCourseProgress(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	row, err := store.GetProgress(r.Context(), user.ID, slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": toCourseProgressDTO(slug, row)})
}

// putCourseProgress writes ONLY the resume position (current_ordinal) — a UX
// convenience, not a floor input. Same rule the pre-v2 handler enforced: even
// if a request body carried completed_ordinals, decodeJSON has no
// DisallowUnknownFields, so it is silently dropped — never honored. Whether
// this call finishes the course (markCompletedAt) is computed here, from the
// course's own authored step count (parsed from structure.steps, the same
// narrow field the render_cache/structure header always exposes), never from
// anything the client asserts.
func (a *API) putCourseProgress(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	var body struct {
		CurrentOrdinal int `json:"current_ordinal"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	payload, courseID, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var structure struct {
		Steps []json.RawMessage `json:"steps"`
	}
	_ = json.Unmarshal(payload.Structure, &structure)
	stepCount := len(structure.Steps)
	markCompletedAt := stepCount > 0 && body.CurrentOrdinal == stepCount-1

	row, err := store.SaveProgress(r.Context(), user.ID, courseID, body.CurrentOrdinal, markCompletedAt)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 铁律④: the step turn itself is evidence, logged even though paging never
	// gates. A log failure must not fail the request — the progress write
	// already succeeded.
	eventPayload, _ := json.Marshal(map[string]any{"ordinal": body.CurrentOrdinal})
	if err := store.LogCourseEvent(r.Context(), user.ID, courseID, "course_step_viewed", eventPayload); err != nil {
		slog.Warn("course progress: append course_step_viewed event failed", "err", err, "slug", slug)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": toCourseProgressDTO(slug, row)})
}

// postCourseQuizAnswer logs one quiz attempt. 铁律②: this NEVER gates —
// selected/correct are recorded as evidence regardless of correctness; the
// 200 response is unconditional.
func (a *API) postCourseQuizAnswer(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	var body struct {
		StepID        string   `json:"stepId"`
		InteractionID string   `json:"interactionId"`
		Selected      []string `json:"selected"`
		Correct       bool     `json:"correct"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.StepID == "" || body.InteractionID == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "stepId / interactionId 不能为空", nil))
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	_, courseID, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	eventPayload, err := json.Marshal(body)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := store.LogCourseEvent(r.Context(), user.ID, courseID, "course_quiz_answered", eventPayload); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// getCourseReport assembles CourseReportData (the store's fresh-computed
// completedStepTitles/cardIds/secondsSpent/quiz tally) with the three header
// fields the store doesn't carry (title/goal/teaching_thread), read from the
// same structure JSON GetCoursePayload already returns.
func (a *API) getCourseReport(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	rep, err := store.CourseReport(r.Context(), user.ID, slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	payload, _, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var header courseStructureHeader
	_ = json.Unmarshal(payload.Structure, &header)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"report": toCourseReportDTO(header, rep)})
}

// postCourseAsk drives one course-coach turn against the course v2 model.
//
// TODO(Task 6): implement using agent.ProposeCourseReply/BuildCourseContext
// (internal/agent/course_coach.go), scoped by (user, course slug) rather than
// the retired course_session row. This is a temporary stub ONLY so the tree
// builds between Task 5 and Task 6 — do not extend it here.
func (a *API) postCourseAsk(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusNotImplemented, map[string]any{
		"error": map[string]string{"code": "not_implemented", "message": "course ask lands in Task 6"},
	})
}

// postAdminUploadCourse publishes one course's authored content.
//
// TODO(Task 7): implement — parse+validate the authored structure/
// render_cache bundle behind the OSS-admin-key bearer gate, then
// agent.UpsertCourse. This is a temporary stub ONLY so the tree builds
// between Task 5 and Task 7 — do not extend it here.
func (a *API) postAdminUploadCourse(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusNotImplemented, map[string]any{
		"error": map[string]string{"code": "not_implemented", "message": "admin course upload lands in Task 7"},
	})
}
