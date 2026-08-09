package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// framework_waive.go — slice 3a (Finding 2) · the non-blocking skip for the 反例
// prompt. The framework prompts for 可能的反例 before generating the plan; a
// student who can't articulate one taps this to proceed anyway (铁律②). Sets
// studio_state.counterpointsWaived so frameworkReadyForPlan lets the funnel run.
func (a *API) waiveCounterpoints(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	state := agent.DefaultStudioState()
	if raw, err := a.d.Queries.GetStudioState(r.Context(), projectID); err == nil && len(raw) > 0 {
		if uerr := json.Unmarshal(raw, &state); uerr != nil {
			httpx.WriteError(w, r, uerr)
			return
		}
	}
	state.CounterpointsWaived = true
	b, err := json.Marshal(state)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.Queries.SetStudioState(r.Context(), sqlc.SetStudioStateParams{ID: projectID, StudioState: b}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Reconcile now so the plan can generate on this call if the four dims are set.
	if reconciled, autoPlan := a.reconcileStudioFunnel(r.Context(), projectID, state); autoPlan || reconciled.Stage != state.Stage {
		if rb, merr := json.Marshal(reconciled); merr == nil {
			if serr := a.d.Queries.SetStudioState(r.Context(), sqlc.SetStudioStateParams{ID: projectID, StudioState: rb}); serr != nil {
				slog.Warn("waive counterpoints: persist reconciled state failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"counterpointsWaived": true})
}
