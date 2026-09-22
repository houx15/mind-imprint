package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/org"
	"mindimprint/api/internal/store/sqlc"
)

// createClass: teacher creates a class they own; admin may create one for any
// teacher in their school via teacher_user_id. The creator/assignee is enrolled
// as role_in_class='teacher'. One transaction (class + enrollment).
func (a *API) createClass(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var body struct {
		Name          string `json:"name"`
		TeacherUserID string `json:"teacher_user_id"`
		Grade         string `json:"grade"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "班级名称不能为空", nil))
		return
	}
	grade, gradeOK := validateClassGrade(strings.TrimSpace(body.Grade))
	if !gradeOK {
		// 不认识的年级是前端的 bug，不是老师填错了字 —— 直接回 400，
		// 不要悄悄存成空串，那样他以为自己填了。
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "年级不在可选范围内", nil))
		return
	}

	// Determine the owning teacher.
	teacherID := u.ID
	if u.Role == "admin" {
		if body.TeacherUserID == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "需要指定班级教师", nil))
			return
		}
		tid, err := uuid.Parse(body.TeacherUserID)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "teacher_user_id 无效", nil))
			return
		}
		// The assignee must be a teacher in the admin's school.
		tu, err := a.d.Queries.GetUserByIDInSchool(r.Context(), sqlc.GetUserByIDInSchoolParams{ID: tid, SchoolID: u.SchoolID})
		if err != nil || tu.Role != "teacher" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "指定的教师无效", nil))
			return
		}
		teacherID = tid
	}

	code, err := org.NewClassJoinCode()
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	cls, err := qtx.CreateClass(r.Context(), sqlc.CreateClassParams{
		SchoolID:  u.SchoolID,
		Name:      strings.TrimSpace(body.Name),
		JoinCode:  code,
		CreatedBy: pgtype.UUID{Bytes: u.ID, Valid: true},
		Grade:     grade, // 已校验过；未填时是合法的空串
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.CreateEnrollment(r.Context(), sqlc.CreateEnrollmentParams{
		UserID: teacherID, ClassID: cls.ID, RoleInClass: "teacher",
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"class": toClassDTO(cls)})
}

func (a *API) getClass(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	cls, err := a.assertTeacherOwnsClass(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.d.Queries.GetClassRoster(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	roster := make([]rosterEntryDTO, 0, len(rows))
	for _, row := range rows {
		roster = append(roster, toRosterEntryDTO(row))
	}
	tRows, err := a.d.Queries.GetClassTeachers(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	teachers := make([]teacherDTO, 0, len(tRows))
	for _, te := range tRows {
		teachers = append(teachers, teacherDTO{ID: te.ID.String(), DisplayName: te.DisplayName, Email: te.Email})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"class": toClassDTO(cls), "roster": roster, "teachers": teachers})
}

func (a *API) listClasses(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var rows []sqlc.Class
	var err error
	if u.Role == "admin" {
		rows, err = a.d.Queries.ListClassesBySchool(r.Context(), u.SchoolID)
	} else {
		rows, err = a.d.Queries.ListClassesForTeacher(r.Context(), u.ID)
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]classDTO, 0, len(rows))
	for _, c := range rows {
		out = append(out, toClassDTO(c))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"classes": out})
}

func (a *API) patchClass(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var body struct {
		Name               *string `json:"name"`
		Grade              *string `json:"grade"`
		RegenerateJoinCode bool    `json:"regenerate_join_code"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var cls sqlc.Class
	changed := false
	if body.Name != nil {
		if strings.TrimSpace(*body.Name) == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "班级名称不能为空", nil))
			return
		}
		cls, err = a.d.Queries.UpdateClassName(r.Context(), sqlc.UpdateClassNameParams{ID: id, Name: strings.TrimSpace(*body.Name)})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		changed = true
	}
	if body.Grade != nil {
		grade, gradeOK := validateClassGrade(strings.TrimSpace(*body.Grade))
		if !gradeOK {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "年级不在可选范围内", nil))
			return
		}
		cls, err = a.d.Queries.SetClassGrade(r.Context(), sqlc.SetClassGradeParams{ID: id, Grade: grade})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		changed = true
	}
	if body.RegenerateJoinCode {
		code, cerr := org.NewClassJoinCode()
		if cerr != nil {
			httpx.WriteError(w, r, cerr)
			return
		}
		cls, err = a.d.Queries.SetClassJoinCode(r.Context(), sqlc.SetClassJoinCodeParams{ID: id, JoinCode: code})
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		changed = true
	}
	if !changed {
		cls, err = a.d.Queries.GetClassByID(r.Context(), id)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"class": toClassDTO(cls)})
}

func (a *API) removeEnrollment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.assertTeacherOwnsClass(r.Context(), id); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	userID, err := uuid.Parse(r.PathValue("userId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.d.Queries.DeleteEnrollment(r.Context(), sqlc.DeleteEnrollmentParams{ClassID: id, UserID: userID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
