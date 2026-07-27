package api

// readturn.go — Task 5: POST /projects/{id}/materials/{mid}/read-turn, the
// read-together router turn (docs/2026-07-26-spec-read-together-redesign —
// Tasks 1-4 landed the pure cores this handler wires up). Scoped to ONE
// material: given the student's current expression and whatever she has
// focused on while reading THAT source, ask the flagship router
// (RouteReading) whether to summon a reading-room card, hint, or just keep
// talking, then apply the deterministic pacing/ordering restraint
// (ApplyReadingGate) before ever emitting a summon. Mirrors postProjectTurn's
// SSE scaffold (studioturn.go) — heartbeat, studioEmitter, done-at-the-end —
// but drives RouteReading/ApplyReadingGate instead of RunAgentStep: this loop
// never mints a post_intervention or runs the coach, only respond/hint/summon.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
)

// readTurnFocusSpan is the wire shape of one span the student currently has
// focused while reading — a block she highlighted or is looking at.
type readTurnFocusSpan struct {
	BlockID string `json:"block_id"`
	Quote   string `json:"quote"`
}

// readTurnReq is postReadingTurn's request body.
type readTurnReq struct {
	StudentText  string              `json:"student_text"`
	FocusedSpans []readTurnFocusSpan `json:"focused_spans"`
}

// readingTurnBreathingRoomTurns approximates PacingState.TurnsSinceLastPropose:
// no per-turn counter is persisted anywhere yet (that would need a new column
// or an event-table scan), so this handler reports a large constant — "well
// past the breathing-room window" — rather than 0 ("just proposed a card").
// This is a documented approximation, not an oversight: the common case (never
// re-propose while a card is already open) is covered by Pacing.OpenCard,
// which IS derived live from the graph below.
const readingTurnBreathingRoomTurns = 99

// postReadingTurn drives one turn of the read-together router. It always
// consults the model once (RouteReading) — the deterministic gate
// (ApplyReadingGate) can only DOWNGRADE that proposal afterward, never veto it
// before asking — then, for a summon, resolves a verbatim example anchor and
// retries the router exactly once if that fails, degrading to a plain
// "respond" (no card) if it still can't produce a real anchor. A card frame
// with a (0,0)/empty anchor is never emitted — that is the exact "lights up
// nothing" bug ResolveExampleAnchor's contract forbids.
func (a *API) postReadingTurn(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	mid, err := uuid.Parse(r.PathValue("mid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
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

	var body readTurnReq
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	g, err := store.LoadGraph(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	// Ownership: mid must be one of THIS project's materials — mirrors
	// prepareSourceAnnotation's parse-based compare (materials.go): a
	// MaterialView.ID is a pgtype.UUID rendering that may differ in
	// dashing/case from google/uuid's canonical String().
	foundMaterial := false
	for _, m := range g.Materials {
		if pid, perr := uuid.Parse(m.ID); perr == nil && pid == mid {
			foundMaterial = true
			break
		}
	}
	if !foundMaterial {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	materials, err := a.projectMaterials(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	var blocks []agent.MaterialBlock
	for _, m := range materials {
		if m.ID == mid.String() {
			blocks = m.Blocks
			break
		}
	}

	// Commit to streaming. After this, errors are SSE frames, not JSON.
	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}
	stop, hbDone := startHeartbeat(r.Context(), em)
	defer func() {
		close(stop)
		<-hbDone
	}()

	catalog, err := agent.ReadingDeck()
	if err != nil {
		// The deck failed to resolve against the card registry (a
		// programming error, not a runtime one) — degrade to a silent,
		// best-effort done rather than 500ing a stream already committed.
		slog.Error("read turn: reading deck resolve failed",
			"err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		_ = em.Done()
		return
	}

	scaffold := make(map[string]int, len(catalog))
	for _, c := range catalog {
		n, cerr := store.CountCompletedCardUsesByUser(r.Context(), u.ID, c.CardID)
		if cerr != nil {
			slog.Warn("read turn: guidance count failed",
				"err", cerr, "card_id", c.CardID, "request_id", httpx.RequestIDFromContext(r.Context()))
			continue
		}
		scaffold[c.CardID] = n
	}

	// OpenCard is the one-active mutex, PROJECT-WIDE (mirrors
	// SurfaceCardCandidates' own in-flight guard): while any card is
	// proposed/active, nothing new fires anywhere in the project.
	openCard := false
	for _, ci := range g.CardInstances {
		if ci.Status == "proposed" || ci.Status == "active" {
			openCard = true
			break
		}
	}
	// CompletedCards/RecentlySkipped/ordering below are all scoped to THIS
	// material, derived directly from card_instance STATUS + the anchors
	// persisted on each instance — every anchor carries its owning MaterialID
	// (the example anchor this endpoint persists via SetCardInstanceAnchors
	// below, and the anchors surfaceAnchors/AnchorGenerator persist for a card
	// the main turn loop summons). This is deliberately NOT the
	// card_instance--evaluates-->material edge SurfaceCardCandidates
	// (agent/classifier.go) reads: that edge is minted ONLY by
	// agent.SurfaceCard's graph effects, never by store.CreateCardInstance —
	// which is what THIS handler calls to summon a reading-room card. Keying
	// off the edge left every card this endpoint itself summons invisible to
	// its own skip-cooldown/new-span-reuse/ordering checks; the anchor-based
	// scan below covers both creation paths uniformly.
	matID := mid.String()
	onMaterial := func(ci agent.CardInstanceView) bool {
		for _, a := range ci.Anchors {
			if a.MaterialID == matID {
				return true
			}
		}
		return false
	}
	var completedOnMaterial, skippedOnMaterial []string
	var craapCompleted, craapInFlight, siftCompleted, siftInFlight bool
	for _, ci := range g.CardInstances {
		if len(ci.Anchors) == 0 || !onMaterial(ci) {
			continue
		}
		switch ci.Status {
		case "completed":
			completedOnMaterial = append(completedOnMaterial, ci.CardID)
			switch ci.CardID {
			case "craap":
				craapCompleted = true
			case "sift":
				siftCompleted = true
			}
		case "skipped":
			// No per-turn timestamp is persisted anywhere (same limitation as
			// readingTurnBreathingRoomTurns above), so ANY skipped instance on
			// this material is treated as in-cooldown indefinitely — stricter
			// than PacingState's real skipCooldownTurns (3-turn) window, but
			// honest and safe: it can only ever suppress a re-summon it
			// shouldn't, never wrongly allow one back through.
			skippedOnMaterial = append(skippedOnMaterial, ci.CardID)
		case "proposed", "active":
			switch ci.CardID {
			case "craap":
				craapInFlight = true
			case "sift":
				siftInFlight = true
			}
		}
	}

	focusedSpans := make([]agent.FocusSpan, 0, len(body.FocusedSpans))
	for _, s := range body.FocusedSpans {
		focusedSpans = append(focusedSpans, agent.FocusSpan{BlockID: s.BlockID, Quote: s.Quote})
	}

	in := agent.ReadingRouteInput{
		StudentText:    body.StudentText,
		FocusedSpans:   focusedSpans,
		Catalog:        catalog,
		ScaffoldLevels: scaffold,
		Pacing: agent.PacingState{
			OpenCard:              openCard,
			TurnsSinceLastPropose: readingTurnBreathingRoomTurns,
			RecentlySkipped:       skippedOnMaterial,
			CompletedCards:        completedOnMaterial,
			HasNewFocus:           len(body.FocusedSpans) > 0,
		},
	}

	// Source-check ordering guard: don't summon SIFT before CRAAP, don't
	// re-summon CRAAP on an already-evaluated (or in-flight) material — the
	// SAME rule the main turn loop's classifier applies (SurfaceCardCandidates),
	// restricted to THIS material, but derived from card_instance status above
	// rather than SurfaceCardCandidates' own evaluates-edge reading — see the
	// derivation comment above for why that edge is the wrong signal here.
	ordering := agent.OrderingGuard{
		AllowCraap: !craapCompleted && !craapInFlight,
		AllowSift:  craapCompleted && !siftCompleted && !siftInFlight,
	}

	routeOnce := func() agent.ReadingDecision {
		d, resolved, usage, _ := agent.RouteReading(r.Context(), a.d.Provider, a.d.EvalResolver, in)
		a.recordReadingLLMCall(r.Context(), store, projectID, "read_router", resolved, usage)
		return d
	}

	decision := agent.ApplyReadingGate(routeOnce(), in.Pacing, ordering)

	var exampleAnchor agent.Anchor
	if decision.Decision == "summon" {
		var resolvedAnchor bool
		exampleAnchor, resolvedAnchor = agent.ResolveExampleAnchor(decision, mid.String(), blocks)
		if !resolvedAnchor {
			// Retry the router exactly once, then degrade — never build a
			// (0,0)/empty anchor. Only force "respond" when the RETRIED
			// decision is itself still a summon whose example anchor failed to
			// resolve: if the retry instead comes back "hint" or "respond",
			// that is already a legitimate final decision — clobbering it
			// unconditionally (checking the stale resolvedAnchor from the
			// FIRST attempt) would discard a real hint/respond the retried
			// router earned.
			decision = agent.ApplyReadingGate(routeOnce(), in.Pacing, ordering)
			if decision.Decision == "summon" {
				exampleAnchor, resolvedAnchor = agent.ResolveExampleAnchor(decision, mid.String(), blocks)
				if !resolvedAnchor {
					decision = agent.ReadingDecision{Decision: "respond"}
				}
			}
		}
	}

	switch decision.Decision {
	case "summon":
		spec, specOK := cards.ByID(decision.CardID)
		if !specOK {
			// The router named a card id outside the registry — should never
			// happen (RouteReading validates against in.Catalog's ids), but
			// degrade rather than emit a card frame with no spec behind it.
			break
		}
		row, cerr := store.CreateCardInstance(r.Context(), projectID, mid, decision.CardID, "")
		if cerr != nil {
			slog.Error("read turn: create card instance failed",
				"err", cerr, "request_id", httpx.RequestIDFromContext(r.Context()))
			break
		}
		anchorsJSON, merr := json.Marshal([]agent.Anchor{exampleAnchor})
		if merr != nil {
			anchorsJSON = []byte("[]")
		}
		if serr := store.SetCardInstanceAnchors(r.Context(), projectID, row.ID, anchorsJSON); serr != nil {
			slog.Warn("read turn: persist anchors failed",
				"err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
		_ = em.Card(row.ID.String(), decision.CardID, spec.Name, anchorsJSON, mid.String())
	case "hint":
		_ = em.Intervention("", decision.Reason, "", decision.CardID, "hint")
	default: // "respond"
		if decision.Reason != "" {
			_ = em.Text(decision.Reason)
		}
	}
	_ = em.Done()
}

// recordReadingLLMCall meters one real read-together LLM call (router or
// evaluate). A real call succeeded whenever resolved.Provider is populated —
// it cost money regardless of what parsing does next — so this must be called
// BEFORE any bail on a malformed/empty reply. A metering failure only warns;
// it never fails the turn.
func (a *API) recordReadingLLMCall(ctx context.Context, store agent.AgentStore, projectID uuid.UUID, purpose string, resolved gateway.Resolved, usage gateway.ChatUsage) {
	if resolved.Provider == "" {
		return
	}
	if err := store.RecordLLMCall(ctx, agent.LLMCallRow{
		ProjectID: projectID, Surface: "studio", Purpose: purpose,
		Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
	}); err != nil {
		slog.Warn("read turn: record llm usage failed",
			"err", err, "purpose", purpose, "request_id", httpx.RequestIDFromContext(ctx))
	}
}
