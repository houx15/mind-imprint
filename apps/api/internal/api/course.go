package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

func (a *API) listCourses(w http.ResponseWriter, r *http.Request) {
	rows, err := a.d.Queries.ListCourses(r.Context())
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

// loadCourse parses {id} and loads the course (404 if absent).
func (a *API) loadCourse(w http.ResponseWriter, r *http.Request) (sqlc.Course, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Course{}, false
	}
	c, err := a.d.Queries.GetCourse(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err) // ErrNoRows → 404
		return sqlc.Course{}, false
	}
	return c, true
}

func (a *API) getCourse(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loadCourse(w, r)
	if !ok {
		return
	}
	steps, err := a.d.Queries.ListCourseSteps(r.Context(), c.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	n, err := a.d.Queries.CountCourseSteps(r.Context(), c.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	stepDTOs := make([]courseStepDTO, 0, len(steps))
	for _, s := range steps {
		stepDTOs = append(stepDTOs, toCourseStepDTO(s))
	}
	dto := courseDTO{
		courseSummaryDTO: courseSummaryDTO{
			ID: c.ID.String(), Branch: c.Branch, Title: c.Title, Blurb: c.Blurb,
			TasksCount: c.TasksCount, ToolsCount: c.ToolsCount, TimeLabel: c.TimeLabel,
			StepCount: int(n),
		},
		Steps: stepDTOs,
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"course": dto})
}

func (a *API) getCourseProgress(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loadCourse(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	p, err := a.d.Queries.GetCourseProgress(r.Context(), sqlc.GetCourseProgressParams{UserID: u.ID, CourseID: c.ID})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, err)
			return
		}
		// no row yet → default zero progress for a first-time visitor
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": courseProgressDTO{
			CourseID: c.ID.String(), CurrentOrdinal: 0, CompletedOrdinals: []int32{}, UpdatedAt: "",
		}})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": toCourseProgressDTO(p)})
}

// putCourseProgress writes ONLY the resume position (current_ordinal) — a UX
// convenience, not a floor input. Slice-12 whole-branch C1+C3 fix:
// completed_ordinals (the steps_viewed floor's input) is never accepted from
// the client here; the request body may still carry it (the existing web
// client's saveCourseProgress does, unchanged), but it is silently ignored —
// decodeJSON has no DisallowUnknownFields, so an unread field is simply
// dropped. The only writer of completed_ordinals is the render handler
// (course_render.go), which records a step as viewed exactly when the
// student's browser actually opens it. A floor the client can assert is not
// a floor.
func (a *API) putCourseProgress(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loadCourse(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	var body struct {
		CurrentOrdinal int32 `json:"current_ordinal"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := a.d.Queries.SetCourseCurrentOrdinal(r.Context(), sqlc.SetCourseCurrentOrdinalParams{
		UserID: u.ID, CourseID: c.ID, CurrentOrdinal: body.CurrentOrdinal,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": toCourseProgressDTO(p)})
}
