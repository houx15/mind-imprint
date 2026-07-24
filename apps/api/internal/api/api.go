package api

import (
	"context"
	"net/http"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// Fetcher turns a student-supplied URL into readable text. Injected so tests
// can bypass the production SSRF guard (httptest binds loopback, which the
// guard blocks by design). Never used by the agent — only the student's
// explicit POST /projects/{id}/materials reaches it (RL-2, spec §4).
type Fetcher interface {
	FetchReadable(ctx context.Context, rawURL string) (title, text string, err error)
}

// Deps are everything the handlers need, wired once at startup.
type Deps struct {
	Queries      *sqlc.Queries
	Provider     gateway.Provider    // the MuxProvider
	ChatResolver gateway.KeyResolver // chaperone (turn)
	EvalResolver gateway.KeyResolver // flagship (course step render)
	Catalog      []cards.Spec
	SpecByID     func(id string) (cards.Spec, bool)
	Pool         TxBeginner   // for multi-statement transactions (signup)
	CookieSecure bool         // Secure flag on the session cookie
	Voice        VoiceService // TTS/ASR seam; nil disables voice routes (503)
	CORSOrigins  []string     // allowlisted SPA origins, used for WS OriginPatterns
	Fetcher      Fetcher      // URL→readable-text seam for student material ingestion (Slice 6b Task 4)
}

// API holds the handler dependencies.
type API struct{ d Deps }

// New builds the API handler set.
func New(d Deps) *API { return &API{d: d} }

// Handler returns the /api/v1 mux. Auth routes (except /me) are public; every
// other route requires a resolved session. SessionAuth runs for all requests
// (so signout can read the cookie); RequireUser guards the protected group.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()

	// Public auth routes.
	mux.HandleFunc("POST /api/v1/auth/signup", a.signup)
	mux.HandleFunc("POST /api/v1/auth/signin", a.signin)
	mux.HandleFunc("POST /api/v1/auth/signout", a.signout)
	mux.HandleFunc("POST /api/v1/auth/verify-email", a.verifyEmail)

	// Protected routes (require a session).
	protected := func(h http.HandlerFunc) http.Handler { return RequireUser(h) }
	mux.Handle("GET /api/v1/auth/me", protected(a.me))
	mux.Handle("GET /api/v1/growth/history", protected(a.getGrowthHistory))
	mux.Handle("GET /api/v1/growth/ability", protected(a.getAbilityModel))
	mux.Handle("GET /api/v1/growth/cards", protected(a.getGrowthCards))
	mux.Handle("GET /api/v1/projects", protected(a.listProjects))
	mux.Handle("POST /api/v1/projects", protected(a.createProject))
	mux.Handle("POST /api/v1/projects/{id}/onboarding", protected(a.submitOnboarding))
	mux.Handle("POST /api/v1/projects/{id}/framing", protected(a.submitFraming))
	mux.Handle("POST /api/v1/projects/{id}/perspectives", protected(a.submitPerspectives))
	mux.Handle("POST /api/v1/projects/{id}/journey/reopen/{code}", protected(a.reopenStation))
	mux.Handle("GET /api/v1/projects/{id}", protected(a.getProject))
	mux.Handle("POST /api/v1/projects/{id}/materials", protected(a.ingestMaterial))
	mux.Handle("POST /api/v1/projects/{id}/materials/{mid}/open", protected(a.logSourceOpen))
	mux.Handle("PUT /api/v1/projects/{id}/buffer", protected(a.putEditBuffer))
	mux.Handle("POST /api/v1/projects/{id}/snapshots", protected(a.commitSnapshot))
	mux.Handle("POST /api/v1/projects/{id}/gate/{contractId}/attest", protected(a.attestGate))
	mux.Handle("POST /api/v1/projects/{id}/contracts/{contractId}/spot-check", protected(a.orderSpotCheck))
	mux.Handle("POST /api/v1/projects/{id}/snapshots/{sid}/review", protected(a.orderReview))
	mux.Handle("POST /api/v1/projects/{id}/self-score", protected(a.submitSelfScore))
	mux.Handle("POST /api/v1/projects/{id}/reflection", protected(a.submitReflection))
	mux.Handle("POST /api/v1/projects/{id}/declaration/sign", protected(a.signDeclaration))
	mux.Handle("GET /api/v1/projects/{id}/assessment", protected(a.getAssessment))
	mux.Handle("POST /api/v1/projects/{id}/finish", protected(a.finishProject))
	mux.Handle("POST /api/v1/projects/{id}/turn", protected(a.postProjectTurn))
	mux.Handle("POST /api/v1/projects/{id}/interventions/{iid}/disposition", protected(a.postInterventionDisposition))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/activate", protected(a.activateProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/skip", protected(a.skipProjectCard))
	mux.Handle("POST /api/v1/projects/{id}/cards/{cid}/submit", protected(a.submitProjectCard))
	mux.Handle("GET /api/v1/courses", protected(a.listCourses))
	mux.Handle("GET /api/v1/courses/{id}", protected(a.getCourse))
	mux.Handle("GET /api/v1/courses/{id}/progress", protected(a.getCourseProgress))
	mux.Handle("PUT /api/v1/courses/{id}/progress", protected(a.putCourseProgress))
	mux.Handle("POST /api/v1/courses/{id}/steps/{ordinal}/render", protected(a.renderCourseStep))
	mux.Handle("POST /api/v1/courses/{id}/session", protected(a.startCourseSession))
	mux.Handle("POST /api/v1/courses/{id}/session/restart", protected(a.restartCourseSession))
	mux.Handle("GET /api/v1/courses/{id}/session", protected(a.getCourseSession))
	mux.Handle("POST /api/v1/courses/{id}/session/ask", protected(a.postCourseAsk))
	mux.Handle("POST /api/v1/courses/{id}/session/advance", protected(a.postCourseAdvance))
	mux.Handle("POST /api/v1/courses/{id}/session/cards/{cid}/submit", protected(a.submitCourseCard))
	mux.Handle("POST /api/v1/courses/{id}/session/cards/{cid}/skip", protected(a.skipCourseCard))
	mux.Handle("GET /api/v1/courses/{id}/session/assessment", protected(a.getCourseAssessment))
	mux.Handle("POST /api/v1/courses/{id}/session/assessment", protected(a.generateCourseAssessment))
	mux.Handle("POST /api/v1/voice/tts", protected(a.postVoiceTTS))
	mux.Handle("GET /api/v1/voice/asr", protected(a.getVoiceASR))
	mux.Handle("GET /api/v1/chat/threads", protected(a.getChatThreads))
	mux.Handle("POST /api/v1/chat/threads", protected(a.createChatThread))
	mux.Handle("GET /api/v1/chat/threads/{id}/messages", protected(a.getChatMessages))
	mux.Handle("POST /api/v1/chat/threads/{id}/turn", protected(a.postChatTurn))
	mux.Handle("POST /api/v1/chat/threads/{id}/cards/{cid}/submit", protected(a.submitChatCard))
	mux.Handle("POST /api/v1/chat/threads/{id}/cards/{cid}/skip", protected(a.skipChatCard))
	mux.Handle("GET /api/v1/chat/threads/{id}/assessment", protected(a.getChatAssessment))
	mux.Handle("POST /api/v1/chat/threads/{id}/assessment", protected(a.generateChatAssessment))

	// Admin-only routes (require a session + admin role).
	adminOnly := func(h http.HandlerFunc) http.Handler {
		return RequireUser(RequireRole("admin")(http.HandlerFunc(h)))
	}
	mux.Handle("POST /api/v1/admin/teacher-invites", adminOnly(a.createTeacherInvite))
	mux.Handle("GET /api/v1/admin/teacher-invites", adminOnly(a.listTeacherInvites))
	mux.Handle("POST /api/v1/admin/import", adminOnly(a.adminImport))
	mux.Handle("GET /api/v1/admin/overview", adminOnly(a.adminOverview))
	mux.Handle("GET /api/v1/admin/teachers", adminOnly(a.adminListTeachers))
	mux.Handle("POST /api/v1/classes/{id}/teachers", adminOnly(a.assignClassTeacher))
	mux.Handle("DELETE /api/v1/classes/{id}/teachers/{userId}", adminOnly(a.removeClassTeacher))

	// Teacher-or-admin routes (require a session + teacher or admin role).
	teacherOrAdmin := func(h http.HandlerFunc) http.Handler {
		return RequireUser(RequireRole("teacher", "admin")(http.HandlerFunc(h)))
	}
	mux.Handle("POST /api/v1/classes", teacherOrAdmin(a.createClass))
	mux.Handle("GET /api/v1/classes", teacherOrAdmin(a.listClasses))
	mux.Handle("GET /api/v1/classes/{id}", teacherOrAdmin(a.getClass))
	mux.Handle("PATCH /api/v1/classes/{id}", teacherOrAdmin(a.patchClass))
	mux.Handle("DELETE /api/v1/classes/{id}/enrollments/{userId}", teacherOrAdmin(a.removeEnrollment))
	mux.Handle("GET /api/v1/classes/{id}/roster-report", teacherOrAdmin(a.getClassRosterReport))
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}", teacherOrAdmin(a.getStudentDetail))
	mux.Handle("GET /api/v1/classes/{id}/students/{userId}/reports/{surface}/{scopeId}", teacherOrAdmin(a.getStudentReport))
	mux.Handle("GET /api/v1/classes/{id}/weekly-report", teacherOrAdmin(a.getClassWeeklyReport))
	mux.Handle("POST /api/v1/classes/{id}/weekly-report/prose", teacherOrAdmin(a.postClassWeeklyProse))

	return SessionAuth(a.d.Queries)(mux)
}
