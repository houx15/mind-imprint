package studio

import (
	"encoding/json"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/store/sqlc"
)

// ProjectData is the fully-loaded read set for one project (spec §3).
type ProjectData struct {
	Project       sqlc.Project
	Nodes         []sqlc.GraphNode
	Edges         []sqlc.GraphEdge
	GateStates    []sqlc.GraphNode // gate_state nodes (recorded)
	Plan          *sqlc.GraphNode  // plan node, nil if none
	Interventions []sqlc.Intervention
	Cards         []sqlc.CardInstance
}

type planBody struct {
	Route  []string `json:"route"`
	Reason string   `json:"reason"`
}

func planRoute(plan *sqlc.GraphNode) []string {
	if plan == nil {
		return nil
	}
	var b planBody
	if err := json.Unmarshal(plan.Body, &b); err != nil {
		return nil
	}
	return b.Route
}

// projectStations maps the skill's contract DAG onto S0..S6 and returns the
// stations plus the current station's code.
func projectStations(sk skills.Skill, d ProjectData) ([]StationDTO, string, error) {
	order, err := sk.TopoOrder()
	if err != nil {
		return nil, "", err
	}
	g := agent.GraphViewFromRows(d.Nodes, d.Edges, nil, d.Cards)
	recorded := agent.RecordedGatesFromNodes(d.GateStates)
	reports := agent.ReconcileGates(sk, g, recorded)

	route := planRoute(d.Plan)
	head := "" // the plan's current contract
	if len(route) > 0 {
		head = route[0]
	} else {
		for _, id := range order { // no plan: first non-solid in topo order
			if !reports[id].Solid {
				head = id
				break
			}
		}
	}

	stations := make([]StationDTO, 0, len(order))
	current := ""
	for i, id := range order {
		c := sk.Contracts[id]
		rep := reports[id]
		st := StationDTO{Code: stationCode(i), Name: c.Title, View: c.View}
		switch {
		case rep.Solid:
			st.State = "done"
			// a done contract still on the route can be revisited
			if inRoute(route, id) {
				st.Backflow = true
			}
		case id == head:
			st.State = "current"
			current = st.Code
		default:
			st.State = "locked"
		}
		if total := gateTotal(c); total > 0 {
			st.Gate = &GateDTO{Total: total, Passed: gatePassed(rep, recorded[id], g)}
		}
		stations = append(stations, st)
	}
	if current == "" && len(stations) > 0 { // fully done: last station is current
		current = stations[len(stations)-1].Code
	}
	return stations, current, nil
}

func stationCode(i int) string { return "S" + string(rune('0'+i)) }

func inRoute(route []string, id string) bool {
	for _, r := range route {
		if r == id {
			return true
		}
	}
	return false
}

func gateTotal(c skills.Contract) int {
	return len(c.Gate.Machine) + len(c.Gate.StudentWritten) + len(c.Gate.Human)
}

// gatePassed counts satisfied gate items: machine items reported Pass (minus
// vacuous passes on untouched data — see vacuousMachinePass), plus
// student_written/human items recorded "solid".
func gatePassed(rep agent.GateReport, rec agent.RecordedGate, g agent.GraphView) int {
	n := 0
	for _, it := range rep.Items {
		if it.Kind == "machine" {
			if it.Pass && !vacuousMachinePass(it.Name, g) {
				n++
			}
			continue
		}
		if rec.Items[it.Name] == "solid" {
			n++
		}
	}
	return n
}

// vacuousMachinePass reports whether a machine item's Pass=true reflects
// "nothing to check yet" rather than genuine progress. evalMachineItem's
// negation-style predicates (agent/gate.go) are vacuously true over an empty
// candidate set — e.g. no_orphan_evidence passes when there is no evidence at
// all. For gate-progress display that vacuous pass must not count, or an
// untouched contract would show partial credit. name is the ItemResult.Name
// ("kind" or "kind:type"); only the kind prefix matters here.
func vacuousMachinePass(name string, g agent.GraphView) bool {
	kind := name
	if i := strings.IndexByte(name, ':'); i >= 0 {
		kind = name[:i]
	}
	switch kind {
	case "no_orphan_evidence", "no_unsupported_claim", "no_single_sourced_claim":
		return len(g.Nodes) == 0
	case "every_source_evaluated":
		return len(g.Materials) == 0
	default:
		return false
	}
}
