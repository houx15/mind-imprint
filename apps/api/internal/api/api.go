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
}

// API holds the handler dependencies.
type API struct{ d Deps }

// New builds the API handler set.
func New(d Deps) *API { return &API{d: d} }

// Handler returns the /api/v1 mux wrapped by the ActAsSeed dev middleware.
// Task 6 registers only the 3 task routes; Tasks 7/8/9 append the remaining routes.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/tasks", a.listTasks)
	mux.HandleFunc("POST /api/v1/tasks", a.createTask)
	mux.HandleFunc("GET /api/v1/tasks/{id}", a.getTask)
	mux.HandleFunc("PATCH /api/v1/tasks/{id}/cards/{cid}", a.patchCard)
	mux.HandleFunc("PUT /api/v1/tasks/{id}/cards/{cid}", a.putCard)
	mux.HandleFunc("POST /api/v1/tasks/{id}/cards/{cid}/skip", a.skipCard)
	return ActAsSeed(a.d.Queries)(mux)
}
