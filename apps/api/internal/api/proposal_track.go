package api

import (
	"context"
	"encoding/json"
	"errors"
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

// stepRefDTO is one entry in the ordered step list (for client-side per-part
// assembly of the guided writing — slice 4b retrofit). Card is the step's cached
// guide card, when one has been generated (i.e. the step was visited): it lets
// the finished-cards fold re-show each finished part's original guidance +
// example, so a re-read/re-edit reads like the writing surface, not a bare slab.
type stepRefDTO struct {
	Key   string        `json:"key"`
	Title string        `json:"title"`
	Kind  string        `json:"kind"`
	Card  *guideCardDTO `json:"card,omitempty"`
}

type proposalGuideStepDTO struct {
	Key          string              `json:"key"`
	Title        string              `json:"title"`
	Kind         string              `json:"kind"`
	Index        int                 `json:"index"`
	Total        int                 `json:"total"`
	Mode         string              `json:"mode"`
	Started      bool                `json:"started"`
	SubQuestions []agent.SubQuestion `json:"subQuestions"`
	Card         *guideCardDTO       `json:"card"`
	// Steps (slice 4b) — the full ordered step list, so the guided surface can
	// assemble the per-part text back into the document in order.
	Steps []stepRefDTO `json:"steps"`
}

// toStepRefs projects the ordered step list to the wire, attaching each step's
// cached guide card (from StepGuides) when present so finished parts can re-show
// their guidance. A nil/absent cache entry leaves Card nil (omitted on the wire).
func toStepRefs(steps []agent.Step, guides map[string]string) []stepRefDTO {
	out := make([]stepRefDTO, 0, len(steps))
	for _, s := range steps {
		ref := stepRefDTO{Key: s.Key, Title: s.Title, Kind: string(s.Kind)}
		if cached := guides[s.Key]; cached != "" {
			var c guideCardDTO
			if json.Unmarshal([]byte(cached), &c) == nil {
				ref.Card = &c
			}
		}
		out = append(out, ref)
	}
	return out
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

// routeE resolves one capability class, returning the gateway's own error. It
// is the seam every call site should use: naming a class says how much
// intelligence this call needs, and the catalog decides which model that means
// today. See docs/superpowers/specs/2026-09-02-llm-routing-taxonomy-design.md.
func (a *API) routeE(ctx context.Context, class string) (gateway.Resolved, error) {
	if a.d.Route != nil {
		return a.d.Route(class)(ctx)
	}
	// Pre-class wiring: a Deps built by hand (every handler test does this) that
	// sets only the old lane resolvers. Falling back to the lane this class
	// aliases keeps those tests meaningful instead of turning them all into
	// "no provider configured".
	if legacy := a.legacyResolver(class); legacy != nil {
		return legacy(ctx)
	}
	return gateway.Resolved{}, errors.New("no LLM provider configured")
}

// legacyResolver maps a class onto the pre-class lane resolver that used to
// serve it, for a Deps wired before Route existed — which is every handler test
// in this package.
//
// It falls through to ANY resolver that is set, not just the preferred one.
// Those tests wire exactly one fake resolver, chosen to match the lane the site
// used before classes; insisting on the preferred lane would hand them "no
// provider configured" and turn a suite of behaviour tests into a suite of
// degraded-path tests that still pass. Production always sets Route and never
// reaches here.
func (a *API) legacyResolver(class string) gateway.KeyResolver {
	var order []gateway.KeyResolver
	switch class {
	case gateway.ClassAssess, gateway.ClassReview:
		order = []gateway.KeyResolver{a.d.EvalResolver, a.d.ChatResolver, a.d.FastChatResolver}
	case gateway.ClassReflex:
		order = []gateway.KeyResolver{a.d.FastChatResolver, a.d.ChatResolver, a.d.EvalResolver}
	default:
		order = []gateway.KeyResolver{a.d.ChatResolver, a.d.EvalResolver, a.d.FastChatResolver}
	}
	for _, r := range order {
		if r != nil {
			return r
		}
	}
	return nil
}

// routeFn hands a class's resolver to an agent helper that resolves it itself
// (agent.RouteReading, agent.ProposeCardExample, agent.ComposeJourney …).
func (a *API) routeFn(class string) gateway.KeyResolver {
	return func(ctx context.Context) (gateway.Resolved, error) { return a.routeE(ctx, class) }
}

// route is the tolerant form, for call sites where a missing provider degrades
// the feature instead of failing the request. ok=false when nothing resolves.
func (a *API) route(ctx context.Context, class string) (gateway.Resolved, bool) {
	if r, err := a.routeE(ctx, class); err == nil {
		return r, true
	}
	// 🚨 assess never falls back. 过程评估走旗舰模型绝不降级 — the catalog refuses
	// a non-flagship binding at boot, and it would be absurd for this helper to
	// hand back at runtime what the catalog refused at startup. The pre-class
	// resolveEval DID fall through to the chaperone here; that was a hole, and
	// it was invisible because in practice every class shares one key, so the
	// path only opens in the one situation where it does real damage — a
	// half-configured environment quietly grading a student's process on the
	// cheap model.
	if class == gateway.ClassAssess {
		return gateway.Resolved{}, false
	}
	// Every other class retries on dialogue: it is the class most likely to be
	// configured in a partially-set-up environment, so a dev box with a single
	// key still answers.
	if class != gateway.ClassDialogue {
		if r, err := a.routeE(ctx, gateway.ClassDialogue); err == nil {
			return r, true
		}
	}
	return gateway.Resolved{}, false
}

// resolveFast returns the fast chaperone resolver (falling back to the chaperone
// when the fast one is unset), plus ok=false when neither resolves.
//
// Deprecated: name a capability class with route(ctx, gateway.Class…) instead.
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

// resolveEval returns the flagship reviewer resolver (the never-downgrade seam
// the doc reserves for reviewers + plan-gen), falling back to the chaperone when
// EvalResolver is unset. ok=false when neither resolves.
func (a *API) resolveEval(ctx context.Context) (gateway.Resolved, bool) {
	if a.d.EvalResolver != nil {
		if er, err := a.d.EvalResolver(ctx); err == nil {
			return er, true
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
		SubQuestions: subs, Steps: toStepRefs(steps, track.StepGuides),
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
	resolved, ok := a.route(ctx, gateway.ClassCompose)
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
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
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
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
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
		To  *int   `json:"to"` // §4 gap G10 · absolute jump to a step (parts overview)
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	state, err := a.loadTrackState(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if body.To != nil {
		state.ProposalTrack.StepIndex = *body.To // clamped in finishTrackWrite
	} else {
		switch body.Dir {
		case "next":
			state.ProposalTrack.StepIndex++
		case "prev":
			state.ProposalTrack.StepIndex--
		default:
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "dir 必须是 next 或 prev", nil))
			return
		}
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

	out := a.runDraftAnnotationReview(r.Context(), projectID, string(agent.DocProposal), focus)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"annotations": out})
}
