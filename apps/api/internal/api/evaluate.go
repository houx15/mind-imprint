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

	// Insert a queued evaluation row, then hand the job off to the async queue.
	ev, err := a.d.Queries.EnqueueEvaluation(r.Context(), t.ID)
	if err != nil {
		// One in-flight eval per task (partial unique index). If one is already
		// queued/running, return it — the client polls it — instead of erroring.
		if isUniqueViolation(err) {
			if existing, gerr := a.d.Queries.GetLatestEvaluation(r.Context(), t.ID); gerr == nil {
				httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"evaluation": toEvaluationDTO(existing)})
				return
			}
		}
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.Enqueuer.EnqueueEvaluate(r.Context(), agent.EvaluateArgs{
		EvaluationID: ev.ID,
		TaskID:       t.ID,
	}); err != nil {
		// Best-effort: mark the row failed so the client stops polling. The real
		// cause is logged, never persisted or leaked.
		_ = a.d.Queries.FailEvaluation(r.Context(), sqlc.FailEvaluationParams{ID: ev.ID, Error: ptrStr("入队失败")})
		slog.Error("evaluation enqueue failed", "task_id", t.ID.String(), "err", err.Error())
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{"evaluation": toEvaluationDTO(ev)})
}

// ptrStr returns a pointer to s (sqlc nullable string columns take *string).
func ptrStr(s string) *string { return &s }

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
