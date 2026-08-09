package api

import (
	"encoding/json"
	"net/http"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// essay_track.go — slice 4a · the essay's stage flip. Research (证据地图 in the
// reading room) → statement (the writing surface). The flexible advance (§6): the
// student can move to writing once a sub-question is ready OR when all research
// is done. Advisory — this is the student's tap (铁律②). 4a lands research→
// statement; 4b/4c own the statement/submission internals.

// GET /projects/{id}/essay-track → { stage }
func (a *API) getEssayTrack(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	stage := ""
	if raw, err := a.d.Queries.GetStudioState(r.Context(), projectID); err == nil && len(raw) > 0 {
		var st agent.StudioState
		if json.Unmarshal(raw, &st) == nil && st.EssayTrack != nil {
			stage = string(st.EssayTrack.Stage)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"stage": stage})
}

// POST /projects/{id}/essay-track/advance-stage { stage }
// Moves the essay stage forward (research→statement→submission) and opens the
// surface that stage works on (statement/submission = the writing room).
func (a *API) advanceEssayStage(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Stage string `json:"stage"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	target := agent.EssayStage(body.Stage)
	if target != agent.EssayStatement && target != agent.EssaySubmission {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "stage 必须是 statement / submission", nil))
		return
	}
	state := agent.DefaultStudioState()
	if raw, err := a.d.Queries.GetStudioState(r.Context(), projectID); err == nil && len(raw) > 0 {
		if uerr := json.Unmarshal(raw, &state); uerr != nil {
			httpx.WriteError(w, r, uerr)
			return
		}
	}
	if state.EssayTrack == nil {
		state.EssayTrack = &agent.EssayTrack{}
	}
	state.EssayTrack.Stage = target
	// statement / submission are written in the writing room.
	state.OpenTool = agent.ToolWriting
	state.WidthTier = agent.WidthForTool(agent.ToolWriting)
	b, err := json.Marshal(state)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.d.Queries.SetStudioState(r.Context(), sqlc.SetStudioStateParams{ID: projectID, StudioState: b}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"stage": string(target), "surface": string(agent.ToolWriting)})
}
