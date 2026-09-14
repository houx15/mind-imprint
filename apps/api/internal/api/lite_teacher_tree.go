package api

import (
	"net/http"

	"mindimprint/api/internal/httpx"
)

// getLiteTeacherTree handles
// GET /api/v1/lite/teacher/classes/{id}/students/{userId}/tree: the
// teacher's read-only view of a student's interest tree. Same payload
// buildInterestTree returns to the student herself (interest.go).
func (a *API) getLiteTeacherTree(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	tree, err := a.buildInterestTree(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tree)
}
