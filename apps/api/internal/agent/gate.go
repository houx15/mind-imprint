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

// hasAnyNodeOfType reports whether the graph has at least one node of the
// given type. Used by CheckGate to distinguish "empty" (nothing attempted)
// from "partial" (some but not enough progress) when a type-counted machine
// item (e.g. node_count_at_least) fails. Type == "" (untyped predicates like
// no_orphan_evidence) never contributes partial credit this way.
func hasAnyNodeOfType(g GraphView, nodeType string) bool {
	if nodeType == "" {
		return false
	}
	for _, n := range g.Nodes {
		if n.Type == nodeType {
			return true
		}
	}
	return false
}

// attemptedFor reports whether a machine item's Pass reflects genuine
// progress rather than a negation-style predicate passing vacuously because
// the scanned node/material type doesn't exist yet (see ItemResult.Attempted
// doc). It uses the same node-type knowledge evalMachineItem's predicates
// scan, so the two stay in lockstep — this is gate-predicate knowledge and
// belongs here, not duplicated by callers.
func attemptedFor(item skills.MachineItem, g GraphView) bool {
	switch item.Kind {
	case "node_present", "node_count_at_least":
		// A Pass here means the nodes genuinely exist; a Fail is genuine too.
		return true
	case "no_orphan_evidence":
		return hasAnyNodeOfType(g, "evidence")
	case "no_unsupported_claim", "no_single_sourced_claim":
		return hasAnyNodeOfType(g, "claim")
	case "every_source_evaluated":
		return len(g.Materials) > 0
	default:
		return false
	}
}

// RecordedGate is the externally-recorded (non-machine) status of a gate,
// read from its gate_state node body. Confirmed is the external "solid"
// (a passed challenge / human / explicit student confirmation — DEC-3); Items
// maps each student_written/human item name to "solid"|"flagged-weak"|"".
type RecordedGate struct {
	Confirmed bool
	Items     map[string]string
}

// ItemResult is one gate item's outcome in a report.
type ItemResult struct {
	Name    string
	Kind    string // "machine" | "student_written" | "human"
	Pass    bool
	Missing string
	// Attempted is only meaningful for Kind=="machine". It reports whether
	// Pass reflects genuine progress rather than a negation-style predicate
	// (no_orphan_evidence, no_unsupported_claim, no_single_sourced_claim,
	// every_source_evaluated) passing vacuously because the scanned node/
	// material type doesn't exist on the graph yet. Consumers that count
	// gate progress (e.g. studio.gatePassed) must require Pass && Attempted.
	Attempted bool
}

// GateReport is CheckGate's verdict. Status is the MACHINE computation and
// never exceeds "machine_clear" (DEC-3); Solid mirrors the recorded external
// confirmation. Missing is the ordered list of what remains.
type GateReport struct {
	Contract string
	Status   string // "empty" | "partial" | "machine_clear"
	Solid    bool
	Items    []ItemResult
	Missing  []string
}

// CheckGate evaluates a contract's gate: machine items over the graph, merged
// with the recorded status of student_written/human items. It never returns
// "solid" as a machine Status (DEC-3) — Solid is a separate recorded flag.
func CheckGate(sk skills.Skill, contractID string, g GraphView, rec RecordedGate) GateReport {
	c := sk.Contracts[contractID]
	rep := GateReport{Contract: contractID, Solid: rec.Confirmed}

	machinePass := true
	anyPass := false
	for _, m := range c.Gate.Machine {
		pass, missing := evalMachineItem(m, g)
		name := m.Kind
		if m.Type != "" {
			name = m.Kind + ":" + m.Type
		}
		rep.Items = append(rep.Items, ItemResult{Name: name, Kind: "machine", Pass: pass, Missing: missing, Attempted: attemptedFor(m, g)})
		if pass {
			anyPass = true
		} else {
			machinePass = false
			rep.Missing = append(rep.Missing, missing)
			// A failed type-counted item (e.g. node_count_at_least) can still
			// show partial progress — some but not enough matching nodes —
			// which should surface as "partial" rather than "empty".
			if hasAnyNodeOfType(g, m.Type) {
				anyPass = true
			}
		}
	}
	for _, tier := range []struct {
		names []string
		kind  string
	}{{c.Gate.StudentWritten, "student_written"}, {c.Gate.Human, "human"}} {
		for _, name := range tier.names {
			sat := rec.Items[name] == "solid"
			rep.Items = append(rep.Items, ItemResult{Name: name, Kind: tier.kind, Pass: sat})
			if sat {
				anyPass = true
			} else {
				rep.Missing = append(rep.Missing, name+" 待完成")
			}
		}
	}

	switch {
	case machinePass:
		rep.Status = "machine_clear"
	case anyPass:
		rep.Status = "partial"
	default:
		rep.Status = "empty"
	}
	return rep
}

// ReconcileGates runs CheckGate for every contract against the reconstructed
// state — the "owe every gate" diff (agent-spec §5.2). recorded may be nil.
func ReconcileGates(sk skills.Skill, g GraphView, recorded map[string]RecordedGate) map[string]GateReport {
	out := make(map[string]GateReport, len(sk.Contracts))
	for id := range sk.Contracts {
		out[id] = CheckGate(sk, id, g, recorded[id])
	}
	return out
}
