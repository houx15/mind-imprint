package agent

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent/enforcement"
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

// AgentStore is the runtime loop's persistence seam: perceive (LoadGraph)
// and record (InsertIntervention, AppendEvent). The sqlc-backed adapter
// lives in agentstore.go; loop_test.go uses an in-memory fake.
type AgentStore interface {
	LoadGraph(ctx context.Context, projectID uuid.UUID) (GraphView, error)
	InsertIntervention(ctx context.Context, row InterventionRow) (uuid.UUID, error)
	AppendEvent(ctx context.Context, row EventRow) error
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
func RunAgentStep(ctx context.Context, deps AgentDeps, projectID uuid.UUID, trigger Trigger) (*Action, error) {
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}

	cands := CandidateMoves(g)
	if len(cands) == 0 {
		return nil, nil // silence: nothing to say
	}
	c := cands[0] // decide-one: at most ONE action per step

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
		Output:         out,
		InterventionID: interventionID.String(),
		Verdict:        verdict,
	}, nil
}
