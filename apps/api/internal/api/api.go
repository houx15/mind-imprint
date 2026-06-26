package api

import (
	"net/http"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// Deps are everything the handlers need, wired once at startup.
type Deps struct {
	Queries      *sqlc.Queries
	Provider     gateway.Provider    // the MuxProvider
	ChatResolver gateway.KeyResolver // chaperone (turn)
	EvalResolver gateway.KeyResolver // flagship (evaluate)
	Catalog      []cards.Spec
	SpecByID     func(id string) (cards.Spec, bool)
	Pool         TxBeginner // for multi-statement transactions (signup)
	CookieSecure bool       // Secure flag on the session cookie
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
	mux.Handle("GET /api/v1/tasks", protected(a.listTasks))
	mux.Handle("POST /api/v1/tasks", protected(a.createTask))
	mux.Handle("GET /api/v1/tasks/{id}", protected(a.getTask))
	mux.Handle("PATCH /api/v1/tasks/{id}/cards/{cid}", protected(a.patchCard))
	mux.Handle("PUT /api/v1/tasks/{id}/cards/{cid}", protected(a.putCard))
	mux.Handle("POST /api/v1/tasks/{id}/cards/{cid}/skip", protected(a.skipCard))
	mux.Handle("POST /api/v1/tasks/{id}/turn", protected(a.postTurn))
	mux.Handle("POST /api/v1/tasks/{id}/evaluate", protected(a.postEvaluate))
	mux.Handle("GET /api/v1/tasks/{id}/evaluation", protected(a.getEvaluation))

	// Admin-only routes (require a session + admin role).
	adminOnly := func(h http.HandlerFunc) http.Handler {
		return RequireUser(RequireRole("admin")(http.HandlerFunc(h)))
	}
	mux.Handle("POST /api/v1/admin/teacher-invites", adminOnly(a.createTeacherInvite))
	mux.Handle("GET /api/v1/admin/teacher-invites", adminOnly(a.listTeacherInvites))
	mux.Handle("POST /api/v1/admin/import", adminOnly(a.adminImport))
	mux.Handle("GET /api/v1/admin/overview", adminOnly(a.adminOverview))
	mux.Handle("GET /api/v1/admin/teachers", adminOnly(a.adminListTeachers))

	// Teacher-or-admin routes (require a session + teacher or admin role).
	teacherOrAdmin := func(h http.HandlerFunc) http.Handler {
		return RequireUser(RequireRole("teacher", "admin")(http.HandlerFunc(h)))
	}
	mux.Handle("POST /api/v1/classes", teacherOrAdmin(a.createClass))
	mux.Handle("GET /api/v1/classes", teacherOrAdmin(a.listClasses))
	mux.Handle("GET /api/v1/classes/{id}", teacherOrAdmin(a.getClass))
	mux.Handle("PATCH /api/v1/classes/{id}", teacherOrAdmin(a.patchClass))
	mux.Handle("DELETE /api/v1/classes/{id}/enrollments/{userId}", teacherOrAdmin(a.removeEnrollment))

	return SessionAuth(a.d.Queries)(mux)
}
