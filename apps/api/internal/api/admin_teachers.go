package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
)

// adminListTeachers returns every teacher in the admin's own school (for the
// admin console's teacher picker + roster). School comes from the session,
// never the request.
func (a *API) adminListTeachers(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListTeachersBySchool(r.Context(), u.SchoolID)
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
