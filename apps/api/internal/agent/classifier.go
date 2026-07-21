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

	// toulminCardID surfaces once the student has evaluated material to argue
	// from (any evaluated-as edge) and has not yet started an argument (no
	// claim node). It is project-scoped, not per-material, so it is decided
	// after the per-material loop, from graph-wide facts.
	toulminCardID = "toulmin"

	// perspectiveMatrixCardID is N3a's matrix card, and the ONLY producer of
	// `perspective` graph nodes anywhere in the system. That is why it must be
	// wired: writing-project.json's `evaluate_perspectives` station gates on
	// node_count_at_least{type:"perspective", n:2} and is a `requires`
	// prerequisite of `evaluate_sources` — with no producer, that gate is
	// unsatisfiable and the whole station chain is walled behind it. The card's
	// own min_items (2) lines up with the gate's n (2) by construction.
	//
	// N3a's other two cards (fact-opinion-value / certainty-spectrum) are
	// deliberately NOT wired here: their moment is a semantic judgment about
	// what the student just wrote, which needs the classifier scoped to N3b.
	perspectiveMatrixCardID = "perspective-matrix"
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

	// Graph-wide facts for the PROJECT-SCOPED branches below (Toulmin,
	// perspective-matrix): those cards are not about one material, so they are
	// decided from the graph as a whole rather than inside the per-material
	// loop above. The in-flight guard at the top of this function already
	// suppresses all of them while any card_instance is proposed/active.
	anyEvaluated := false
	hasClaim := false
	perspectiveNodes := 0
	for _, e := range g.Edges {
		if e.Type == "evaluated-as" && e.FromKind == "material" {
			anyEvaluated = true
		}
	}
	for _, n := range g.Nodes {
		if n.Type == "claim" {
			hasClaim = true
		}
		if n.Type == "perspective" {
			perspectiveNodes++
		}
	}

	// Project-scoped perspective-matrix surface (N3a): the missing producer for
	// evaluate_perspectives' node_count_at_least{perspective,2} gate.
	//
	// PLACEMENT — before the Toulmin branch, after the per-material loop:
	//   * after the per-material loop, because like Toulmin this is a card ABOUT
	//     the project, decided from graph-wide facts rather than one source;
	//   * before Toulmin, because the skill's station chain puts
	//     evaluate_perspectives strictly upstream of the argument work (it is a
	//     `requires` prerequisite of evaluate_sources, whose evidence a claim
	//     argues from). The loop only ever acts on cands[0], so when both are
	//     eligible in the same turn, list order IS the priority — and mapping
	//     the terrain should precede committing to a position on it.
	//
	// MOMENT — gated on anyEvaluated, exactly the signal the Toulmin branch
	// uses: the cheapest honest "this project has real substance in it now"
	// fact available here. Firing on a bare, empty project would be an ambush
	// (铁律 2), and re-deriving the full gate DAG belongs in ReconcileGates,
	// not in a trigger predicate.
	//
	// SUPPRESSION — the in-flight guard at the top of this function covers
	// proposed/active. It does NOT cover a card the student already finished or
	// SKIPPED: a project-scoped SurfaceCard mints no card_instance->material
	// edge (there is no material), so the per-material `evaluated` bookkeeping
	// above cannot see it. So suppress on the existence of ANY
	// perspective-matrix card_instance in ANY status. An offer is never a wall:
	// once she has said no, we do not ask again.
	perspectiveMatrixSeen := false
	for _, ci := range g.CardInstances {
		if ci.CardID == perspectiveMatrixCardID {
			perspectiveMatrixSeen = true
		}
	}
	if anyEvaluated && perspectiveNodes < 2 && !perspectiveMatrixSeen {
		out = append(out, Candidate{
			Verb:       "surface_card",
			AnchorKind: "project",
			AnchorID:   "",
			CardID:     perspectiveMatrixCardID,
			Reason:     "project argues from fewer than two perspectives",
		})
	}

	// Project-scoped Toulmin surface (Slice 7): trigger once ANY source is
	// evaluated (there is something to argue from) and NO claim node exists yet
	// (no argument started). This is deliberately the cheapest honest signal
	// available here — "S3 has produced its structural output" — rather than
	// re-deriving the full gate DAG (that lives in ReconcileGates); it does not
	// check per-material completeness or which specific evidence a claim might
	// eventually cite.
	//
	// Toulmin skip-suppression (N3b): like perspective-matrix above, ANY
	// toulmin card_instance — including a skipped one — retires the offer.
	// Without this, skipping toulmin mints no claim node (GraphEffects'
	// "toulmin" case only mints a node from a filled slot), so
	// anyEvaluated && !hasClaim stays true and the card re-offers forever. An
	// offer is never a wall (铁律 2 · 不操纵).
	toulminSeen := false
	for _, ci := range g.CardInstances {
		if ci.CardID == toulminCardID {
			toulminSeen = true
		}
	}
	if anyEvaluated && !hasClaim && !toulminSeen {
		out = append(out, Candidate{
			Verb:       "surface_card",
			AnchorKind: "project",
			AnchorID:   "",
			CardID:     toulminCardID,
			Reason:     "sources evaluated but no argument started",
		})
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
