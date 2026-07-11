package agent

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
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

// EventRow is the persistence payload for one appended C4 event.
type EventRow struct {
	ProjectID uuid.UUID
	Surface   string
	Type      string
	Payload   []byte
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

	// CreateCardInstance instantiates a proposed card_instance for cardID
	// on materialID (SurfaceCard, card_lifecycle.go). The legacy
	// card_instances.task_id NOT NULL FK is resolved from materialID's own
	// task_id by the adapter — pure agent code never has to know about
	// tasks.
	CreateCardInstance(ctx context.Context, projectID, materialID uuid.UUID, cardID, contractRef string) (CardInstanceRow, error)
	GetCardInstance(ctx context.Context, id uuid.UUID) (CardInstanceRow, error)
	SetCardInstanceFramework(ctx context.Context, projectID, id uuid.UUID, framework []byte) error

	// InsertGraphNode/InsertGraphEdge apply one card's graph_effects
	// (CompleteCard, card_lifecycle.go) and mint SurfaceCard's
	// card_instance->material edge. node/edge ids must already be resolved
	// to real uuids (no "$new:" placeholders) by the caller.
	InsertGraphNode(ctx context.Context, projectID uuid.UUID, node MintNode) (uuid.UUID, error)
	InsertGraphEdge(ctx context.Context, projectID uuid.UUID, edge MintEdge) error

	InsertDisposition(ctx context.Context, interventionID uuid.UUID, action, reason string) (uuid.UUID, error)

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
}

// RunAgentStep runs one perceive -> classify -> decide-one -> act -> enforce
// -> record pass for a project (design §2). Silence is a first-class
// outcome — both "no candidate move" and "the coach's output was rejected
// by enforcement" return (nil, nil); a rejected output is never persisted.
// trigger identifies which tier (T-A/T-B/T-C) invoked this step; Slice 2's
// single trigger predicate does not branch on it yet.
//
// Candidate ordering (Task 5): surface_card candidates come first, ahead of
// every post_intervention candidate (whether from the unsupported-claim
// predicate or an active card's observe rules) — surfacing an unevaluated
// source is a one-time offer the student can act on immediately, while a
// post_intervention nudge can always wait one more step. Within each tier,
// candidates stay in the classifier's stable (material/node) order.
func RunAgentStep(ctx context.Context, deps AgentDeps, projectID uuid.UUID, trigger Trigger) (*Action, error) {
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}

	cands := SurfaceCardCandidates(g)
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

	out, verdict, err := ProposeIntervention(ctx, deps.Provider, deps.Resolved, g, c, deps.Sim)
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
