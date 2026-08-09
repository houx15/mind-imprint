package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
)

// essay_statement.go — slice 4b · the essay statement stage's guide-step track
// REST, mirroring proposal_track.go. Steps are derived from the sub-questions
// (the claims); the current step's guide card is generated (essay-worded) +
// cached; 我写好了 produces essay 批注 (doc=essay). The walk is guided from the
// coach; this drives which step/card is active.

// loadEssayState loads the studio_state and ensures a non-nil EssayTrack (+ its
// StepGuides map). The caller persists after mutating.
func (a *API) loadEssayState(ctx context.Context, projectID uuid.UUID) (agent.StudioState, error) {
	state := agent.DefaultStudioState()
	if raw, err := a.d.Queries.GetStudioState(ctx, projectID); err == nil && len(raw) > 0 {
		if uerr := json.Unmarshal(raw, &state); uerr != nil {
			return state, uerr
		}
	}
	if state.EssayTrack == nil {
		state.EssayTrack = &agent.EssayTrack{Stage: agent.EssayResearch}
	}
	if state.EssayTrack.StepGuides == nil {
		state.EssayTrack.StepGuides = map[string]string{}
	}
	return state, nil
}

func (a *API) essaySubQuestions(state agent.StudioState) []agent.SubQuestion {
	if state.ProposalTrack != nil {
		return state.ProposalTrack.SubQuestions
	}
	return nil
}

// buildEssayStatementStep derives the current statement step and (started only)
// generates/reads its cached guide card. Mutates state; reports whether changed.
func (a *API) buildEssayStatementStep(ctx context.Context, projectID uuid.UUID, state *agent.StudioState) (proposalGuideStepDTO, bool) {
	et := state.EssayTrack
	subs := a.essaySubQuestions(*state)
	steps := agent.DeriveStatementSteps(subs)
	total := len(steps)
	changed := false

	idx := agent.ClampStepIndex(et.StatementStep, total)
	if idx != et.StatementStep {
		et.StatementStep = idx
		changed = true
	}
	cur := steps[idx]

	if subs == nil {
		subs = []agent.SubQuestion{}
	}
	dto := proposalGuideStepDTO{
		Key: cur.Key, Title: cur.Title, Kind: string(cur.Kind),
		Index: idx, Total: total, Mode: "guided", Started: et.Started, SubQuestions: subs,
	}

	if et.Started {
		if cached := et.StepGuides[cur.Key]; cached != "" {
			var c guideCardDTO
			if json.Unmarshal([]byte(cached), &c) == nil {
				dto.Card = &c
			}
		}
		if dto.Card == nil {
			if c, ok := a.generateEssayGuideCard(ctx, projectID, *state, cur); ok {
				et.StepGuides[cur.Key] = string(mustJSON(c))
				dto.Card = &c
				changed = true
			}
		}
	}
	return dto, changed
}

func (a *API) generateEssayGuideCard(ctx context.Context, projectID uuid.UUID, state agent.StudioState, step agent.Step) (guideCardDTO, bool) {
	if a.d.Provider == nil {
		return guideCardDTO{}, false
	}
	resolved, ok := a.resolveFast(ctx)
	if !ok {
		return guideCardDTO{}, false
	}
	in := agent.GuideGenInput{Step: step, Doc: "essay"}
	if p, err := a.d.Queries.GetProject(ctx, projectID); err == nil {
		in.Title = p.Title
	}
	if prop, err := a.d.Queries.GetProjectProposal(ctx, projectID); err == nil {
		in.Objective = prop.Objective
	}
	if step.Kind == agent.KindSubq {
		in.SiblingSubQuestions = a.essaySubQuestions(state)
		for _, sq := range in.SiblingSubQuestions {
			if sq.ID == step.SubQuestionID {
				in.ThisSubQuestion = sq.Text
			}
		}
	}
	out, usage, err := agent.GenerateProposalGuideStep(ctx, a.d.Provider, resolved, in)
	a.meterCall(ctx, projectID, resolved, "essay_guide", usage)
	if err != nil {
		slog.Warn("essay guide: generation failed", "err", err, "step", step.Key, "request_id", httpx.RequestIDFromContext(ctx))
		return guideCardDTO{}, false
	}
	return guideCardDTO{Prompt: out.Prompt, Example: out.Example, RefHint: out.RefHint}, true
}

func (a *API) finishEssayWrite(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, state agent.StudioState) {
	dto, _ := a.buildEssayStatementStep(r.Context(), projectID, &state)
	if serr := a.saveTrackState(r.Context(), projectID, state); serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// GET /projects/{id}/essay-statement
func (a *API) getEssayStatement(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	state, err := a.loadEssayState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, changed := a.buildEssayStatementStep(r.Context(), projectID, &state)
	if changed {
		if serr := a.saveTrackState(r.Context(), projectID, state); serr != nil {
			slog.Warn("essay statement: persist failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// POST /projects/{id}/essay-statement/start — passed the outline-intro ready gate.
func (a *API) startEssayStatement(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	state, err := a.loadEssayState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	state.EssayTrack.Started = true
	state.EssayTrack.StatementStep = 0
	a.finishEssayWrite(w, r, projectID, state)
}

// POST /projects/{id}/essay-statement/advance {dir}
func (a *API) advanceEssayStatement(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Dir string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "请求格式不对", nil))
		return
	}
	state, err := a.loadEssayState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	switch body.Dir {
	case "next":
		state.EssayTrack.StatementStep++
	case "prev":
		state.EssayTrack.StatementStep--
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "dir 必须是 next 或 prev", nil))
		return
	}
	a.finishEssayWrite(w, r, projectID, state)
}

// POST /projects/{id}/essay-statement/review {stepKey} — 我写好了 → essay 批注.
func (a *API) reviewEssayStatement(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		StepKey string `json:"stepKey"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	state, err := a.loadEssayState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	steps := agent.DeriveStatementSteps(a.essaySubQuestions(state))
	cur := steps[agent.ClampStepIndex(state.EssayTrack.StatementStep, len(steps))]
	if body.StepKey != "" {
		for _, s := range steps {
			if s.Key == body.StepKey {
				cur = s
			}
		}
	}
	focus := cur.Title
	if cached := state.EssayTrack.StepGuides[cur.Key]; cached != "" {
		var c guideCardDTO
		if json.Unmarshal([]byte(cached), &c) == nil && c.Prompt != "" {
			focus = cur.Title + "：" + c.Prompt
		}
	}
	out := a.runDraftAnnotationReview(r.Context(), projectID, string(agent.DocEssay), focus)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"annotations": out})
}
