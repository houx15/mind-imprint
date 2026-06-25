package api

import (
	"log/slog"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

func (a *API) postEvaluate(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	ev, err := agent.RunEvaluation(r.Context(), agent.EvalDeps{
		Store:    agent.NewSqlcEvalStore(a.d.Queries),
		Provider: a.d.Provider,
		Resolver: a.d.EvalResolver, // FLAGSHIP — never downgraded
		SpecByID: a.d.SpecByID,
		TaskID:   t.ID,
	})
	if err != nil {
		// Log the real cause; return a generic 500 (no internal detail leaked).
		slog.Error("evaluation failed", "task_id", t.ID.String(), "err", err.Error())
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	// Mark the task evaluated (best-effort; the evaluation row is the source of truth).
	_, _ = a.d.Queries.SetTaskEvaluated(r.Context(), sqlc.SetTaskEvaluatedParams{ID: t.ID, UserID: u.ID})

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"evaluation": toEvaluationDTO(ev)})
}

func (a *API) getEvaluation(w http.ResponseWriter, r *http.Request) {
	t, ok := a.loadOwnedTask(w, r)
	if !ok {
		return
	}
	ev, err := a.d.Queries.GetLatestEvaluation(r.Context(), t.ID)
	if err != nil {
		httpx.WriteError(w, r, err) // pgx.ErrNoRows → 404
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"evaluation": toEvaluationDTO(ev)})
}
