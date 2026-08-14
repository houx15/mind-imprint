package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// finishProject is the project's terminal (A3), now NON-BLOCKING (BE5). After
// the guards (ownership → entitlement → already-finished/evaluating 409 →
// reflection-done gate), it flips status to 'evaluating', returns 202 {status:
// "evaluating"} immediately, and a DETACHED goroutine (context.Background(), NOT
// the request context) generates the flagship report. On success it marks the
// project 'finished' + appends project_finished; on failure/reject it rolls
// status back to 'active' (retryable).
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

	// Gate guard (Slice 5 · #20): the ESSAY (final paper) must be finished
	// (完成写作) before the project can be finalized — the two-stage 写作→回顾 flow
	// archives only after the essay is locked (Phase B: the proposal doc's finish
	// does NOT gate 定稿). Defensive: the UI already gates 定稿 behind 完成写作, but
	// never trust the client.
	if _, ferr := a.d.Queries.GetWritingFinish(r.Context(), sqlc.GetWritingFinishParams{ProjectID: projectID, DocKind: string(agent.DocEssay)}); errors.Is(ferr, pgx.ErrNoRows) {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "writing_not_finished",
			Message: "先在写作房间完成写作，再定稿评估",
		})
		return
	} else if ferr != nil {
		httpx.WriteError(w, r, ferr)
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
	// Mechanism-1 finish trigger (best-effort, additive): snapshot every
	// writing/proposal artifact at the moment the project locks for evaluation.
	a.recordCheckpoints(r.Context(), projectID, triggerFinish, nil,
		checkpointDraft, checkpointOutline, checkpointSnippets, checkpointProposal, checkpointClaim)
	go a.runProjectReport(u, projectID)

	httpx.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "evaluating"})
}

// runProjectReport is the detached finish worker. It runs on a fresh
// context.Background() (the request context is already gone) carrying the user
// (WithUser) so cost rows record. On a successful report it marks the project
// 'finished' and appends project_finished. On any failure it rolls status back
// to 'active' so the terminal stays retryable.
func (a *API) runProjectReport(u User, projectID uuid.UUID) {
	ctx := WithUser(context.Background(), u)
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	if gerr := a.generateAndStoreEvaluationReport(ctx, projectID); gerr != nil {
		slog.Error("finish worker: evaluation report generation failed — reverting to active", "err", gerr, "project", projectID)
		if aerr := a.d.Queries.SetProjectActive(ctx, projectID); aerr != nil {
			slog.Error("finish worker: revert to active failed", "err", aerr, "project", projectID)
		}
		return
	}

	if err := a.d.Queries.SetProjectFinished(ctx, projectID); err != nil {
		slog.Error("finish worker: mark finished failed", "err", err, "project", projectID)
		return
	}
	if err := store.AppendEvent(ctx, agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "project_finished",
		Payload: mustJSON(map[string]any{"generatedAt": time.Now().UTC().Format(time.RFC3339)}),
	}); err != nil {
		slog.Warn("finish worker: append project_finished event", "err", err)
	}
}
