package agent

import (
	"context"
	"encoding/json"
	"log/slog"

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

	InsertDisposition(ctx context.Context, interventionID uuid.UUID, action, reason string) (uuid.UUID, error)

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

	var cands []Candidate
	if !deps.SkipSurfaceCards {
		cands = SurfaceCardCandidates(g)
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
		materialID, err := uuid.Parse(c.AnchorID)
		if err != nil {
			return nil, err
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
	out, verdict, usage, err := ProposeIntervention(ctx, deps.Provider, deps.Resolved, g, c, history, deps.Sim)
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
