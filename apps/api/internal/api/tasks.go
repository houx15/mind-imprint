package api

import (
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// loadOwnedTask parses {id} and confirms the request user owns it. On any
// failure it writes a 404 envelope and returns ok=false (ownership is hidden as
// not-found, never 403, so task existence doesn't leak).
//
// The old /api/v1/tasks/* HTTP surface (list/create/get/turn/cards/materials)
// was retired in Slice 5d, but this helper survives: postEvaluate and
// getEvaluation (evaluate.go) — owned by a later slice's evaluator retirement
// — still scope /api/v1/tasks/{id}/evaluate[/evaluation] to the request user
// via GetTask, so this file (and the GetTask query) stay.
func (a *API) loadOwnedTask(w http.ResponseWriter, r *http.Request) (sqlc.Task, bool) {
	u, _ := UserFromContext(r.Context())
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Task{}, false
	}
	t, err := a.d.Queries.GetTask(r.Context(), id)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return sqlc.Task{}, false
	}
	if t.UserID != u.ID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return sqlc.Task{}, false
	}
	return t, true
}
