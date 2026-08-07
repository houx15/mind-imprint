package api

import (
	"encoding/json"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// studiostate_handler.go — Task 6 · GET /projects/{id}/studio-state exposes
// the AI-managed studio_state (Task 1's column, Task 5's orchestrator writes)
// so the frontend can resume a project at its stage/openTool/widthTier/
// reference without replaying the whole coach history. Read-only, no spend:
// the orchestrator (POST /coach) is the only writer. The column is
// NOT NULL DEFAULT, so a fresh project already deserializes to
// agent.DefaultStudioState() — no special-casing needed here.
func (a *API) getStudioState(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	raw, err := a.d.Queries.GetStudioState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var state agent.StudioState
	if err := json.Unmarshal(raw, &state); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, state)
}
