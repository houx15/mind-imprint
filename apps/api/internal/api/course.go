package api

import (
	"net/http"

	"github.com/google/uuid"

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
		// no row yet → default zero progress
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": map[string]any{
			"course_id": c.ID.String(), "current_ordinal": 0, "completed_ordinals": []int32{}, "updated_at": "",
		}})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": toCourseProgressDTO(p)})
}

func (a *API) putCourseProgress(w http.ResponseWriter, r *http.Request) {
	c, ok := a.loadCourse(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	var body struct {
		CurrentOrdinal    int32   `json:"current_ordinal"`
		CompletedOrdinals []int32 `json:"completed_ordinals"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.CompletedOrdinals == nil {
		body.CompletedOrdinals = []int32{}
	}
	p, err := a.d.Queries.UpsertCourseProgress(r.Context(), sqlc.UpsertCourseProgressParams{
		UserID: u.ID, CourseID: c.ID, CurrentOrdinal: body.CurrentOrdinal, CompletedOrdinals: body.CompletedOrdinals,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"progress": toCourseProgressDTO(p)})
}
