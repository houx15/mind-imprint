package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// InterventionRow is the persistence payload for one coach intervention,
// mapped to the `intervention` table (Task 1's sqlc queries). CardInstanceID
// is nil in Slice 2 — no card runtime yet (design §0 out-of-scope).
type InterventionRow struct {
	ProjectID          uuid.UUID
	CardInstanceID     *uuid.UUID
	Type               string
	Anchor             []byte // jsonb: enforcement.OutputAnchor, marshaled by the loop
	Criterion          string
	Body               string
	Level              string
	OutputCheckVerdict string
}

// ChatTurn is one turn of the coach's conversational context: the student's
// message (role "user") or a prior coach intervention (role "assistant").
type ChatTurn struct {
	Role    string // "user" | "assistant"
	Content string
}

// EventRow is the persistence payload for one appended C4 event.
type EventRow struct {
	ProjectID uuid.UUID
	Surface   string
	Type      string
	Payload   []byte
}

// LLMCallRow is the persistence payload for one live LLM call's usage
// (design's "记录档位 + token + 成本" hard constraint — migration 0019's
// llm_call table, unioned into the llm_usage view the admin console reads).
// Surface/Purpose classify which call site produced it ("studio"/"coach" for
// RunAgentStep's own coach turn, "studio"/"anchors" for a just-surfaced
// annotate card's anchor generation); Resolved carries the already-resolved
// provider/model/tier, and PromptTokens/CompletionTokens are the raw usage
// counts the cost formula (gateway.EstimateCost) is computed from.
type LLMCallRow struct {
	ProjectID        uuid.UUID
	Surface          string
	Purpose          string
	Resolved         gateway.Resolved
	PromptTokens     int32
	CompletionTokens int32
}

// CardInstanceRow is the persistence view of one project-scoped
// card_instance row (Task 5) — just what the card lifecycle (card_lifecycle.go)
// needs: which project it belongs to, its card id, its live anchors (to
// evaluate completion), and its framework_fill (consolidation).
type CardInstanceRow struct {
	ID            uuid.UUID
	ProjectID     uuid.UUID
	CardID        string
	Status        string
	Anchors       []byte
	FrameworkFill []byte

	// FieldValues is the card's submitted field_values jsonb — the refeed
	// serializer's other half beside Anchors (N3b Seam B).
	FieldValues []byte
}

// SourceLogRow is the persistence view of one project-scoped
// source_log_entry row that CompleteCard needs: the pyramid tier the
// student assigned at ingestion time (Slice 6b), read BEFORE a cross_check's
// LateralRead write overwrites it with her post-check re-tier.
type SourceLogRow struct {
	Tier string
}

// AgentStore is the runtime loop's persistence seam: perceive (LoadGraph)
// and record (InsertIntervention, AppendEvent, and the Task 5 card/graph-mint/
// disposition ops). The sqlc-backed adapter lives in agentstore.go;
// loop_test.go/card_lifecycle_test.go use an in-memory fake.
type AgentStore interface {
	LoadGraph(ctx context.Context, projectID uuid.UUID) (GraphView, error)
	InsertIntervention(ctx context.Context, row InterventionRow) (uuid.UUID, error)
	AppendEvent(ctx context.Context, row EventRow) error

	// CreateChatMessage/LoadChatHistory are the chat-history seam (Slice 5c
	// task 2): persisting the student's spoken/typed turns and reading back
	// the merged student+coach conversation so the coach can see it.
	CreateChatMessage(ctx context.Context, projectID uuid.UUID, role, content string) error
	LoadChatHistory(ctx context.Context, projectID uuid.UUID, limit int) ([]ChatTurn, error)

	// CreateCardInstance instantiates a proposed card_instance for cardID
	// on materialID (SurfaceCard, card_lifecycle.go). The legacy
	// card_instances.task_id NOT NULL FK is resolved from materialID's own
	// task_id by the adapter — pure agent code never has to know about
	// tasks.
	CreateCardInstance(ctx context.Context, projectID, materialID uuid.UUID, cardID, contractRef string) (CardInstanceRow, error)
	GetCardInstance(ctx context.Context, id uuid.UUID) (CardInstanceRow, error)
	SetCardInstanceFramework(ctx context.Context, projectID, id uuid.UUID, framework []byte) error

	// GetSourceLogByMaterial reads one source's log row — CompleteCard's
	// only use is the ingestion-time tier (tier_before) a cross_check
	// consolidates into its node body before its own LateralRead write
	// overwrites that same tier with the student's post-check re-tier
	// (Task 7). A missing log entry is not fatal to card completion:
	// tier_before is simply absent from the node body.
	GetSourceLogByMaterial(ctx context.Context, materialID uuid.UUID) (SourceLogRow, error)

	// CountCompletedCardUsesByUser is the guidance fade's producer (Task 3,
	// design §3): how many times this student has already COMPLETED cardID,
	// across all her projects/courses/chats. surfaceAnchors (Task 4) maps
	// this onto GuidanceLevel (guidance.go) to decide how much of an
	// annotate card's span-locating work the AI still does for her.
	CountCompletedCardUsesByUser(ctx context.Context, userID uuid.UUID, cardID string) (int, error)

	// SetCardInstanceStatus/SetCardInstanceAnchors/SubmitProjectCardInstance
	// are the Slice 5c-2 card-runtime mutation seam: opening a card
	// (proposed->active), each live field/observe-event write (anchors),
	// and the student's final submission (field_values + event_trace).
	SetCardInstanceStatus(ctx context.Context, projectID, id uuid.UUID, status string) error
	SetCardInstanceAnchors(ctx context.Context, projectID, id uuid.UUID, anchors []byte) error
	SubmitProjectCardInstance(ctx context.Context, projectID, id uuid.UUID, fieldValues, eventTrace []byte) error

	// InsertGraphNode/InsertGraphEdge apply one card's graph_effects
	// (CompleteCard, card_lifecycle.go) and mint SurfaceCard's
	// card_instance->material edge. node/edge ids must already be resolved
	// to real uuids (no "$new:" placeholders) by the caller.
	InsertGraphNode(ctx context.Context, projectID uuid.UUID, node MintNode) (uuid.UUID, error)
	InsertGraphEdge(ctx context.Context, projectID uuid.UUID, edge MintEdge) error

	// CommitCardMint writes everything a completed card produces — the
	// minted nodes, their edges, and the consolidation framework/idempotency
	// guard (agentstore.go's CardMint) — in ONE transaction (Task 6: closes
	// the atomicity gap the three calls above left when run separately).
	// CompleteCard uses this instead of InsertGraphNode/InsertGraphEdge/
	// SetCardInstanceFramework directly; those three stay on the interface
	// because SurfaceCard still uses InsertGraphEdge on its own, non-mint
	// path (the card_instance->material edge).
	CommitCardMint(ctx context.Context, projectID, cardInstanceID uuid.UUID, m CardMint) error

	InsertDisposition(ctx context.Context, interventionID uuid.UUID, action, reason string) (uuid.UUID, error)

	// InsertReviewIntervention persists one whole-draft-review work-order
	// item (Task 6, orderReview) as a review_item intervention row.
	InsertReviewIntervention(ctx context.Context, row ReviewInterventionRow) error

	// RecordLLMCall persists one live LLM call's usage (5d review CRITICAL
	// fix — every DeepSeek call must be metered, AGENTS.md's "记录档位 +
	// token + 成本" hard constraint). A failure here must never fail the
	// student's turn/submit — callers slog.Warn and continue, same policy as
	// TouchProject (studioturn.go/projectcards.go).
	RecordLLMCall(ctx context.Context, row LLMCallRow) error

	// Gate/plan graph-node state (Slice 4). gate_state is one graph_node per
	// (project, contract) keyed on body->>'contract'; plan is one per project.
	ListGateStates(ctx context.Context, projectID uuid.UUID) (map[string]RecordedGate, error)
	UpsertGateState(ctx context.Context, projectID uuid.UUID, contract string, rec RecordedGate) error
	UpsertPlan(ctx context.Context, projectID uuid.UUID, body []byte) error
}

// AgentDeps bundles the runtime loop's dependencies (design §2): the
// persistence seam, the model provider + its resolved routing (the coach
// runs flagship), and the injected embedding-similarity seam OutputCheck
// needs.
type AgentDeps struct {
	Store    AgentStore
	Provider gateway.Provider
	Resolved gateway.Resolved
	Sim      enforcement.Similarity

	// Skill is the optional Slice-4 planner seam: when set, RunAgentStep
	// reconciles gates + computes the route and considers a check_gate
	// candidate (Task 10). Slice-2/3 callers leave it nil, and the
	// check_gate path is skipped entirely — back-compat for every existing
	// test that constructs AgentDeps without a Skill.
	Skill *skills.Skill

	// SkipSurfaceCards, when true, stops RunAgentStep from producing
	// surface_card candidates at all (Slice 5c's conversational loop, which
	// defers card-surfacing to its own dedicated pass — 5c-2). Zero value
	// (false) is back-compat for every existing caller/test: candidates are
	// seeded from SurfaceCardCandidates(g) exactly as before.
	SkipSurfaceCards bool
}

// RunAgentStep runs one perceive -> classify -> decide-one -> act -> enforce
// -> record pass for a project (design §2). Silence is a first-class
// outcome — both "no candidate move" and "the coach's output was rejected
// by enforcement" return (nil, nil); a rejected output is never persisted.
// trigger identifies which tier (T-A/T-B/T-C) invoked this step; Slice 2's
// single trigger predicate does not branch on it yet.
//
// Candidate ordering (Task 5, extended Task 10): surface_card candidates
// come first, then check_gate (when deps.Skill is set), ahead of every
// post_intervention candidate (whether from the unsupported-claim predicate
// or an active card's observe rules) — surfacing an unevaluated source is a
// one-time offer the student can act on immediately, a gate check is a
// no-model structural read, while a post_intervention nudge can always wait
// one more step. Within each tier, candidates stay in the classifier's
// stable (material/node) order.
func RunAgentStep(ctx context.Context, deps AgentDeps, projectID uuid.UUID, trigger Trigger) (*Action, error) {
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// N3b Seam B — 摘要回灌. A card the student just COMPLETED gets exactly one
	// coach question about what she wrote in it. This is the acceptance
	// mainline's 摘要回灌, and it outranks every other candidate: it is a direct
	// response to something she just did.
	//
	// A SKIPPED card gets silence. The skip is recorded as data (铁律 4 ·
	// 过程即数据); answering a decline with a question is the nagging posture
	// 铁律 2 forbids.
	var refeedCand *Candidate
	var refeedPayload *RefeedPayload
	if trigger.Kind == "card_refeed" && trigger.CardInstanceID != "" {
		if cand, payload, ok := refeedCandidate(ctx, deps, trigger.CardInstanceID); ok {
			refeedCand, refeedPayload = &cand, &payload
		}
	}

	var cands []Candidate
	if refeedCand != nil {
		cands = append(cands, *refeedCand)
	}
	if !deps.SkipSurfaceCards {
		cands = append(cands, SurfaceCardCandidates(g)...)
	}

	// When a Project skill is loaded, reconcile its gates once and hold the
	// reports: they feed the (lowest-priority) check_gate fallback below and are
	// reused by the check_gate handler (no second fetch/recompute).
	var gateReports map[string]GateReport
	var checkGateCands []Candidate
	if deps.Skill != nil {
		recorded, err := deps.Store.ListGateStates(ctx, projectID)
		if err != nil {
			return nil, err
		}
		gateReports = ReconcileGates(*deps.Skill, g, recorded)
		route := Route(*deps.Skill, gateReports)
		checkGateCands = CheckGateCandidates(route, gateReports)
	}

	cands = append(cands, CandidateMoves(g)...)
	for _, ci := range g.CardInstances {
		if ci.Status != "active" {
			continue
		}
		spec, ok := cards.ByID(ci.CardID)
		if !ok {
			continue
		}
		cands = append(cands, ObserveCandidates(spec, ci.ID, ci.Anchors)...)
	}
	// check_gate is the lowest-priority fallback: surface_card and every coaching
	// nudge (post_intervention / observe) outrank it, so a gate report never
	// starves the coaching that moves the student toward the gate.
	cands = append(cands, checkGateCands...)

	// N3b Seam A — the semantic moment classifier. It runs ONLY when the
	// structural classifier said nothing about cards: decide-one acts on
	// cands[0], so a semantic answer produced alongside a structural
	// surface_card would simply be discarded, and paying a model call for a
	// discarded answer is waste. The resulting behaviour is also the right
	// one: structure first, semantics as the fallback that notices what
	// structure cannot see.
	if !deps.SkipSurfaceCards && trigger.Kind == "student_turn" && !hasSurfaceCard(cands) {
		if c, ok := semanticCardCandidate(ctx, deps, projectID, g, trigger.StudentText); ok {
			cands = append([]Candidate{c}, cands...)
		}
	}

	if len(cands) == 0 {
		return nil, nil // silence: nothing to say
	}
	c := cands[0] // decide-one: at most ONE action per step

	if c.Verb == "surface_card" {
		spec, ok := cards.ByID(c.CardID)
		if !ok {
			slog.Warn("agent: surface_card candidate names an unknown card", "project_id", projectID.String(), "card_id", c.CardID)
			return nil, nil
		}
		// A project-scoped candidate (AnchorKind "project", e.g. toulmin) is not
		// about any one material — its AnchorID is empty by design. Parsing that
		// empty string as a uuid would hard-error ("invalid UUID length: 0"), so
		// branch: pass uuid.Nil straight through and let SurfaceCard skip the
		// material wiring. Every other candidate carries a real material id.
		materialID := uuid.Nil
		if c.AnchorKind != "project" {
			var err error
			materialID, err = uuid.Parse(c.AnchorID)
			if err != nil {
				return nil, err
			}
		}
		return SurfaceCard(ctx, deps, projectID, spec, materialID)
	}

	if c.Verb == "check_gate" {
		// Reuse the report already reconciled above (nothing mutates between);
		// a check_gate candidate only exists when deps.Skill != nil, so
		// gateReports is populated.
		report := gateReports[c.AnchorID]
		payload, err := json.Marshal(map[string]any{"contract": c.AnchorID, "status": report.Status, "missing": report.Missing})
		if err != nil {
			return nil, err
		}
		if err := deps.Store.AppendEvent(ctx, EventRow{ProjectID: projectID, Surface: "studio", Type: "gate_checked", Payload: payload}); err != nil {
			return nil, err
		}
		return &Action{Kind: "check_gate", GateReport: &report}, nil
	}

	// History is best-effort context for the coach (Slice 5c): on a load
	// error, fall back to nil rather than failing the whole turn.
	history, err := deps.Store.LoadChatHistory(ctx, projectID, 12)
	if err != nil {
		slog.Warn("agent: load chat history failed; proceeding without it", "project_id", projectID.String(), "err", err.Error())
		history = nil
	}
	out, verdict, usage, err := ProposeIntervention(ctx, deps.Provider, deps.Resolved, g, c, history, deps.Sim, refeedPayload)
	// Usage is non-zero whenever the model call itself succeeded — including
	// when enforcement then rejects the output (err != nil): a rejected
	// reply still cost real money, so it must still be metered even though
	// it is never persisted or emitted. Metering must never fail the turn.
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if rerr := deps.Store.RecordLLMCall(ctx, LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "coach",
			Resolved: deps.Resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("agent: record llm usage failed", "project_id", projectID.String(), "err", rerr.Error())
		}
	} else if err == nil {
		// The call succeeded and produced an accepted output, yet reported no
		// usage — the provider stopped emitting it (e.g. DeepSeek's
		// stream_options.include_usage). The turn goes unmetered; do not let
		// that happen quietly.
		slog.Warn("agent: coach call returned no usage — turn is unmetered",
			"project_id", projectID.String(), "provider", deps.Resolved.Provider, "model", deps.Resolved.Model)
	}
	if err != nil {
		// Enforcement (or the model call itself) rejected the output — log
		// server-side and stay silent. A rejected output is never persisted
		// or returned to the caller.
		slog.Warn("agent: coach output not emitted", "project_id", projectID.String(), "err", err.Error())
		return nil, nil
	}

	anchorJSON, err := json.Marshal(out.Anchor)
	if err != nil {
		return nil, err
	}
	interventionID, err := deps.Store.InsertIntervention(ctx, InterventionRow{
		ProjectID:          projectID,
		Type:               out.Type,
		Anchor:             anchorJSON,
		Criterion:          out.Criterion,
		Body:               out.Body,
		Level:              c.Level,
		OutputCheckVerdict: verdict,
	})
	if err != nil {
		return nil, err
	}

	eventPayload, err := json.Marshal(map[string]any{
		"intervention_id": interventionID.String(),
		"anchor":          out.Anchor,
		"criterion":       out.Criterion,
	})
	if err != nil {
		return nil, err
	}
	if err := deps.Store.AppendEvent(ctx, EventRow{
		ProjectID: projectID,
		Surface:   "studio",
		Type:      "intervention_posted",
		Payload:   eventPayload,
	}); err != nil {
		return nil, err
	}

	return &Action{
		Kind:           "intervention",
		Output:         out,
		InterventionID: interventionID.String(),
		Verdict:        verdict,
	}, nil
}

// refeedCandidate loads the just-submitted card instance and, when it is
// COMPLETED, returns the coach candidate + the serialized payload the coach
// context renders. Any failure — unknown instance, unknown card id, load error
// — is silence: the submit itself already succeeded and must not be failed by
// its follow-up question.
func refeedCandidate(ctx context.Context, deps AgentDeps, cardInstanceID string) (Candidate, RefeedPayload, bool) {
	id, err := uuid.Parse(cardInstanceID)
	if err != nil {
		return Candidate{}, RefeedPayload{}, false
	}
	row, err := deps.Store.GetCardInstance(ctx, id)
	if err != nil {
		slog.Warn("agent: refeed load card instance failed", "card_instance_id", cardInstanceID, "err", err.Error())
		return Candidate{}, RefeedPayload{}, false
	}
	if row.Status != "completed" {
		return Candidate{}, RefeedPayload{}, false
	}
	spec, ok := cards.ByID(row.CardID)
	if !ok {
		return Candidate{}, RefeedPayload{}, false
	}
	inst := CardInstance{ID: cardInstanceID, CardID: row.CardID, Status: row.Status}
	_ = json.Unmarshal(row.Anchors, &inst.Anchors)         // absent/invalid → no anchor steps
	_ = json.Unmarshal(row.FieldValues, &inst.FieldValues) // absent/invalid → no field steps
	payload := SerializeCardForRefeed(spec, inst)

	// A card can complete with NO steps: e.g. steelman.json declares no
	// completion predicates, so EvaluateCompletion reports complete=true over
	// zero predicates, and a student who opens the card and immediately
	// submits gets status "completed" with nothing in it. An empty card is
	// not a thinking moment to respond to — asking the coach to produce one
	// anchored question about nothing would invent a question with no basis
	// in what the student actually wrote, which is exactly what 铁律 1 (AI
	// 克制，绝不替学生定论) forbids. Silence here, not a fabricated prompt.
	// This belongs in Go, not in the card JSON: it is correct for ANY card
	// that completes empty, not just steelman, and card JSON changes are out
	// of scope for this slice.
	if len(payload.Steps) == 0 {
		return Candidate{}, RefeedPayload{}, false
	}
	return Candidate{
		Verb:       "post_intervention",
		AnchorKind: "card_instance",
		AnchorID:   cardInstanceID,
		Criterion:  "D6", // 元认知与反思
		Level:      "I2",
		Reason:     "学生刚完成了一张工具卡",
	}, payload, true
}

// hasSurfaceCard reports whether any structural surface_card candidate already
// fired this turn.
func hasSurfaceCard(cands []Candidate) bool {
	for _, c := range cands {
		if c.Verb == "surface_card" {
			return true
		}
	}
	return false
}

// semanticCardCandidate runs N3b's classifier behind its structural pre-gate
// and turns a named moment into a project-scoped surface_card candidate.
//
// The call is metered even when the answer is `none` or the reply was
// unparseable — it cost real money either way. A metering failure logs and
// continues; a classifier failure is silence. Neither ever fails the turn.
func semanticCardCandidate(ctx context.Context, deps AgentDeps, projectID uuid.UUID, g GraphView, text string) (Candidate, bool) {
	// The REAL in-flight guard. SurfaceCardCandidates signals "a card is in
	// flight" by returning nil, so `!hasSurfaceCard(cands)` at the call site
	// is true PRECISELY when one is — it orders the semantic pass behind the
	// structural one, it does not gate on in-flight state. Without this, a
	// student who types another message before opening the card she was just
	// offered gets a second card minted over the first (whole-branch review
	// CRITICAL 1). An offer is never a wall, but nor is it a pile-up.
	for _, ci := range g.CardInstances {
		if ci.Status == "proposed" || ci.Status == "active" {
			return Candidate{}, false
		}
	}
	if len([]rune(strings.TrimSpace(text))) < MinClassifyRunes {
		return Candidate{}, false
	}
	eligible := EligibleMoments(g.CardInstances)
	if len(eligible) == 0 {
		return Candidate{}, false
	}
	moment, usage, err := ClassifyMoment(ctx, deps.Provider, deps.Resolved, text, eligible)
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if rerr := deps.Store.RecordLLMCall(ctx, LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "classify",
			Resolved: deps.Resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("agent: record classifier usage failed", "project_id", projectID.String(), "err", rerr.Error())
		}
	} else if err == nil {
		// The call succeeded and produced an answer, yet reported no usage —
		// the provider stopped emitting it (e.g. DeepSeek's
		// stream_options.include_usage). Mirrors the coach call's warning
		// below: the classify call goes unmetered; do not let that happen
		// quietly.
		slog.Warn("agent: classify call returned no usage — turn is unmetered",
			"project_id", projectID.String(), "provider", deps.Resolved.Provider, "model", deps.Resolved.Model)
	}
	if err != nil {
		slog.Warn("agent: moment classifier failed; staying silent", "project_id", projectID.String(), "err", err.Error())
		return Candidate{}, false
	}
	if moment == MomentNone {
		return Candidate{}, false
	}
	e := momentCard[moment]
	return Candidate{
		Verb:       "surface_card",
		AnchorKind: "project",
		AnchorID:   "",
		CardID:     e.CardID,
		Criterion:  e.Criterion,
		Reason:     e.Reason,
	}, true
}
