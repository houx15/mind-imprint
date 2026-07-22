package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/skills"
)

// studioEmitter serializes all SSE writes (intervention/gate/done deltas +
// heartbeats) under one mutex, since the heartbeat goroutine and
// postProjectTurn both write the same stream. Mirrors turn.go's syncEmitter,
// but over the Studio event vocabulary (Intervention/Gate) rather than the
// legacy Text/Card.
type studioEmitter struct {
	mu  sync.Mutex
	sse *gateway.SSEWriter
}

func (e *studioEmitter) Intervention(id, body, anchor, criterion, level string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Intervention(id, body, anchor, criterion, level)
}

func (e *studioEmitter) Gate(contract, status string, passed, total int, missing []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Gate(contract, status, passed, total, missing)
}

func (e *studioEmitter) Done() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Done("")
}

// Text emits an assistant prose delta (Chat surface, Task 5) — the older
// Text/Card/Done vocabulary Chat's postChatTurn reuses via this same emitter
// rather than standing up a second emitter type.
func (e *studioEmitter) Text(delta string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Text(delta)
}

// Phase announces a course phase transition (Slice 12). Mutex-guarded like the
// emitter's other frames.
func (e *studioEmitter) Phase(to string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Phase(to)
}

// DoneCard is submitProjectCard's own Done — see gateway.SSEWriter.DoneCard.
func (e *studioEmitter) DoneCard(cardStatus string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.DoneCard(cardStatus)
}

func (e *studioEmitter) ErrorEnvelope(code, msg string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.ErrorEnvelope(code, msg)
}

func (e *studioEmitter) Heartbeat() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Heartbeat()
}

func (e *studioEmitter) Card(cardInstanceID, cardID, nudgeText string, anchors []byte, materialID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Card(cardInstanceID, cardID, nudgeText, anchors, materialID)
}

// Review streams the whole-draft review's work-order as one batch event
// (Task 6, orderReview).
func (e *studioEmitter) Review(items []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sse.Review(items)
}

// startHeartbeat starts the heartbeat goroutine shared by every Studio SSE
// endpoint (postProjectTurn, orderReview): a 15s-cadence ping until the
// caller closes stop or the request context is done, so a heartbeat write
// can never outlive the handler (the ResponseWriter is invalid once
// ServeHTTP returns). The caller must always `close(stop); <-hbDone` in a
// defer, in that order, before returning.
func startHeartbeat(ctx context.Context, em *studioEmitter) (stop chan struct{}, hbDone chan struct{}) {
	stop = make(chan struct{})
	hbDone = make(chan struct{})
	go func() {
		defer close(hbDone)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = em.Heartbeat()
			}
		}
	}()
	return stop, hbDone
}

// studioSimilarity is the Studio turn endpoint's enforcement.Similarity seam:
// a keyless, offline lexical-overlap heuristic (no embedding call, no key, no
// network). A real embedding-backed Similarity is a separate, later task once
// an embedding provider is chosen.
func studioSimilarity() enforcement.Similarity { return enforcement.LexicalSimilarity{} }

// postProjectTurn drives the agent runtime one step (RunAgentStep) for a
// Studio project and streams the result over SSE: at most one card,
// intervention, or gate event, then done.
func (a *API) postProjectTurn(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())

	// Entitlement gate — JSON error BEFORE committing to the stream.
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var body struct {
		UserInput string `json:"user_input"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if len(body.UserInput) == 0 {
		// Same constructor decodeJSON itself uses for a malformed body — 400
		// validation_failed, just with a field-specific message.
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "user_input 不能为空", nil))
		return
	}

	resolved, err := a.d.ChatResolver(r.Context())
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}

	// Commit to streaming. After this, errors are SSE error events, not JSON.
	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}

	// Heartbeat until the turn returns or the client disconnects.
	stop, hbDone := startHeartbeat(r.Context(), em)
	defer func() {
		close(stop)
		<-hbDone
	}()

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if err := store.CreateChatMessage(r.Context(), projectID, "user", body.UserInput); err != nil {
		slog.Error("studio turn: persist student message",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		_ = em.Done()
		return
	}
	promptPayload, perr := json.Marshal(map[string]string{"text": body.UserInput})
	if perr != nil {
		// Never fail the turn over telemetry — fall back to an empty payload.
		promptPayload = []byte(`{}`)
	}
	if err := store.AppendEvent(r.Context(), agent.EventRow{
		ProjectID: projectID, Surface: "studio", Type: "prompt_sent", Payload: promptPayload,
	}); err != nil {
		slog.Warn("studio turn: append prompt_sent event failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	sk, _ := skills.ByID("writing-project")
	deps := agent.AgentDeps{
		Store:            store,
		Provider:         a.d.Provider,
		Resolved:         resolved,
		Sim:              studioSimilarity(),
		Skill:            &sk,
		SkipSurfaceCards: false,
	}
	action, err := agent.RunAgentStep(r.Context(), deps, projectID, agent.Trigger{Kind: "student_turn", StudentText: body.UserInput})
	if err != nil {
		slog.Error("studio turn: RunAgentStep",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.ErrorEnvelope("internal_error", "对话处理失败，请重试")
		_ = em.Done()
		return
	}
	// A turn is activity — the roster's 最近活跃 depends on it. A failure to
	// touch must not fail the student's turn, which already succeeded.
	if err := a.d.Queries.TouchProject(r.Context(), projectID); err != nil {
		slog.Warn("studio turn: touch project last_active_at",
			"err", err, "project_id", projectID, "request_id", httpx.RequestIDFromContext(r.Context()))
	}

	a.streamAction(r.Context(), em, action, projectID, store)
	_ = em.Done()
}

// streamAction emits one RunAgentStep result over the Studio SSE emitter —
// shared by postProjectTurn and submitProjectCard (Task 5), the two
// endpoints that both drive RunAgentStep and stream whatever it returns.
// Does not call em.Done(); callers do that themselves once, after any
// endpoint-specific streaming this function doesn't cover.
func (a *API) streamAction(ctx context.Context, em *studioEmitter, action *agent.Action, projectID uuid.UUID, store agent.AgentStore) {
	switch {
	case action == nil:
		// silence: a legitimate first-class outcome (design §2) — nothing to
		// stream but done.
	case action.Kind == "surface_card":
		spec, _ := cards.ByID(action.CardID)
		anchors := []byte("[]")
		// surfaceAnchors is the guidance fade's entry point (spec §3): it
		// computes the level live (L1/L2/L3, annotate cards only) from how
		// many times this student has already completed this card, then has
		// AnchorGenerator author whatever that level still leaves for the AI
		// to do. Not a fixed L1 any more — see surfaceAnchors' own doc comment.
		// A compare card (SIFT) gets anchors too (whole-branch review finding
		// [3]) — authored on the CHECKED material only (the source under
		// review), never the lateral one, which does not exist yet at surface
		// time. surfaceAnchors itself restricts scope + drops the lateral
		// dimension; see its doc comment.
		if spec.Primitive == "annotate" || spec.Primitive == "compare" {
			if raw, ok := a.surfaceAnchors(ctx, store, projectID, spec, action.CardInstanceID, action.MaterialID); ok {
				anchors = raw
			}
		}
		_ = em.Card(action.CardInstanceID, action.CardID, spec.Name, anchors, action.MaterialID)
	case action.Kind == "intervention":
		// Anchor is sent EMPTY, deliberately: a live intervention's anchor is
		// a {kind,id} node reference, not the seed data's {label} shape — there
		// is no clean display label to derive from a node id (a UUID) without
		// resolving the node's text or storing a label at mint time, which is
		// deferred polish. The client shows the criterion chip only, which
		// matches what the reload projection (projectCoach) already produces
		// for these interventions (anchorLabel("{kind,id}") == "").
		_ = em.Intervention(action.InterventionID, action.Output.Body, "", action.Output.Criterion, "")
	case action.Kind == "check_gate" && action.GateReport != nil:
		gr := action.GateReport
		_ = em.Gate(gr.Contract, gr.Status, 0, 0, gr.Missing) // passed/total: deferred polish, see GateReport.Items
	}
}

// surfaceAnchors generates the guidance-faded anchors (spec §3, L1/L2/L3 for
// annotate cards; always L1 for compare) for a just-surfaced annotate or
// compare card (e.g. craap, sift), persists them on the card_instance, and
// returns the JSON to carry on the same SSE `card` frame. Degrades to
// (nil,false) on any error — parse failure, persistence failure, empty
// generation — so the card still surfaces with no anchors rather than
// failing the whole turn; generating pre-anchored questions is a nice-to-have
// on top of the card existing, not a precondition for it.
//
// checkedMaterialID is the card's own material (Action.MaterialID — the
// card_instance--evaluates-->material edge target, never anchors[0] or
// project-materials[0]). For a compare card it scopes generation to ONLY
// that material — a compare card's questions are about the source under
// review, not every material in the project — and, since spec.Params
// carries the lateral dimension (SIFT's "find"), the AI-authored anchors
// generated for it are dropped afterward: no lateral source has been chosen
// yet at surface time, so authoring a "quote" for that dimension off the
// checked material alone would misattribute it (whole-branch review finding
// [3]). An annotate card at L1 keeps today's behavior (all project materials
// in scope) unchanged — that first, broadest read is deliberate and must not
// silently narrow for every first-time student. At L2/L3 an annotate card is
// narrowed the same way compare is: the card is evaluating ONE source, so its
// question (L2) or dimension-only prompt (L3) must be scoped to that source,
// not whichever material happens to be project-materials[0] (the OLDEST one
// by created_at, per ListMaterialsByProject — see the narrowing below).
func (a *API) surfaceAnchors(ctx context.Context, store agent.AgentStore, projectID uuid.UUID, spec cards.Spec, cardInstanceID, checkedMaterialID string) ([]byte, bool) {
	cid, err := uuid.Parse(cardInstanceID)
	if err != nil {
		return nil, false
	}
	materials, err := a.projectMaterials(ctx, projectID)
	if err != nil {
		return nil, false
	}
	gen := agent.NewAnchorGenerator(a.d.Provider, a.d.ChatResolver)
	// The guidance fade (spec §3): the scaffold recedes as she repeats a card.
	// annotate only — compare/SIFT stays L1 (its lateral read is already her
	// own work, and its generation is additionally constrained below).
	level := agent.GuidanceL1
	if spec.Primitive == "annotate" {
		if u, ok := UserFromContext(ctx); ok {
			uses, err := store.CountCompletedCardUsesByUser(ctx, u.ID, spec.ID)
			if err != nil {
				// Degrade to L1 rather than failing the surface: a card that
				// asks too much is a wall, a card that asks too little is
				// merely a slower fade.
				slog.Warn("surface anchors: guidance count failed", "err", err, "request_id", httpx.RequestIDFromContext(ctx))
			} else {
				level = agent.GuidanceFor(uses)
			}
		}
	}
	if spec.Primitive == "compare" && checkedMaterialID != "" {
		materials = onlyMaterial(materials, checkedMaterialID)
	}
	// L2/L3 annotate: narrow to the card's own material (mirrors compare's
	// guard above). Deliberately does NOT run at L1 (see doc comment). A miss
	// (checkedMaterialID == "", e.g. a project-scoped card with no material)
	// leaves materials unfiltered — no worse than the pre-fix behavior.
	if spec.Primitive == "annotate" && level != agent.GuidanceL1 && checkedMaterialID != "" {
		materials = onlyMaterial(materials, checkedMaterialID)
	}
	result, err := gen.Generate(ctx, spec, materials, level)
	if err != nil || len(result.Anchors) == 0 {
		return nil, false
	}
	if spec.Params.LateralDimension != "" {
		result.Anchors = dropDimension(result.Anchors, spec.Params.LateralDimension)
	}
	// A real call succeeded whenever Resolved is populated (GenerateResult's
	// contract) — it cost money regardless of whether persisting the
	// generated anchors below succeeds, so record it unconditionally here,
	// before any further step can bail out. A metering failure must never
	// fail the turn (same policy as TouchProject).
	if result.Resolved.Provider != "" {
		if rerr := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "anchors",
			Resolved: result.Resolved, PromptTokens: int32(result.Usage.InputTokens), CompletionTokens: int32(result.Usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("surface anchors: record llm usage failed", "err", rerr, "request_id", httpx.RequestIDFromContext(ctx))
		}
	}
	raw, err := json.Marshal(result.Anchors)
	if err != nil {
		return nil, false
	}
	if err := store.SetCardInstanceAnchors(ctx, projectID, cid, raw); err != nil {
		slog.Warn("surface anchors: persist failed", "err", err, "request_id", httpx.RequestIDFromContext(ctx))
		return nil, false
	}
	return raw, true
}

// onlyMaterial narrows a materials list to the single entry matching id, when
// present — used to scope anchor generation to the checked material only:
// for a compare card at every level, and for an annotate card at L2/L3 (where
// the anchors carry materials[0] rather than a per-block resolution, so an
// unnarrowed list would pin them to the project's OLDEST material — see the
// annotate branch above). A miss degrades to the full (unfiltered) list rather than
// an empty one, the same "never fail the turn over anchor generation"
// posture the rest of this file follows.
func onlyMaterial(materials []agent.Material, id string) []agent.Material {
	for _, m := range materials {
		if m.ID == id {
			return []agent.Material{m}
		}
	}
	return materials
}

// dropDimension removes every anchor authored for dim — used to strip a
// compare card's lateral-dimension anchor (SIFT's "find") from the AI-
// authored surface set: that dimension is about a source the student has not
// chosen yet, so there is nothing honest to anchor it to.
func dropDimension(anchors []agent.Anchor, dim string) []agent.Anchor {
	out := make([]agent.Anchor, 0, len(anchors))
	for _, a := range anchors {
		if a.Dimension == dim {
			continue
		}
		out = append(out, a)
	}
	return out
}

// projectMaterials reads the project's materials into the agent runtime's
// view — ID (needed by the deterministic fallback) and Blocks (needed by
// the real LLM path for quote->offset resolution). Mirrors
// sqlcTurnStore.ListMaterials (turn.go), the legacy task-scoped equivalent.
func (a *API) projectMaterials(ctx context.Context, projectID uuid.UUID) ([]agent.Material, error) {
	rows, err := a.d.Queries.ListMaterialsByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]agent.Material, 0, len(rows))
	for _, r := range rows {
		m := agent.Material{ID: r.ID.String(), Title: r.Title}
		if len(r.Blocks) > 0 {
			_ = json.Unmarshal(r.Blocks, &m.Blocks)
		}
		out = append(out, m)
	}
	return out, nil
}
