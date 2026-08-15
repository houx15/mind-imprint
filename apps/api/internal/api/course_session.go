package api

// course_session.go — Course Runtime Slice 8: CourseSession (§16) persistence.
// The 2.0 runtime holds authoritative session state client-side and snapshots
// the whole blob here per (user, course):
//   - POST /api/v1/courses/{slug}/session → get-or-create → { session }
//   - PUT  /api/v1/courses/{slug}/session  → snapshot-save → { ok: true }
// Owner-scoped by the authed user: a second student always gets their own new
// session (get-or-create keys by user_id), and can only ever overwrite their
// own. On first create the server mints the initial CourseSession from the
// stored 2.0 definition (course.id) + the authed user (studentId) — the runtime
// never dictates identity or authorship.

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// courseSessionInit is the initial CourseSession blob the server mints on first
// create. Field set matches @mind-imprint/course-contract's CourseSession
// (.strict()) required fields; the optional scene/current fields are omitted
// until the runtime fills them via snapshot-save. sliceStates/events are
// initialized non-nil so they marshal as {}/[] (never null), which the client
// Zod schema requires.
type courseSessionInit struct {
	ID                  string                     `json:"id"`
	CourseID            string                     `json:"courseId"`
	CourseSchemaVersion string                     `json:"courseSchemaVersion"`
	StudentID           string                     `json:"studentId"`
	Status              string                     `json:"status"`
	SliceStates         map[string]json.RawMessage `json:"sliceStates"`
	Events              []json.RawMessage          `json:"events"`
}

// postCourseSession get-or-creates the authed student's CourseSession for one
// course. An existing session is returned as-is (resume); otherwise a fresh
// "created" session is minted from the stored 2.0 definition.
func (a *API) postCourseSession(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// Resolve the course uuid (404 on an unknown slug).
	_, courseID, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Existing session → return verbatim (resume). Owner-scoped by user id.
	if existing, _, found, gerr := store.GetCourseSession(r.Context(), user.ID, slug); gerr != nil {
		httpx.WriteError(w, r, gerr)
		return
	} else if found {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"session": json.RawMessage(existing)})
		return
	}

	// No session yet — mint one. The 2.0 definition's course.id is the session's
	// courseId (a course-id primitive, not the DB uuid). A course without a 2.0
	// definition has no runtime session (404).
	def, err := store.GetCourseDefinition(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, r, httpx.ErrNotFound("课程不存在"))
			return
		}
		httpx.WriteError(w, r, err)
		return
	}
	if len(def) == 0 {
		httpx.WriteError(w, r, httpx.ErrNotFound("该课程没有 2.0 定义"))
		return
	}
	var docHead struct {
		Course struct {
			ID string `json:"id"`
		} `json:"course"`
	}
	if err := json.Unmarshal(def, &docHead); err != nil || docHead.Course.ID == "" {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "invalid_course_definition", Message: "课程定义格式无效",
		})
		return
	}

	initial := courseSessionInit{
		ID:                  uuid.NewString(),
		CourseID:            docHead.Course.ID,
		CourseSchemaVersion: "2.0",
		StudentID:           user.ID.String(),
		Status:              "created",
		SliceStates:         map[string]json.RawMessage{},
		Events:              []json.RawMessage{},
	}
	raw, err := json.Marshal(initial)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	stored, err := store.CreateCourseSession(r.Context(), user.ID, courseID, raw, initial.Status)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"session": json.RawMessage(stored)})
}

// putCourseSession snapshot-writes the whole CourseSession blob. Owner-scoped:
// the WHERE user_id keeps a student from overwriting anyone else's session.
func (a *API) putCourseSession(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	slug := r.PathValue("slug")
	var body struct {
		Session json.RawMessage `json:"session"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(body.Session) == 0 {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "session 不能为空", nil))
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	_, courseID, err := store.GetCoursePayload(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var meta struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(body.Session, &meta)
	if err := store.SaveCourseSession(r.Context(), user.ID, courseID, body.Session, meta.Status); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
}
