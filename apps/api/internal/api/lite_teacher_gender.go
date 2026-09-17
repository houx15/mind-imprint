package api

import (
	"encoding/json"
	"net/http"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteworkspace"
	"mindimprint/api/internal/store/sqlc"
)

// putLiteStudentGender handles
// PUT /api/v1/lite/teacher/classes/{id}/students/{userId}/gender.
//
// Body {"gender": "female" | "male" | ""}; "" clears the setting. The teacher
// sets it on the student page, and the AI on the teacher end (class summary,
// class chat, parent report) uses it to pick 她 or 他. With no gender set the
// AI uses neither.
func (a *API) putLiteStudentGender(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	var req struct {
		Gender *string `json:"gender"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	if req.Gender == nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_gender", "缺少 gender", nil))
		return
	}
	gender, valid := liteworkspace.ParseGender(*req.Gender)
	if !valid {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_gender", "gender 只能是 female、male 或空", nil))
		return
	}
	var stored *string
	if gender != "" {
		stored = &gender
	}
	if err := a.d.Queries.SetLiteStudentGender(r.Context(), sqlc.SetLiteStudentGenderParams{UserID: userID, Gender: stored}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"gender": gender})
}
