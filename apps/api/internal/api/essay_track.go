package api

import (
	"encoding/json"
	"log/slog"
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
		Stage   string `json:"stage"`
		ClaimID string `json:"claimId"` // slice 4b-2 · deep-link: land on this claim's step
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
	enteringStatement := state.EssayTrack.Stage != agent.EssayStatement && target == agent.EssayStatement
	state.EssayTrack.Stage = target
	// slice 4b · entering the statement stage seeds the essay outline from the RQ
	// + sub-questions if it's empty (§6). Best-effort.
	if enteringStatement {
		mainRQ := ""
		if prop, perr := a.d.Queries.GetProjectProposal(r.Context(), projectID); perr == nil {
			mainRQ = prop.Objective
		}
		var subs []agent.SubQuestion
		if state.ProposalTrack != nil {
			subs = state.ProposalTrack.SubQuestions
		}
		store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
		if serr := store.SeedEssayOutlineFromProposal(r.Context(), projectID, mainRQ, subs); serr != nil {
			slog.Warn("advance essay stage: seed outline failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	// slice 4b-2 · deep-link from the 4a research panel's 去写这条论点: land the
	// student directly on that claim's statement step (§133). Started so the walk
	// skips the ready gate.
	if target == agent.EssayStatement && body.ClaimID != "" {
		var subs []agent.SubQuestion
		if state.ProposalTrack != nil {
			subs = state.ProposalTrack.SubQuestions
		}
		steps := agent.DeriveStatementSteps(subs)
		for i, s := range steps {
			if s.Key == "claim:"+body.ClaimID {
				state.EssayTrack.Started = true
				state.EssayTrack.StatementStep = i
				break
			}
		}
	}
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
