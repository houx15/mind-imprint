// Package agent — planner.go: the deterministic Project planner (agent-spec
// §5.2). Slice 4 computes the route from gate state + the contract DAG; the
// flagship model-judgment layer (prioritization, route_to_course on stalls)
// is a documented later seam.
package agent

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"mindimprint/api/internal/skills"
)

// Route is the advisory route: the unmet-and-reachable contracts in
// topological DAG order. A contract is reachable iff every `requires` gate is
// machine_clear-or-solid (work may begin once predecessors are structurally
// sound); "unmet" means not yet Solid. Pure function.
func Route(sk skills.Skill, reports map[string]GateReport) []string {
	order, err := sk.TopoOrder()
	if err != nil {
		return nil
	}
	var route []string
	for _, id := range order {
		if reports[id].Solid {
			continue // finished
		}
		reachable := true
		for _, req := range sk.Contracts[id].Requires {
			r := reports[req]
			if !(r.Solid || r.Status == "machine_clear") {
				reachable = false
				break
			}
		}
		if reachable {
			route = append(route, id)
		}
	}
	return route
}

// IntakeCandidate is one already-decomposed piece of incoming material. The
// model step that turns raw prose into candidates is out of Slice-4 scope; the
// planner mints what it is handed.
type IntakeCandidate struct {
	Type string // "claim" | "evidence" | "research_question" | ... a graph_node type
	Text string
}

// Intake maps incoming material onto the graph (author=imported), reconciles
// every gate against the reconstructed state ("owe every gate", agent-spec
// §5.2), writes the first plan artifact, and returns the initial route.
func Intake(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, candidates []IntakeCandidate) ([]string, error) {
	for _, c := range candidates {
		if _, err := deps.Store.InsertGraphNode(ctx, projectID, MintNode{
			Type:   c.Type,
			Author: "imported",
			Body:   map[string]any{"text": c.Text},
		}); err != nil {
			return nil, err
		}
	}
	route, err := writePlan(ctx, deps, projectID, sk, "intake")
	if err != nil {
		return nil, err
	}
	return route, nil
}

// writePlan reconciles gates, computes the route, upserts the plan artifact,
// and appends the plan event. Shared by Intake and Replan.
func writePlan(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, reason string) ([]string, error) {
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}
	recorded, err := deps.Store.ListGateStates(ctx, projectID)
	if err != nil {
		return nil, err
	}
	reports := ReconcileGates(sk, g, recorded)
	route := Route(sk, reports)
	body, err := json.Marshal(map[string]any{"route": route, "reason": reason})
	if err != nil {
		return nil, err
	}
	if err := deps.Store.UpsertPlan(ctx, projectID, body); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{"route": route, "reason": reason})
	if err != nil {
		return nil, err
	}
	evType := "plan_written"
	if reason != "intake" {
		evType = "plan_revised"
	}
	if err := deps.Store.AppendEvent(ctx, EventRow{
		ProjectID: projectID, Surface: "studio", Type: evType, Payload: payload,
	}); err != nil {
		return nil, err
	}
	return route, nil
}

// Replan recomputes and rewrites the route, recording the reason as a
// plan_revised event (agent-spec §5.2).
func Replan(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, reason string) ([]string, error) {
	return writePlan(ctx, deps, projectID, sk, reason)
}

// Advance is the blocking unlock (DEC-8): it confirms a contract's gate solid
// only when every item is satisfied — machine items computed AND every
// student_written/human item recorded solid. It never marks a non-machine
// item itself (DEC-3). A machine-only gate advances on machine_clear. Records
// a gate_attempt event either way.
func Advance(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill, contractID string) (bool, error) {
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return false, err
	}
	recorded, err := deps.Store.ListGateStates(ctx, projectID)
	if err != nil {
		return false, err
	}
	rec := recorded[contractID]
	report := CheckGate(sk, contractID, g, rec)

	var missing []string
	if report.Status != "machine_clear" {
		missing = append(missing, report.Missing...)
	} else {
		c := sk.Contracts[contractID]
		for _, name := range append(append([]string{}, c.Gate.StudentWritten...), c.Gate.Human...) {
			if rec.Items[name] != "solid" {
				missing = append(missing, name+" 待完成")
			}
		}
	}

	if len(missing) > 0 {
		payload, _ := json.Marshal(map[string]any{"contract": contractID, "result": "blocked", "missing": missing})
		if err := deps.Store.AppendEvent(ctx, EventRow{ProjectID: projectID, Surface: "studio", Type: "gate_attempt", Payload: payload}); err != nil {
			return false, err
		}
		return false, nil
	}

	rec.Confirmed = true
	if err := deps.Store.UpsertGateState(ctx, projectID, contractID, rec); err != nil {
		return false, err
	}
	payload, _ := json.Marshal(map[string]any{"contract": contractID, "result": "passed"})
	if err := deps.Store.AppendEvent(ctx, EventRow{ProjectID: projectID, Surface: "studio", Type: "gate_attempt", Payload: payload}); err != nil {
		return false, err
	}
	return true, nil
}
