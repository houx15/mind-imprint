// Package agent — planner.go: the deterministic Project planner (agent-spec
// §5.2). Slice 4 computes the route from gate state + the contract DAG; the
// flagship model-judgment layer (prioritization, route_to_course on stalls)
// is a documented later seam.
package agent

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/google/uuid"

	"mindimprint/api/internal/skills"
)

// planBody is the canonical shape of the project's single `plan` graph-node.
// Route/Reason are the advisory plan; Waived is N6-E's per-project journey —
// the contract ids the composer (or the student, by re-opening) has set aside.
type planBody struct {
	Route  []string `json:"route"`
	Reason string   `json:"reason"`
	Waived []string `json:"waived,omitempty"`
}

// Route is the advisory route: the unmet-and-reachable contracts in
// topological DAG order. A contract is reachable iff every `requires` gate is
// machine_clear-or-solid (work may begin once predecessors are structurally
// sound); "unmet" means not yet Solid. A waived contract (N6-E's journey) is
// excluded from the route the same way a Solid one is, and also counts as
// satisfied for its successors' reachability check. waived is nil-safe. Pure
// function.
func Route(sk skills.Skill, reports map[string]GateReport, waived map[string]bool) []string {
	order, err := sk.TopoOrder()
	if err != nil {
		return nil
	}
	var route []string
	for _, id := range order {
		if reports[id].Solid || waived[id] {
			continue // finished, or waived out of this journey
		}
		reachable := true
		for _, req := range sk.Contracts[id].Requires {
			r := reports[req]
			if !(r.Solid || r.Status == "machine_clear" || waived[req]) {
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
	waived, err := deps.Store.LoadWaived(ctx, projectID)
	if err != nil {
		return nil, err
	}
	reports := ReconcileGates(sk, g, recorded)
	route := Route(sk, reports, waived)
	waivedList := make([]string, 0, len(waived))
	for id := range waived {
		waivedList = append(waivedList, id)
	}
	sort.Strings(waivedList) // deterministic body
	body, err := json.Marshal(planBody{Route: route, Reason: reason, Waived: waivedList})
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

	// CheckGate.Missing already accumulates BOTH the failing machine items and
	// every unrecorded student_written/human item — so a non-empty Missing is
	// exactly the DEC-8 refuse condition. The gate passes only when every item
	// (machine computed + non-machine recorded "solid") is satisfied; Advance
	// never records a non-machine item itself (DEC-3).
	if missing := report.Missing; len(missing) > 0 {
		payload, _ := json.Marshal(map[string]any{"contract": contractID, "result": "blocked", "missing": missing})
		if err := deps.Store.AppendEvent(ctx, EventRow{ProjectID: projectID, Surface: "studio", Type: "gate_attempt", Payload: payload}); err != nil {
			return false, err
		}
		return false, nil
	}

	rec.Confirmed = true
	payload, _ := json.Marshal(map[string]any{"contract": contractID, "result": "passed"})
	if err := deps.Store.ConfirmGate(ctx, projectID, contractID, rec, EventRow{ProjectID: projectID, Surface: "studio", Type: "gate_attempt", Payload: payload}); err != nil {
		return false, err
	}
	return true, nil
}

// AdvanceAll confirms every contract that is now genuinely finished, walking
// the contract DAG once in topological order, and returns the ids it newly
// advanced. It is the live caller Advance never had: before N3d nothing in
// production ever set RecordedGate.Confirmed, so no station could become
// `done` for any project whose gate_state rows weren't hand-written by a seed
// migration.
//
// A contract advances iff (a) it is not already solid, (b) every id in its
// Requires is solid — already recorded, or advanced earlier in THIS walk — and
// (c) its own gate has nothing Missing. Rule (b) is deliberately stricter than
// Route's reachability (which admits a machine_clear predecessor): work may
// begin once predecessors are structurally sound, but a station is only
// FINISHED behind finished predecessors.
//
// DEC-3 holds throughout: this delegates the confirm to Advance, which never
// records a student_written or human item itself.
func AdvanceAll(ctx context.Context, deps AgentDeps, projectID uuid.UUID, sk skills.Skill) ([]string, error) {
	order, err := sk.TopoOrder()
	if err != nil {
		return nil, err
	}
	g, err := deps.Store.LoadGraph(ctx, projectID)
	if err != nil {
		return nil, err
	}
	recorded, err := deps.Store.ListGateStates(ctx, projectID)
	if err != nil {
		return nil, err
	}
	waived, err := deps.Store.LoadWaived(ctx, projectID)
	if err != nil {
		return nil, err
	}
	solid := make(map[string]bool, len(order))
	for _, id := range order {
		// A waived contract counts as satisfied for successors, but is NEVER
		// itself advanced/confirmed (the `if solid[id] { continue }` below skips
		// it before Advance can run) — waived ≠ done (DEC-3, 铁律 4).
		solid[id] = recorded[id].Confirmed || waived[id]
	}

	var advanced []string
	for _, id := range order {
		if solid[id] {
			continue
		}
		ready := true
		for _, req := range sk.Contracts[id].Requires {
			if !solid[req] {
				ready = false
				break
			}
		}
		if !ready {
			continue
		}
		if len(CheckGate(sk, id, g, recorded[id]).Missing) > 0 {
			continue
		}
		ok, err := Advance(ctx, deps, projectID, sk, id)
		if err != nil {
			return nil, err
		}
		if ok {
			solid[id] = true
			advanced = append(advanced, id)
		}
	}
	if len(advanced) > 0 {
		if _, err := Replan(ctx, deps, projectID, sk, "advanced"); err != nil {
			return nil, err
		}
	}
	return advanced, nil
}
