package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

func (a *API) listTasks(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListTasksByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]taskDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, toTaskDTO(t))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"tasks": out})
}

func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	var body struct {
		Title string  `json:"title"`
		Seed  *string `json:"seed"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(body.Title) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "title 不能为空", nil))
		return
	}
	t, err := a.d.Queries.CreateTask(r.Context(), sqlc.CreateTaskParams{
		UserID: u.ID,
		Title:  body.Title,
		Seed:   body.Seed,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"task": toTaskDTO(t)})
}

// loadOwnedTask parses {id} and confirms the request user owns it. On any
// failure it writes a 404 envelope and returns ok=false (ownership is hidden as
// not-found, never 403, so task existence doesn't leak).
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

func (a *API) getTask(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	msgs, err := a.d.Queries.ListMessagesByTask(r.Context(), t.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	crds, err := a.d.Queries.ListCardsByTask(r.Context(), t.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"task":     toTaskDTO(t),
		"messages": toMessageDTOs(msgs),
		"cards":    toCardDTOs(crds),
	})
}
