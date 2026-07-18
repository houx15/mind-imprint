package api

import (
	"errors"
	"log/slog"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// finishProject is the project's terminal (A3): the one-time, student-initiated
// act that mints the project's growth report. Guards the S5 gate server-side,
// generates the flagship report (cost recorded even on reject), and — only on
// success — marks the project finished and appends a project_finished event.
// finished ⟺ has a terminal report: a rejected report leaves the project active
// (retryable). Already finished → 409 (no regeneration; DEC-A3.5).
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

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)

	// One-time guard: already finished → 409. (Load the row fresh — loadOwnedProject
	// already fetched it, but re-read via studio.Load's project or GetProject to
	// read status.)
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

	// Gate guard: 整稿体检 must have passed. Never trust the client.
	recorded, gerr := store.ListGateStates(r.Context(), projectID)
	if gerr != nil {
		httpx.WriteError(w, r, gerr)
		return
	}
	if recorded["draft_polish"].Items["whole_draft_review"] != "solid" {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "gate_not_met",
			Message: "还没通过整稿体检，先完成整稿体检再归档",
		})
		return
	}

	// Generate the terminal report (flagship; cost recorded even on reject).
	dto, gerr2 := a.generateProjectReport(r.Context(), projectID)
	if errors.Is(gerr2, errAssessmentRejected) {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusUnprocessableEntity, Code: "assessment_rejected",
			Message: "这次评估没通过内部校验，请再试一次",
		})
		return
	}
	if gerr2 != nil {
		httpx.WriteError(w, r, gerr2)
		return
	}

	// Success: mark finished + record the terminal event. finished ⟺ report.
	if err := a.d.Queries.SetProjectFinished(r.Context(), projectID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "project_finished",
		Payload: mustJSON(map[string]any{"generatedAt": dto.GeneratedAt}),
	}); err != nil {
		slog.Warn("finish_project: append event", "err", err)
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}
