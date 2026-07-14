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

// craapCardID is the first card Slice 3 surfaces automatically (agent-spec
// §5.7): a source material with no evaluation gets CRAAP proposed onto it.
// siftCardID is Slice 6c's follow-on: once CRAAP has finished, the same
// material becomes eligible for a lateral SIFT check (see the SIFT branch
// below). Slice 4's planner generalizes "which card for which target" beyond
// these two hardcoded mappings.
const (
	craapCardID = "craap"
	siftCardID  = "sift"
)

// SurfaceCardCandidates implements the surface_card trigger predicate
// (design §3, agent-spec §4.2) for two stages over the same source
// ("article") material, in sequence:
//
//  1. CRAAP: a material with no card_instance already evaluating it and no
//     evidence node minted from it yields a surface_card candidate naming
//     CRAAP. An already-surfaced or already-evaluated source yields none —
//     CRAAP never re-proposes itself onto the same material.
//  2. SIFT: once CRAAP has finished (an "evaluated-as" edge exists), the
//     SAME material becomes eligible for a lateral cross-check — UNLESS it
//     already has one (a "cross-checked-by" edge) or a SIFT card_instance is
//     already surfaced on it (in flight), or it is itself a lateral
//     instrument some other cross_check cited (a "cites" edge FROM a
//     cross_check node TO this material). That last exclusion is the
//     treadmill guard: without it, adding a lateral source, having it
//     auto-CRAAP'd, and then having SIFT propose on IT TOO would let every
//     lateral read manufacture its own next chore, forever.
//
// Pure, no DB, no model call; perceive (loop.go) supplies the GraphView's
// Nodes/Materials/Edges/CardInstances. Candidates are returned in material
// order (stable), at most one per material.
//
// Whole-branch-review CRITICAL 1: while ANY card_instance is "proposed" (offered,
// not yet opened) or "active" (open, being filled), this function is silent
// PROJECT-WIDE — not just on the in-flight card's own material. The loop's
// decide-one (loop.go) only ever acts on cands[0], and surface_card outranks
// every post_intervention/observe candidate, so a surface_card proposed on a
// completely different, untouched material would still win and get applied
// unconditionally — unmounting the open card and orphaning whatever the
// student has typed into it (held only in client state until submit; the
// row itself is left a permanently "active" zombie no later turn can ever
// re-surface, since surface_card never re-proposes onto an already-spoken-for
// material). Suppressing here, in the classifier, also means the loop never
// even reaches SurfaceCard — no zombie card_instance is minted in the first
// place. This is SUPPRESSION, not queueing: the candidate simply isn't
// produced this turn: it will be proposed again once nothing is in flight.
func SurfaceCardCandidates(g GraphView) []Candidate {
	for _, ci := range g.CardInstances {
		if ci.Status == "proposed" || ci.Status == "active" {
			return nil
		}
	}

	// card_instance id -> card id: an "evaluates" edge only carries endpoint
	// ids, so this is how a material's surfaced-card edge gets attributed to
	// the SPECIFIC card that minted it (CRAAP vs SIFT can both mint one on
	// the same material, at different stages).
	cardOf := make(map[string]string, len(g.CardInstances))
	for _, ci := range g.CardInstances {
		cardOf[ci.ID] = ci.CardID
	}

	crossCheckNode := make(map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		if n.Type == "cross_check" {
			crossCheckNode[n.ID] = true
		}
	}

	evaluated := make(map[string]bool, len(g.Edges))         // any card (CRAAP or SIFT) already surfaced or promoted here
	evaluatedAs := make(map[string]bool, len(g.Edges))       // CRAAP has finished on this material
	crossCheckedBy := make(map[string]bool, len(g.Edges))    // this material already has its own cross_check
	siftSurfaced := make(map[string]bool, len(g.Edges))      // a SIFT card_instance already targets this material
	lateralInstrument := make(map[string]bool, len(g.Edges)) // this material IS some cross_check's lateral source
	for _, e := range g.Edges {
		switch {
		case e.FromKind == "card_instance" && e.ToKind == "material":
			evaluated[e.ToID] = true
			if cardOf[e.FromID] == siftCardID {
				siftSurfaced[e.ToID] = true
			}
		case e.FromKind == "material" && e.ToKind == "graph_node" && e.Type == "evaluated-as":
			evaluated[e.FromID] = true
			evaluatedAs[e.FromID] = true
		case e.FromKind == "material" && e.ToKind == "graph_node" && e.Type == "cross-checked-by":
			crossCheckedBy[e.FromID] = true
		case e.FromKind == "graph_node" && e.ToKind == "material" && e.Type == "cites" && crossCheckNode[e.FromID]:
			lateralInstrument[e.ToID] = true
		}
	}

	var out []Candidate
	for _, m := range g.Materials {
		if m.Kind != "article" {
			continue
		}
		if !evaluated[m.ID] {
			out = append(out, Candidate{
				Verb:       "surface_card",
				AnchorKind: "material",
				AnchorID:   m.ID,
				CardID:     craapCardID,
				Reason:     "source material has no evaluation card",
			})
			continue
		}
		if evaluatedAs[m.ID] && !crossCheckedBy[m.ID] && !siftSurfaced[m.ID] && !lateralInstrument[m.ID] {
			out = append(out, Candidate{
				Verb:       "surface_card",
				AnchorKind: "material",
				AnchorID:   m.ID,
				CardID:     siftCardID,
				Reason:     "evaluated source has no lateral cross-check yet",
			})
		}
	}
	return out
}

// CheckGateCandidates proposes a structural check_gate for the contract the
// route currently points at, when that contract's gate is not yet
// machine_clear. It is a no-model action (like surface_card): the loop runs
// CheckGate and records the report. At most one candidate. The loop ranks it
// BELOW post_intervention — a check_gate report is a no-op read that never
// clears itself, so it must never preempt a coaching nudge (which is what moves
// the gate toward machine_clear); it fires only as an idle fallback.
func CheckGateCandidates(route []string, reports map[string]GateReport) []Candidate {
	for _, id := range route {
		if reports[id].Status != "machine_clear" {
			return []Candidate{{
				Verb:       "check_gate",
				AnchorKind: "contract",
				AnchorID:   id,
				Level:      "I1",
				Reason:     "gate not yet machine-clear",
			}}
		}
	}
	return nil
}
