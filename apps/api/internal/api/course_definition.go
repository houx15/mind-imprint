package api

// course_definition.go — Course Runtime Slice 8: serve the stored
// CourseDefinition 2.0 document. GET /api/v1/courses/{slug}/definition returns
// { definition: <CourseDefinitionDocument>, hash: <sha256 hex> } (the raw
// stored jsonb, passed through untouched, plus a content hash of those exact
// bytes) for a course that has a 2.0 definition; 404 for a legacy course with
// none (or an unknown slug) — the frontend routes that course to the legacy
// player. The document is border-validated (schemaVersion == "2.0") before
// serving; a malformed blob is 422, never shipped to the runtime player (whose
// own full structural validation is the deep source of truth).
//
// `hash` is P2-08/D5's definition-revision signal: a stable sha256 of the
// stored bytes, deterministic per byte-identical row (same course_definition
// blob → same hash, always — see course_definition_test.go). The frontend
// carries it through to the runtime player, which compares it against a
// resumed CourseSession's own recorded `courseDefinitionHash`
// (@mind-imprint/course-contract's `isCourseSessionStale`) to detect a course
// edited out from under an in-flight session and reset rather than restore
// stale slice/step/block state that may reference removed ids.

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
	sum := sha256.Sum256(def)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"definition": json.RawMessage(def),
		"hash":       hex.EncodeToString(sum[:]),
	})
}
