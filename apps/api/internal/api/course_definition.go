package api

// course_definition.go — Course Runtime Slice 8: serve the stored
// CourseDefinition 2.0 document. GET /api/v1/courses/{slug}/definition returns
// { definition: <CourseDefinitionDocument> } (the raw stored jsonb, passed
// through untouched) for a course that has a 2.0 definition; 404 for a legacy
// course with none (or an unknown slug) — the frontend routes that course to the
// legacy player. The document is border-validated (schemaVersion == "2.0")
// before serving; a malformed blob is 422, never shipped to the runtime player
// (whose own full structural validation is the deep source of truth).

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

func (a *API) getCourseDefinition(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !a.requireVisibleCourse(w, r, slug) {
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	def, _, err := store.GetCourseDefinition(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("课程不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	// NULL column: the course exists but has no 2.0 definition (legacy course).
	if len(def) == 0 {
		httpx.WriteError(w, r, httpx.ErrNotFound("该课程没有 2.0 定义"))
		return
	}
	// Border validation: the runtime consumes { schemaVersion: "2.0", course }.
	// The full structural/referential/workflow validation is the frontend
	// validateCourseDefinition's job (it renders a diagnostic surface); here we
	// only refuse to ship an obviously wrong-shaped blob to the player.
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"definition": json.RawMessage(def)})
}
