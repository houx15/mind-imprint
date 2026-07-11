package agent

import (
	"fmt"

	"mindimprint/api/internal/skills"
)

// evalMachineItem evaluates one closed-set machine gate predicate over the
// graph (agent-spec §5.1; Slice 4's closed set). Returns pass + a
// human-readable "what's missing" when it fails. A new kind is a runtime
// change — an unknown kind fails closed with a diagnostic.
func evalMachineItem(item skills.MachineItem, g GraphView) (bool, string) {
	switch item.Kind {
	case "node_present":
		for _, n := range g.Nodes {
			if n.Type == item.Type {
				return true, ""
			}
		}
		return false, fmt.Sprintf("缺少 %s", item.Type)
	case "node_count_at_least":
		c := 0
		for _, n := range g.Nodes {
			if n.Type == item.Type {
				c++
			}
		}
		if c >= item.N {
			return true, ""
		}
		return false, fmt.Sprintf("%s 需要至少 %d 个（当前 %d）", item.Type, item.N, c)
	case "no_orphan_evidence":
		for _, n := range g.Nodes {
			if n.Type == "evidence" && !hasOutgoingSupports(n.ID, g) {
				return false, "存在未连到主张的证据"
			}
		}
		return true, ""
	case "no_unsupported_claim":
		for _, n := range g.Nodes {
			if n.Type == "claim" && len(supportingEvidence(n.ID, g)) == 0 {
				return false, "存在没有证据支撑的主张"
			}
		}
		return true, ""
	case "no_single_sourced_claim":
		for _, n := range g.Nodes {
			if n.Type == "claim" && len(supportingEvidence(n.ID, g)) < 2 {
				return false, "存在只靠单一来源的主张"
			}
		}
		return true, ""
	case "every_source_evaluated":
		for _, m := range g.Materials {
			if m.Kind != "article" {
				continue
			}
			if !hasEvaluatedEdge(m.ID, g) {
				return false, "有来源尚未做来源评估"
			}
		}
		return true, ""
	default:
		return false, fmt.Sprintf("未知的机器判据 %q", item.Kind)
	}
}

func hasOutgoingSupports(nodeID string, g GraphView) bool {
	for _, e := range g.Edges {
		if e.Type == "supports" && e.FromKind == "graph_node" && e.FromID == nodeID {
			return true
		}
	}
	return false
}

// supportingEvidence returns the distinct evidence node ids that support a
// claim via a "supports" edge (evidence -> claim). Slice-4 proxy for source
// distinctness (refined to distinct source materials in Slice 7).
func supportingEvidence(claimID string, g GraphView) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range g.Edges {
		if e.Type == "supports" && e.ToKind == "graph_node" && e.ToID == claimID && e.FromKind == "graph_node" {
			if !seen[e.FromID] {
				seen[e.FromID] = true
				out = append(out, e.FromID)
			}
		}
	}
	return out
}

func hasEvaluatedEdge(materialID string, g GraphView) bool {
	for _, e := range g.Edges {
		if e.Type == "evaluated-as" && e.FromKind == "material" && e.FromID == materialID {
			return true
		}
	}
	return false
}

// EvalMachineItemForTest exposes evalMachineItem for the gate_test.go unit
// tests without widening the package API surface elsewhere.
func EvalMachineItemForTest(item skills.MachineItem, g GraphView) (bool, string) {
	return evalMachineItem(item, g)
}
