package api

import "net/http"

// registerLiteTeacherRoutes mounts the lite teacher end. Every route is lite
// edition AND teacher/admin role; student-scoped routes additionally go
// through authTeacherStudent inside the handler.
func (a *API) registerLiteTeacherRoutes(mux *http.ServeMux) {
	liteTeacher := func(h http.HandlerFunc) http.Handler {
		return a.requireEdition("lite", RequireRole("teacher", "admin")(h))
	}
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/roster", liteTeacher(a.getLiteClassRoster))
}
