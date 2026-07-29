package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// finishProject is the project's terminal (A3), now NON-BLOCKING (BE5). After
// the guards (ownership → entitlement → already-finished/evaluating 409 →
// reflection-done gate), it flips status to 'evaluating', returns 202 {status:
// "evaluating"} immediately, and a DETACHED goroutine (context.Background(), NOT
// the request context) generates the flagship report. On success it marks the
// project 'finished' + appends project_finished + best-effort composes the
// mirror; on failure/reject it rolls status back to 'active' (retryable).
// finished ⟺ has a terminal report. One report only (DEC-A3.5): a second finish
// while 'finished' or 'evaluating' is refused with 409.
func (a *API) finishProject(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	// One-time / in-flight guard: finished → 409 (no regeneration); evaluating →
	// 409 (a report is already being generated; don't spawn a second).
	proj, err := a.d.Queries.GetProject(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if proj.Status == "finished" {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusConflict, Code: "already_finished", Message: "任务已归档",
		})
		return
	}
	if proj.Status == "evaluating" {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusConflict, Code: "evaluating", Message: "正在评估中",
		})
		return
	}

	// Gate guard (Slice 5): the student must have finished their own reflection
	// (完成回顾) before the project archives. Never trust the client — read the
	// reflection row server-side. No row, or done=false → not yet.
	refl, rerr := a.d.Queries.GetProjectReflection(r.Context(), projectID)
	if rerr != nil && !errors.Is(rerr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, rerr)
		return
	}
	if errors.Is(rerr, pgx.ErrNoRows) || !refl.Done {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "reflection_not_done",
			Message: "先完成回顾再归档",
		})
		return
	}

	// Claim the in-flight state synchronously (so a concurrent finish 409s), then
	// return 202 and generate off the request path.
	if err := a.d.Queries.SetProjectEvaluating(r.Context(), projectID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	go a.runProjectReport(u, projectID)

	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "evaluating"})
}

// runProjectReport is the detached finish worker. It runs on a fresh
// context.Background() (the request context is already gone) carrying the user
// (WithUser) so cost rows record. On a successful report it marks the project
// 'finished', appends project_finished, and best-effort composes the mirror over
// the now-complete process record. On any failure — including a rejected
// assessment — it rolls status back to 'active' so the terminal stays retryable.
func (a *API) runProjectReport(u User, projectID uuid.UUID) {
	ctx := WithUser(context.Background(), u)
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	dto, gerr := a.generateProjectReport(ctx, projectID)
	if gerr != nil {
		if errors.Is(gerr, errAssessmentRejected) {
			slog.Warn("finish worker: assessment rejected — reverting to active", "project", projectID)
		} else {
			slog.Error("finish worker: report generation failed — reverting to active", "err", gerr, "project", projectID)
		}
		if aerr := a.d.Queries.SetProjectActive(ctx, projectID); aerr != nil {
			slog.Error("finish worker: revert to active failed", "err", aerr, "project", projectID)
		}
		return
	}

	// Best-effort real post-completion mirror (persist only on success; BE4).
	// Composed BEFORE flipping to 'finished' so that status='finished' is a clean
	// "report + mirror both done" signal — no window where a client polling on
	// finished races an in-flight mirror.
	a.composeAndStoreProjectMirror(ctx, projectID)

	if err := a.d.Queries.SetProjectFinished(ctx, projectID); err != nil {
		slog.Error("finish worker: mark finished failed", "err", err, "project", projectID)
		return
	}
	if err := store.AppendEvent(ctx, agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "project_finished",
		Payload: mustJSON(map[string]any{"generatedAt": dto.GeneratedAt}),
	}); err != nil {
		slog.Warn("finish worker: append project_finished event", "err", err)
	}
}
