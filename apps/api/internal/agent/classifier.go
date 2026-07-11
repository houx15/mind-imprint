package agent

// CandidateMoves implements the Slice-2 trigger predicate (design §3): for
// each claim node with no incoming "supports" edge from an evidence node,
// emit a post_intervention candidate anchored to that claim. Pure, no DB,
// no model call — perceive (loop.go, Task 4) supplies the GraphView; the
// classifier never talks to the model or the student. Candidates are
// returned in node order (stable).
func CandidateMoves(g GraphView) []Candidate {
	nodeByID := make(map[string]GraphNodeView, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeByID[n.ID] = n
	}

	supported := make(map[string]bool)
	for _, e := range g.Edges {
		if e.Type != "supports" {
			continue
		}
		from, ok := nodeByID[e.FromID]
		if !ok || from.Type != "evidence" {
			continue
		}
		supported[e.ToID] = true
	}

	var out []Candidate
	for _, n := range g.Nodes {
		if n.Type != "claim" || supported[n.ID] {
			continue
		}
		out = append(out, Candidate{
			Verb:       "post_intervention",
			AnchorKind: "graph_node",
			AnchorID:   n.ID,
			Criterion:  "D5",
			Reason:     "claim has no supporting evidence",
			Level:      "I2",
		})
	}
	return out
}

// craapCardID is the only card Slice 3 surfaces automatically (agent-spec
// §5.7): a source material with no evaluation gets CRAAP proposed onto it.
// Slice 4's planner generalizes "which card for which target" beyond this
// single hardcoded mapping.
const craapCardID = "craap"

// SurfaceCardCandidates implements the surface_card trigger predicate
// (design §3, agent-spec §4.2): a source ("article") material with no
// card_instance already evaluating it and no evidence node minted from it
// yields a surface_card candidate naming CRAAP. An already-surfaced or
// already-evaluated source yields none — surface_card never re-proposes
// itself onto the same material. Pure, no DB; perceive (loop.go) supplies
// the GraphView's Materials + Edges. Candidates are returned in material
// order (stable).
func SurfaceCardCandidates(g GraphView) []Candidate {
	evaluated := make(map[string]bool, len(g.Edges))
	for _, e := range g.Edges {
		switch {
		case e.FromKind == "card_instance" && e.ToKind == "material":
			evaluated[e.ToID] = true
		case e.FromKind == "material" && e.ToKind == "graph_node" && e.Type == "evaluated-as":
			evaluated[e.FromID] = true
		}
	}

	var out []Candidate
	for _, m := range g.Materials {
		if m.Kind != "article" || evaluated[m.ID] {
			continue
		}
		out = append(out, Candidate{
			Verb:       "surface_card",
			AnchorKind: "material",
			AnchorID:   m.ID,
			CardID:     craapCardID,
			Reason:     "source material has no evaluation card",
		})
	}
	return out
}
