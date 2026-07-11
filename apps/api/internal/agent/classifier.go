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
