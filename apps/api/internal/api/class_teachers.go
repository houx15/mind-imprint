package api

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// loadAdminClass parses the {id} path value and loads the class, enforcing that
// it belongs to the admin's own school. Cross-school / missing → 404 (existence
// hiding), matching the P3.1 tenancy guards.
func (a *API) loadAdminClass(r *http.Request) (sqlc.Class, error) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
	}
	cls, err := a.d.Queries.GetClassByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
		}
		return sqlc.Class{}, err
	}
	if cls.SchoolID != u.SchoolID {
		return sqlc.Class{}, httpx.ErrNotFound("资源不存在")
	}
	return cls, nil
}

// assignClassTeacher enrolls a teacher (of the admin's school) into the class as
// role_in_class='teacher'. Idempotent (upsert). adminOnly.
func (a *API) assignClassTeacher(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	cls, err := a.loadAdminClass(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var body struct {
		TeacherUserID string `json:"teacher_user_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	tid, err := uuid.Parse(body.TeacherUserID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "teacher_user_id 无效", nil))
		return
	}
	// The target must be a teacher in the admin's school (mirrors createClass).
	tu, err := a.d.Queries.GetUserByIDInSchool(r.Context(), sqlc.GetUserByIDInSchoolParams{ID: tid, SchoolID: u.SchoolID})
	if err != nil || tu.Role != "teacher" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "指定的教师无效", nil))
		return
	}
	if err := a.d.Queries.AssignClassTeacher(r.Context(), sqlc.AssignClassTeacherParams{UserID: tid, ClassID: cls.ID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	a.writeClassTeachers(w, r, cls.ID)
}

// removeClassTeacher drops a teacher enrollment from the class. adminOnly.
func (a *API) removeClassTeacher(w http.ResponseWriter, r *http.Request) {
	cls, err := a.loadAdminClass(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.d.Queries.DeleteClassTeacher(r.Context(), sqlc.DeleteClassTeacherParams{ClassID: cls.ID, UserID: userID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) writeClassTeachers(w http.ResponseWriter, r *http.Request, classID uuid.UUID) {
	rows, err := a.d.Queries.GetClassTeachers(r.Context(), classID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]teacherDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, teacherDTO{ID: t.ID.String(), DisplayName: t.DisplayName, Email: t.Email})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"teachers": out})
}
