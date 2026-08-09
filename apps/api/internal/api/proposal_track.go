package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// proposal_track.go — slice 3a · the proposal-writing guide-step track REST.
// The track (free/guided + dynamic per-sub-question steps) lives on
// studio_state.proposalTrack (jsonb, no migration). Guide cards are generated on
// the fast model and cached per step key so a re-read never re-spends. Advancing
// / re-deriving / generating are deterministic system steps (no confirm gate).

type guideCardDTO struct {
	Prompt  string `json:"prompt"`
	Example string `json:"example"`
	RefHint string `json:"refHint,omitempty"`
}

type proposalGuideStepDTO struct {
	Key          string               `json:"key"`
	Title        string               `json:"title"`
	Kind         string               `json:"kind"`
	Index        int                  `json:"index"`
	Total        int                  `json:"total"`
	Mode         string               `json:"mode"`
	Started      bool                 `json:"started"`
	SubQuestions []agent.SubQuestion  `json:"subQuestions"`
	Card         *guideCardDTO        `json:"card"`
}

// loadTrackState loads the studio_state and returns it with a non-nil
// ProposalTrack (initialized if absent). The caller persists when it mutates.
func (a *API) loadTrackState(ctx context.Context, projectID uuid.UUID) (agent.StudioState, error) {
	state := agent.DefaultStudioState()
	if raw, err := a.d.Queries.GetStudioState(ctx, projectID); err == nil && len(raw) > 0 {
		if uerr := json.Unmarshal(raw, &state); uerr != nil {
			return state, uerr
		}
	}
	if state.ProposalTrack == nil {
		state.ProposalTrack = &agent.WritingTrack{}
	}
	if state.ProposalTrack.StepGuides == nil {
		state.ProposalTrack.StepGuides = map[string]string{}
	}
	return state, nil
}

func (a *API) saveTrackState(ctx context.Context, projectID uuid.UUID, state agent.StudioState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return a.d.Queries.SetStudioState(ctx, sqlc.SetStudioStateParams{ID: projectID, StudioState: b})
}

// resolveFast returns the fast chaperone resolver (falling back to the chaperone
// when the fast one is unset), plus ok=false when neither resolves.
func (a *API) resolveFast(ctx context.Context) (gateway.Resolved, bool) {
	if a.d.FastChatResolver != nil {
		if fr, err := a.d.FastChatResolver(ctx); err == nil {
			return fr, true
		}
	}
	if a.d.ChatResolver != nil {
		if r, err := a.d.ChatResolver(ctx); err == nil {
			return r, true
		}
	}
	return gateway.Resolved{}, false
}

// buildProposalGuideStep derives the current step and (guided+started only)
// generates/reads its cached guide card. Mutates state (clamps StepIndex, caches
// the guide) and reports whether it changed so the caller persists once.
func (a *API) buildProposalGuideStep(ctx context.Context, projectID uuid.UUID, state *agent.StudioState) (proposalGuideStepDTO, bool) {
	track := state.ProposalTrack
	steps := agent.DeriveProposalSteps(*track)
	total := len(steps)
	changed := false

	idx := agent.ClampStepIndex(track.StepIndex, total)
	if idx != track.StepIndex {
		track.StepIndex = idx
		changed = true
	}
	cur := steps[idx]

	subs := track.SubQuestions
	if subs == nil {
		subs = []agent.SubQuestion{}
	}
	dto := proposalGuideStepDTO{
		Key: cur.Key, Title: cur.Title, Kind: string(cur.Kind),
		Index: idx, Total: total, Mode: string(track.Mode), Started: track.Started,
		SubQuestions: subs,
	}

	if track.Mode == agent.ModeGuided && track.Started {
		if cached := track.StepGuides[cur.Key]; cached != "" {
			var c guideCardDTO
			if json.Unmarshal([]byte(cached), &c) == nil {
				dto.Card = &c
			}
		}
		if dto.Card == nil {
			if c, ok := a.generateGuideCard(ctx, projectID, *state, cur); ok {
				track.StepGuides[cur.Key] = string(mustJSON(c))
				dto.Card = &c
				changed = true
			}
		}
	}
	return dto, changed
}

// generateGuideCard runs the fast-model generator for one step. Best-effort:
// nil resolver / provider error → ok=false (the card stays nil, the read still
// succeeds). Metered purpose="proposal_guide".
func (a *API) generateGuideCard(ctx context.Context, projectID uuid.UUID, state agent.StudioState, step agent.Step) (guideCardDTO, bool) {
	if a.d.Provider == nil {
		return guideCardDTO{}, false
	}
	resolved, ok := a.resolveFast(ctx)
	if !ok {
		return guideCardDTO{}, false
	}
	in := agent.GuideGenInput{Step: step}
	if p, err := a.d.Queries.GetProject(ctx, projectID); err == nil {
		in.Title = p.Title
	}
	if prop, err := a.d.Queries.GetProjectProposal(ctx, projectID); err == nil {
		in.Objective, in.Reason, in.Activities, in.Resources = prop.Objective, prop.Reason, prop.Activities, prop.Resources
	}
	if step.Kind == agent.KindSubq && state.ProposalTrack != nil {
		in.SiblingSubQuestions = state.ProposalTrack.SubQuestions
		for _, sq := range state.ProposalTrack.SubQuestions {
			if sq.ID == step.SubQuestionID {
				in.ThisSubQuestion = sq.Text
			}
		}
	}
	out, usage, err := agent.GenerateProposalGuideStep(ctx, a.d.Provider, resolved, in)
	a.meterCall(ctx, projectID, resolved, "proposal_guide", usage)
	if err != nil {
		slog.Warn("proposal guide: generation failed", "err", err, "step", step.Key, "request_id", httpx.RequestIDFromContext(ctx))
		return guideCardDTO{}, false
	}
	return guideCardDTO{Prompt: out.Prompt, Example: out.Example, RefHint: out.RefHint}, true
}

// meterCall records an llm_call for a studio-surface model call (best-effort).
func (a *API) meterCall(ctx context.Context, projectID uuid.UUID, resolved gateway.Resolved, purpose string, usage gateway.ChatUsage) {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.RecordLLMCall(ctx, agent.LLMCallRow{
		ProjectID: projectID, Surface: "studio", Purpose: purpose,
		Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
	}); err != nil {
		slog.Warn("proposal track: record llm call failed", "err", err, "purpose", purpose, "request_id", httpx.RequestIDFromContext(ctx))
	}
}

// GET /projects/{id}/proposal-track
func (a *API) getProposalTrack(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	state, err := a.loadTrackState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	dto, changed := a.buildProposalGuideStep(r.Context(), projectID, &state)
	if changed {
		if serr := a.saveTrackState(r.Context(), projectID, state); serr != nil {
			slog.Warn("proposal track: persist failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// POST /projects/{id}/proposal-track/mode {mode}
func (a *API) setProposalMode(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "请求格式不对", nil))
		return
	}
	mode := agent.WriteMode(body.Mode)
	if mode != agent.ModeFree && mode != agent.ModeGuided {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "mode 必须是 free 或 guided", nil))
		return
	}
	state, err := a.loadTrackState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	state.ProposalTrack.Mode = mode
	if mode == agent.ModeGuided {
		state.ProposalTrack.Started = false // the outline intro comes first
	}
	a.finishTrackWrite(w, r, projectID, state)
}

// POST /projects/{id}/proposal-track/start
func (a *API) startProposalGuide(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	state, err := a.loadTrackState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	state.ProposalTrack.Started = true
	state.ProposalTrack.StepIndex = 0
	a.finishTrackWrite(w, r, projectID, state)
}

// POST /projects/{id}/proposal-track/subquestions {subQuestions:[{id?,text}]}
func (a *API) setProposalSubQuestions(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		SubQuestions []agent.SubQuestion `json:"subQuestions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "请求格式不对", nil))
		return
	}
	state, err := a.loadTrackState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// Normalize: keep only non-empty text; mint ids for new rows, keep existing.
	kept := make([]agent.SubQuestion, 0, len(body.SubQuestions))
	keepIDs := map[string]bool{}
	for _, sq := range body.SubQuestions {
		text := strings.TrimSpace(sq.Text)
		if text == "" {
			continue
		}
		id := strings.TrimSpace(sq.ID)
		if id == "" {
			id = uuid.NewString()
		}
		kept = append(kept, agent.SubQuestion{ID: id, Text: text})
		keepIDs[id] = true
	}
	// Drop cached guides for removed sub-questions.
	for k := range state.ProposalTrack.StepGuides {
		if strings.HasPrefix(k, "subq:") && !keepIDs[strings.TrimPrefix(k, "subq:")] {
			delete(state.ProposalTrack.StepGuides, k)
		}
	}
	state.ProposalTrack.SubQuestions = kept
	a.finishTrackWrite(w, r, projectID, state)
}

// POST /projects/{id}/proposal-track/advance {dir}
func (a *API) advanceProposalStep(w http.ResponseWriter, r *http.Request) {
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
	state, err := a.loadTrackState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	switch body.Dir {
	case "next":
		state.ProposalTrack.StepIndex++
	case "prev":
		state.ProposalTrack.StepIndex--
	default:
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "dir 必须是 next 或 prev", nil))
		return
	}
	a.finishTrackWrite(w, r, projectID, state)
}

// finishTrackWrite rebuilds the step (clamping + lazy card gen), persists, and
// returns the fresh step DTO — the one-call update the frontend applies.
func (a *API) finishTrackWrite(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, state agent.StudioState) {
	dto, _ := a.buildProposalGuideStep(r.Context(), projectID, &state)
	if serr := a.saveTrackState(r.Context(), projectID, state); serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dto)
}

// POST /projects/{id}/proposal-track/review {stepKey}
// The "我写好了" action (slice 3b): the flagship 批注 reviewer reads the proposal
// buffer (focused on the current step) and returns layered colored 批注, which
// the left panel renders view-only. (Slice 3a returned a chat-bubble
// ReviewVerdict; that is superseded here.)
func (a *API) reviewProposalPart(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		StepKey string `json:"stepKey"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body) // stepKey optional; default = current step

	state, err := a.loadTrackState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	steps := agent.DeriveProposalSteps(*state.ProposalTrack)
	cur := steps[agent.ClampStepIndex(state.ProposalTrack.StepIndex, len(steps))]
	if body.StepKey != "" {
		for _, s := range steps {
			if s.Key == body.StepKey {
				cur = s
			}
		}
	}
	// Focus the reviewer on this step (its title + cached guiding question).
	focus := cur.Title
	if state.ProposalTrack.StepGuides != nil {
		if cached := state.ProposalTrack.StepGuides[cur.Key]; cached != "" {
			var c guideCardDTO
			if json.Unmarshal([]byte(cached), &c) == nil && c.Prompt != "" {
				focus = cur.Title + "：" + c.Prompt
			}
		}
	}

	out := a.runDraftAnnotationReview(r.Context(), projectID, focus)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"annotations": out})
}
