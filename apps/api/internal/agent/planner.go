// Package agent — planner.go: the deterministic Project planner (agent-spec
// §5.2). Slice 4 computes the route from gate state + the contract DAG; the
// flagship model-judgment layer (prioritization, route_to_course on stalls)
// is a documented later seam.
package agent

import (
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
