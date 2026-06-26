package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/org"
	"mindimprint/api/internal/store/sqlc"
)

type importRow struct {
	Class        string `json:"class"`
	TeacherEmail string `json:"teacher_email"`
	StudentEmail string `json:"student_email"`
}

// adminImport provisions org structure from parsed rows in one transaction.
// Idempotent per (school, class name): an existing class is reused, not recreated.
// A distinct teacher_email yields one invite per address it appears with.
// Student rows create no DB artifacts — the class join code is the binding mechanism.
func (a *API) adminImport(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var body struct {
		Rows []importRow `json:"rows"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Validate up front: every row must have a non-empty class name.
	// Nothing is created if any row fails (whole-request rejection before tx).
	for i, row := range body.Rows {
		if strings.TrimSpace(row.Class) == "" {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "班级名称不能为空", map[string]any{"row": i}))
			return
		}
	}

	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	classByName := map[string]sqlc.Class{}
	invitedTeachers := map[string]string{} // email → code, deduped across rows
	type classOut struct{ Name, JoinCode string }
	var classes []classOut

	for _, row := range body.Rows {
		name := strings.TrimSpace(row.Class)
		cls, ok := classByName[name]
		if !ok {
			// Reuse an existing class with this (school, name), else create one.
			existing, gerr := qtx.GetClassBySchoolAndName(r.Context(), sqlc.GetClassBySchoolAndNameParams{
				SchoolID: u.SchoolID,
				Name:     name,
			})
			switch {
			case gerr == nil:
				cls = existing
			case errors.Is(gerr, pgx.ErrNoRows):
				code, cerr := org.NewClassJoinCode()
				if cerr != nil {
					httpx.WriteError(w, r, cerr)
					return
				}
				cls, cerr = qtx.CreateClass(r.Context(), sqlc.CreateClassParams{
					SchoolID:  u.SchoolID,
					Name:      name,
					JoinCode:  code,
					CreatedBy: pgtype.UUID{Bytes: u.ID, Valid: true},
				})
				if cerr != nil {
					httpx.WriteError(w, r, cerr)
					return
				}
			default:
				httpx.WriteError(w, r, gerr)
				return
			}
			classByName[name] = cls
			classes = append(classes, classOut{Name: cls.Name, JoinCode: cls.JoinCode})
		}

		if te := strings.TrimSpace(strings.ToLower(row.TeacherEmail)); te != "" {
			if _, done := invitedTeachers[te]; !done {
				code, cerr := org.NewTeacherInviteCode()
				if cerr != nil {
					httpx.WriteError(w, r, cerr)
					return
				}
				email := te
				if _, ierr := qtx.CreateTeacherInvite(r.Context(), sqlc.CreateTeacherInviteParams{
					SchoolID:  u.SchoolID,
					Code:      code,
					Email:     &email,
					CreatedBy: u.ID,
					ExpiresAt: nowPlusDays(defaultInviteTTLDays),
				}); ierr != nil {
					httpx.WriteError(w, r, ierr)
					return
				}
				invitedTeachers[te] = code
			}
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	invites := make([]map[string]string, 0, len(invitedTeachers))
	for email, code := range invitedTeachers {
		invites = append(invites, map[string]string{"email": email, "code": code})
	}
	classesOut := make([]map[string]string, 0, len(classes))
	for _, c := range classes {
		classesOut = append(classesOut, map[string]string{"name": c.Name, "join_code": c.JoinCode})
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"classes": classesOut, "teacher_invites": invites})
}
