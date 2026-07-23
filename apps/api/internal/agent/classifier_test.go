package agent

import (
	"testing"

	"mindimprint/api/internal/skills"
)

func TestCandidateMoves_ClaimWithoutEvidence(t *testing.T) {
	g := GraphView{
		Nodes: []GraphNodeView{{ID: "n1", Type: "claim", Author: "student"}},
		Edges: nil, // no evidence supports the claim
	}
	cands := CandidateMoves(g)
	if len(cands) != 1 {
		t.Fatalf("want 1 candidate, got %d", len(cands))
	}
	if cands[0].Verb != "post_intervention" || cands[0].AnchorID != "n1" {
		t.Fatalf("unexpected candidate: %+v", cands[0])
	}
}

func TestCandidateMoves_SupportedClaimIsSilent(t *testing.T) {
	g := GraphView{
		Nodes: []GraphNodeView{{ID: "n1", Type: "claim", Author: "student"}, {ID: "e1", Type: "evidence", Author: "student"}},
		Edges: []GraphEdgeView{{FromKind: "graph_node", FromID: "e1", ToKind: "graph_node", ToID: "n1", Type: "supports"}},
	}
	if got := CandidateMoves(g); len(got) != 0 {
		t.Fatalf("supported claim must be silent, got %d", len(got))
	}
}

// TestSurfaceCardCandidates_UnevaluatedSourceFires covers design §3's
// surface_card trigger predicate: a source ("article") material with no
// card_instance evaluating it and no evidence node minted from it yields a
// surface_card candidate naming CRAAP.
func TestSurfaceCardCandidates_UnevaluatedSourceFires(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
	}
	got := SurfaceCardCandidates(g)
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
	c := got[0]
	if c.Verb != "surface_card" || c.AnchorID != "m1" || c.CardID != "craap" {
		t.Fatalf("unexpected candidate: %+v", c)
	}
}

// TestSurfaceCardCandidates_InFlightCraapIsSilent covers one way a material
// can already be "spoken for" as far as CRAAP goes: an existing
// card_instance->material edge (surfaced, maybe still in flight) never gets a
// second CRAAP candidate. (The other way — a material whose CRAAP has
// already completed, i.e. an "evaluated-as" edge — used to silence the
// predicate entirely; it no longer does, because that is now exactly the
// signal that makes the material eligible for SIFT instead. See the
// TestSurfaceCardCandidates_SIFT_* tests below for that behavior.)
func TestSurfaceCardCandidates_InFlightCraapIsSilent(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
		Edges: []GraphEdgeView{
			{FromKind: "card_instance", FromID: "ci1", ToKind: "material", ToID: "m1"},
		},
	}
	if got := SurfaceCardCandidates(g); len(got) != 0 {
		t.Fatalf("already-surfaced material must be silent, got %d", len(got))
	}
}

// TestSurfaceCardCandidates_SIFT_FiresAfterCraapEvaluated is the summon-hop
// keystone (whole-branch review finding [1]): a material with a finished
// CRAAP evaluation (an "evaluated-as" edge) and no cross-check of its own yet
// must propose SIFT. Before this, SurfaceCardCandidates only ever knew how to
// propose "craap" — this material could never surface a SIFT card_instance,
// no matter how the rest of the agent runtime evolved.
func TestSurfaceCardCandidates_SIFT_FiresAfterCraapEvaluated(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
		Edges: []GraphEdgeView{
			{FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "e1", Type: "evaluated-as"},
		},
	}
	got := SurfaceCardCandidates(g)
	// Slice 7: this same evaluated-as-edge/no-claim state also satisfies the
	// project-scoped Toulmin surface rule. N3a: and the project-scoped
	// perspective-matrix rule (a source is evaluated, zero perspective nodes),
	// so all three are expected here.
	if len(got) != 3 {
		t.Fatalf("candidates = %d, want 3 (SIFT + perspective-matrix + toulmin), got %+v", len(got), got)
	}
	c := got[0]
	if c.Verb != "surface_card" || c.AnchorID != "m1" || c.CardID != siftCardID {
		t.Fatalf("unexpected candidate: %+v", c)
	}
	if !hasCard(got, toulminCardID) {
		t.Fatalf("expected toulmin to also surface alongside SIFT, got %+v", got)
	}
	if !hasCard(got, perspectiveMatrixCardID) {
		t.Fatalf("expected perspective-matrix to also surface alongside SIFT, got %+v", got)
	}
}

// TestSurfaceCardCandidates_SIFT_AlreadyCrossCheckedIsSilent proves SIFT
// never re-proposes itself once a material has its own cross_check — the
// same "surface_card never re-fires on the same material" invariant CRAAP
// already has. Without the crossCheckedBy exclusion, this material would
// propose a fresh SIFT candidate on every single turn forever.
func TestSurfaceCardCandidates_SIFT_AlreadyCrossCheckedIsSilent(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
		Nodes:     []GraphNodeView{{ID: "cc1", Type: "cross_check", Author: "student"}},
		Edges: []GraphEdgeView{
			{FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "e1", Type: "evaluated-as"},
			{FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "cc1", Type: "cross-checked-by"},
		},
	}
	// Slice 7: the evaluated-as edge with no claim node also satisfies the
	// project-scoped Toulmin surface rule, so this asserts SIFT specifically
	// does not re-propose, not that the candidate list is empty overall.
	if got := SurfaceCardCandidates(g); hasCard(got, siftCardID) {
		t.Fatalf("already cross-checked material must not re-propose SIFT, got %+v", got)
	}
}

// TestSurfaceCardCandidates_SIFT_LateralInstrumentIsSilent is the treadmill
// guard (agent-spec): m2 is the independent source some other cross_check
// cited (e.g. the NASA page the student found while checking a suspicious
// blog post). m2 has since been independently CRAAP-evaluated with no
// cross-check of its own — by the bare "evaluated-as, no cross-check" rule
// alone it would qualify for SIFT. But proposing SIFT on the lateral
// instrument itself would let every added lateral source manufacture its own
// next SIFT chore, forever. Without this exclusion, this test fails.
func TestSurfaceCardCandidates_SIFT_LateralInstrumentIsSilent(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m2", Kind: "article"}},
		Nodes:     []GraphNodeView{{ID: "cc1", Type: "cross_check", Author: "student"}},
		Edges: []GraphEdgeView{
			{FromKind: "material", FromID: "m2", ToKind: "graph_node", ToID: "e2", Type: "evaluated-as"},
			{FromKind: "graph_node", FromID: "cc1", ToKind: "material", ToID: "m2", Type: "cites"},
		},
	}
	// Slice 7: the evaluated-as edge with no claim node also satisfies the
	// project-scoped Toulmin surface rule, so this asserts SIFT specifically
	// does not fire on the lateral instrument, not that the candidate list
	// is empty overall.
	if got := SurfaceCardCandidates(g); hasCard(got, siftCardID) {
		t.Fatalf("lateral instrument must never get its own SIFT proposed, got %+v", got)
	}
}

// TestSurfaceCardCandidates_DraftMaterialIsSilent covers target_type
// "material.source": the student's own draft is never proposed for CRAAP.
func TestSurfaceCardCandidates_DraftMaterialIsSilent(t *testing.T) {
	g := GraphView{Materials: []MaterialView{{ID: "d1", Kind: "draft"}}}
	if got := SurfaceCardCandidates(g); len(got) != 0 {
		t.Fatalf("draft material must be silent, got %d", len(got))
	}
}

// TestSurfaceCardCandidates_ActiveCardSuppressesAllSurfaceCard is the
// whole-branch-review CRITICAL 1 fix: a card_instance that is already
// "active" (open, being filled by the student) must suppress EVERY
// surface_card candidate project-wide, not just one on its own material. m2
// is a wholly different, never-touched article — by the bare per-material
// rule alone it would fire a fresh CRAAP proposal. Before this fix, that
// candidate would win decide-one over any post_intervention nudge and
// RunAgentStep would apply it unconditionally, clobbering the open card's
// unsaved in-progress answers (which live only in client state until
// submit) and leaving its card_instance a permanently "active" zombie.
func TestSurfaceCardCandidates_ActiveCardSuppressesAllSurfaceCard(t *testing.T) {
	g := GraphView{
		CardInstances: []CardInstanceView{
			{ID: "ci1", CardID: siftCardID, Status: "active"},
		},
		Materials: []MaterialView{
			{ID: "m2", Kind: "article"}, // wholly unevaluated — would otherwise fire CRAAP
		},
	}
	if got := SurfaceCardCandidates(g); len(got) != 0 {
		t.Fatalf("an active card must suppress every surface_card candidate, got %+v", got)
	}
}

// TestSurfaceCardCandidates_ProposedCardSuppressesAllSurfaceCard covers the
// other in-flight status: a card that has been offered but not yet opened
// ("proposed") must suppress just as hard as "active" — the student hasn't
// decided whether to open it yet, and a second surface_card in the meantime
// would let the loop's decide-one silently swap in a different card before
// she ever gets to respond to the first offer.
func TestSurfaceCardCandidates_ProposedCardSuppressesAllSurfaceCard(t *testing.T) {
	g := GraphView{
		CardInstances: []CardInstanceView{
			{ID: "ci1", CardID: craapCardID, Status: "proposed"},
		},
		Materials: []MaterialView{
			{ID: "m2", Kind: "article"},
		},
	}
	if got := SurfaceCardCandidates(g); len(got) != 0 {
		t.Fatalf("a proposed card must suppress every surface_card candidate, got %+v", got)
	}
}

// TestSurfaceCardCandidates_ActiveCardClosesTreadmillWindowToo is IMPORTANT
// 2's verification: the lateralInstrument exclusion (the treadmill guard)
// only recognizes a lateral source once its cross_check node exists — while
// SIFT is in flight, the guard alone is blind to it. This test reconstructs
// exactly that blind state (m2 is CRAAP-evaluated, has no cross-check of its
// own, and no "cites" edge from any cross_check node marks it as a lateral
// instrument yet) WITH an active SIFT card_instance also present, and proves
// the blanket suppression above closes the window anyway: SurfaceCardCandidates
// must still be silent, because no surface_card of any kind may fire while
// any card is in flight, regardless of which material it would target.
func TestSurfaceCardCandidates_ActiveCardClosesTreadmillWindowToo(t *testing.T) {
	g := GraphView{
		CardInstances: []CardInstanceView{
			{ID: "ci1", CardID: siftCardID, Status: "active"},
		},
		Materials: []MaterialView{
			{ID: "m2", Kind: "article"},
		},
		Edges: []GraphEdgeView{
			// m2 was CRAAP-evaluated and has no cross-check yet, and — the
			// treadmill guard's blind spot — no "cites" edge from a
			// cross_check exists to mark it as a lateral instrument, because
			// the in-flight SIFT card hasn't minted its cross_check node yet.
			{FromKind: "material", FromID: "m2", ToKind: "graph_node", ToID: "e2", Type: "evaluated-as"},
		},
	}
	if got := SurfaceCardCandidates(g); len(got) != 0 {
		t.Fatalf("blanket suppression must close the treadmill window even though the per-material guard alone cannot see it, got %+v", got)
	}
}

// hasCard reports whether cands contains a surface_card candidate naming
// cardID, regardless of anchor.
func hasCard(cands []Candidate, cardID string) bool {
	for _, c := range cands {
		if c.Verb == "surface_card" && c.CardID == cardID {
			return true
		}
	}
	return false
}

// TestSurfaceToulminWhenEvaluatedNoClaim covers Slice 7's Toulmin surface
// rule: once any source has been evaluated (an "evaluated-as" edge exists),
// there is material to argue from, and — with no claim node yet — no
// argument has been started, so the argument-builder card should surface.
func TestSurfaceToulminWhenEvaluatedNoClaim(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
		Nodes:     []GraphNodeView{{ID: "q1", Type: "source_quality"}},
		Edges: []GraphEdgeView{
			{Type: "evaluated-as", FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "q1"},
		},
	}
	cands := SurfaceCardCandidates(g)
	if !hasCard(cands, "toulmin") {
		t.Fatalf("expected toulmin surfaced when a source is evaluated and no claim exists; got %v", cands)
	}
}

// TestNoToulminOnceClaimExists proves the rule stops firing once the
// student has begun an argument (a claim node exists), even though the
// evaluated-as edge that unlocked it is still present.
func TestNoToulminOnceClaimExists(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
		Nodes: []GraphNodeView{
			{ID: "q1", Type: "source_quality"},
			{ID: "c1", Type: "claim"},
		},
		Edges: []GraphEdgeView{
			{Type: "evaluated-as", FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "q1"},
		},
	}
	if hasCard(SurfaceCardCandidates(g), "toulmin") {
		t.Fatalf("toulmin must not surface once a claim node exists")
	}
}

// evaluatedProjectGraph is the minimal graph state the project-scoped surface
// rules key off: one article with a finished CRAAP evaluation (an
// "evaluated-as" edge) and nothing else. Extra nodes/card_instances are layered
// on per test.
func evaluatedProjectGraph() GraphView {
	return GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
		Nodes:     []GraphNodeView{{ID: "q1", Type: "source_quality"}},
		Edges: []GraphEdgeView{
			{Type: "evaluated-as", FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "q1"},
			// already cross-checked, so the per-material SIFT rule stays quiet
			// and these tests are about the project-scoped branch alone.
			{Type: "cross-checked-by", FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "cc1"},
		},
	}
}

// TestSurfacePerspectiveMatrixWhenFewerThanTwoPerspectives is whole-branch
// review CRITICAL 2: before this, perspective-matrix was dead config — nothing
// in the system could mint a card_instance for it, so nothing could ever
// produce a `perspective` graph node, so writing-project.json's
// evaluate_perspectives gate (node_count_at_least{perspective,2}) was
// permanently unsatisfiable and evaluate_sources — which `requires` it — was
// walled behind it.
func TestSurfacePerspectiveMatrixWhenFewerThanTwoPerspectives(t *testing.T) {
	g := evaluatedProjectGraph()
	if !hasCard(SurfaceCardCandidates(g), perspectiveMatrixCardID) {
		t.Fatal("expected perspective-matrix surfaced: a source is evaluated and the project has zero perspectives")
	}

	// One perspective is still short of the gate's n=2 — keep offering.
	g.Nodes = append(g.Nodes, GraphNodeView{ID: "p1", Type: "perspective", Author: "student"})
	if !hasCard(SurfaceCardCandidates(g), perspectiveMatrixCardID) {
		t.Fatal("expected perspective-matrix still surfaced at one perspective node (gate needs two)")
	}
}

// TestNoPerspectiveMatrixOnBareProject: firing on a project with nothing in it
// would be an ambush (铁律 2). The branch uses the same anyEvaluated moment the
// Toulmin branch does — there must be real substance in the project first.
func TestNoPerspectiveMatrixOnBareProject(t *testing.T) {
	if hasCard(SurfaceCardCandidates(GraphView{}), perspectiveMatrixCardID) {
		t.Fatal("perspective-matrix must not surface on a bare empty project")
	}
	// A pasted-but-unevaluated article is still not the moment: CRAAP owns it.
	g := GraphView{Materials: []MaterialView{{ID: "m1", Kind: "article"}}}
	if hasCard(SurfaceCardCandidates(g), perspectiveMatrixCardID) {
		t.Fatal("perspective-matrix must not surface before any source is evaluated")
	}
}

// TestNoPerspectiveMatrixOnceTwoPerspectivesExist: the gate is satisfied, so
// the offer has done its job and stops.
func TestNoPerspectiveMatrixOnceTwoPerspectivesExist(t *testing.T) {
	g := evaluatedProjectGraph()
	g.Nodes = append(g.Nodes,
		GraphNodeView{ID: "p1", Type: "perspective", Author: "student"},
		GraphNodeView{ID: "p2", Type: "perspective", Author: "student"},
	)
	if hasCard(SurfaceCardCandidates(g), perspectiveMatrixCardID) {
		t.Fatal("perspective-matrix must not re-surface once two perspective nodes exist")
	}
}

// TestNoPerspectiveMatrixWhileInFlight: the function-wide in-flight guard.
func TestNoPerspectiveMatrixWhileInFlight(t *testing.T) {
	for _, status := range []string{"proposed", "active"} {
		g := evaluatedProjectGraph()
		g.CardInstances = []CardInstanceView{{ID: "ci1", CardID: perspectiveMatrixCardID, Status: status}}
		if got := SurfaceCardCandidates(g); len(got) != 0 {
			t.Fatalf("status %q: nothing may surface while a card is in flight, got %+v", status, got)
		}
	}
}

// TestNoPerspectiveMatrixOnceDispositioned is the "an offer is never a wall"
// rule: a card the student already COMPLETED or SKIPPED must never be
// re-offered. A project-scoped SurfaceCard mints no card_instance->material
// edge (there is no material), so the per-material bookkeeping cannot see it —
// suppression has to read the card_instance list directly. A skipped card
// mints no perspective nodes, so without this the offer would return on every
// single turn, forever.
func TestNoPerspectiveMatrixOnceDispositioned(t *testing.T) {
	for _, status := range []string{"skipped", "completed"} {
		g := evaluatedProjectGraph()
		g.CardInstances = []CardInstanceView{{ID: "ci1", CardID: perspectiveMatrixCardID, Status: status}}
		if hasCard(SurfaceCardCandidates(g), perspectiveMatrixCardID) {
			t.Fatalf("status %q: perspective-matrix must never be re-offered once dispositioned", status)
		}
	}
}

// TestSurfaceCardCandidatesToulminNotReofferedAfterSkip is N3b's toulmin
// carry-forward fix: mirrors TestNoPerspectiveMatrixOnceDispositioned above —
// an offer is never a wall. Skipping (or completing) toulmin mints no claim
// node (see GraphEffects' "toulmin" case, card_effects.go — a claim node is
// only minted from a filled "claim" slot), so the bare
// anyEvaluated && !hasClaim predicate stays true forever and would re-offer
// on every single turn without this. Suppress on ANY toulmin card_instance,
// in ANY status, exactly like perspective-matrix.
func TestSurfaceCardCandidatesToulminNotReofferedAfterSkip(t *testing.T) {
	for _, status := range []string{"skipped", "completed"} {
		g := GraphView{
			Materials: []MaterialView{{ID: "m1", Kind: "article"}},
			Nodes:     []GraphNodeView{{ID: "q1", Type: "source_quality"}},
			Edges: []GraphEdgeView{
				{Type: "evaluated-as", FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "q1"},
			},
			CardInstances: []CardInstanceView{{ID: "ci1", CardID: toulminCardID, Status: status}},
		}
		if hasCard(SurfaceCardCandidates(g), toulminCardID) {
			t.Fatalf("status %q: toulmin must never be re-offered once dispositioned", status)
		}
	}
}

// TestPerspectiveMatrixOutranksToulmin pins the placement choice: when both
// project-scoped rules are eligible in the same turn, the perspective card
// comes first. The loop only ever acts on cands[0] (loop.go), so list order IS
// the priority here, and the skill's station chain puts evaluate_perspectives
// strictly upstream of the argument work.
func TestPerspectiveMatrixOutranksToulmin(t *testing.T) {
	g := evaluatedProjectGraph()
	got := SurfaceCardCandidates(g)
	if len(got) != 2 {
		t.Fatalf("candidates = %d, want 2 (perspective-matrix + toulmin), got %+v", len(got), got)
	}
	if got[0].CardID != perspectiveMatrixCardID || got[1].CardID != toulminCardID {
		t.Fatalf("order = [%s %s], want [perspective-matrix toulmin]", got[0].CardID, got[1].CardID)
	}
	if got[0].AnchorKind != "project" || got[0].AnchorID != "" {
		t.Fatalf("perspective-matrix must be project-scoped with no anchor id, got %+v", got[0])
	}
}

// TestSurfaceSearchPlanWhenPreregistrationExists is N3e's summon branch: once
// the student has written a search plan (a `preregistration` node exists),
// search-plan should surface, and it must never re-offer once a search-plan
// card_instance exists in any status — an offer is never a wall (铁律 2).
func TestSurfaceSearchPlanWhenPreregistrationExists(t *testing.T) {
	base := GraphView{Nodes: []GraphNodeView{{ID: "n1", Type: "preregistration", Author: "student"}}}

	if !hasCard(SurfaceCardCandidates(base), searchPlanCardID) {
		t.Fatalf("expected search-plan candidate when a preregistration node exists, got %+v", SurfaceCardCandidates(base))
	}

	// Suppressed once ANY search-plan instance exists, in ANY status:
	// proposed/active by the function-wide in-flight guard, completed/skipped
	// by this branch's own searchPlanSeen guard.
	for _, status := range []string{"proposed", "active", "completed", "skipped"} {
		g := base
		g.CardInstances = []CardInstanceView{{ID: "c1", CardID: searchPlanCardID, Status: status}}
		if hasCard(SurfaceCardCandidates(g), searchPlanCardID) {
			t.Fatalf("search-plan should be suppressed when an instance is %q", status)
		}
	}
}

// TestNoSearchPlanWithoutPreregistration: no preregistration node means no
// search plan has been written yet, so there is nothing to interrogate.
func TestNoSearchPlanWithoutPreregistration(t *testing.T) {
	if hasCard(SurfaceCardCandidates(GraphView{}), searchPlanCardID) {
		t.Fatal("search-plan must not fire without a preregistration node")
	}
}

// TestSearchPlanRetiresOnceAnySourceEvaluated: search-plan is a PRE-sourcing
// card — its `when` is 「在你按这份计划真正开始检索之前」(before she actually
// starts searching against the plan). Once she has evaluated her first
// source (an "evaluated-as" edge off a material node exists), the card must
// retire even though a preregistration node is present and no search-plan
// instance has ever been seen. This is what keeps it from later hijacking
// the S4 toulmin surface.
func TestSearchPlanRetiresOnceAnySourceEvaluated(t *testing.T) {
	g := GraphView{
		Nodes: []GraphNodeView{{ID: "n1", Type: "preregistration", Author: "student"}},
		Edges: []GraphEdgeView{
			{Type: "evaluated-as", FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "q1"},
		},
	}
	if hasCard(SurfaceCardCandidates(g), searchPlanCardID) {
		t.Fatal("search-plan must retire once any source has been evaluated, even with a preregistration node and no instance")
	}
}

// TestBuildArgumentGateDropsNoUnsupportedClaim pins the maintainer decision
// (铁律 2): an argument hole is surfaced (D5 post_intervention warn), not
// blocked. no_unsupported_claim must no longer be a machine gate item on
// build_argument — the warn channel in CandidateMoves is the only thing
// left standing.
func TestBuildArgumentGateDropsNoUnsupportedClaim(t *testing.T) {
	sk, ok := skills.ByID("writing-project")
	if !ok {
		t.Fatal("writing-project skill not embedded")
	}
	c, ok := sk.Contracts["build_argument"]
	if !ok {
		t.Fatal("build_argument contract missing")
	}
	for _, m := range c.Gate.Machine {
		if m.Kind == "no_unsupported_claim" {
			t.Fatal("no_unsupported_claim must no longer be a blocking machine gate item on build_argument (warn-not-block)")
		}
	}
}

// TestCandidateMovesStillWarnsUnsupportedClaim pins the warn we now rely on:
// one claim node, no supporting evidence -> D5 post_intervention candidate.
func TestCandidateMovesStillWarnsUnsupportedClaim(t *testing.T) {
	g := GraphView{Nodes: []GraphNodeView{{ID: "c1", Type: "claim"}}}
	got := CandidateMoves(g)
	found := false
	for _, cand := range got {
		if cand.Criterion == "D5" && cand.AnchorID == "c1" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a D5 warn candidate for the unsupported claim")
	}
}
