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
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/students/{userId}", liteTeacher(a.getLiteStudentPage))
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}", liteTeacher(a.getLiteTeacherItem))
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/students/{userId}/tree", liteTeacher(a.getLiteTeacherTree))

	// Weekly summary. A GET never calls a model; only the POST …/prose does.
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/students/{userId}/weekly", liteTeacher(a.getLiteStudentWeekly))
	mux.Handle("POST /api/v1/lite/teacher/classes/{id}/students/{userId}/weekly/prose", liteTeacher(a.postLiteStudentWeeklyProse))
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/weekly", liteTeacher(a.getLiteClassWeekly))
	mux.Handle("POST /api/v1/lite/teacher/classes/{id}/weekly/prose", liteTeacher(a.postLiteClassWeeklyProse))

	// Assignments. POST …/assignments/extract and the {aid} routes differ by
	// method, so the Go 1.22 mux registers them without a pattern conflict.
	mux.Handle("POST /api/v1/lite/teacher/classes/{id}/assignments", liteTeacher(a.createLiteAssignment))
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/assignments", liteTeacher(a.listLiteAssignments))
	mux.Handle("GET /api/v1/lite/teacher/assignments/{aid}", liteTeacher(a.getLiteAssignment))
	mux.Handle("PATCH /api/v1/lite/teacher/assignments/{aid}", liteTeacher(a.patchLiteAssignment))
	mux.Handle("DELETE /api/v1/lite/teacher/assignments/{aid}", liteTeacher(a.archiveLiteAssignment))
	mux.Handle("POST /api/v1/lite/teacher/assignments/extract", liteTeacher(a.extractLiteAssignment))

	// Parent reports. Only the generate POST loads facts (which may create
	// phase-1 student reports); only generate and redraft call a model. Every
	// mutating {rid} route locks the report row.
	mux.Handle("POST /api/v1/lite/teacher/classes/{id}/students/{userId}/parent-reports", liteTeacher(a.createLiteParentReport))
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/students/{userId}/parent-reports", liteTeacher(a.listLiteStudentParentReports))
	mux.Handle("GET /api/v1/lite/teacher/classes/{id}/parent-reports", liteTeacher(a.listLiteClassParentReports))
	mux.Handle("GET /api/v1/lite/teacher/parent-reports/{rid}", liteTeacher(a.getLiteParentReport))
	mux.Handle("PATCH /api/v1/lite/teacher/parent-reports/{rid}", liteTeacher(a.patchLiteParentReport))
	mux.Handle("POST /api/v1/lite/teacher/parent-reports/{rid}/redraft", liteTeacher(a.redraftLiteParentReport))
	mux.Handle("POST /api/v1/lite/teacher/parent-reports/{rid}/publish", liteTeacher(a.publishLiteParentReport))
	mux.Handle("DELETE /api/v1/lite/teacher/parent-reports/{rid}/share", liteTeacher(a.revokeLiteParentReportShare))
}
