package agent

import "testing"

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
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
	c := got[0]
	if c.Verb != "surface_card" || c.AnchorID != "m1" || c.CardID != siftCardID {
		t.Fatalf("unexpected candidate: %+v", c)
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
	if got := SurfaceCardCandidates(g); len(got) != 0 {
		t.Fatalf("already cross-checked material must be silent, got %+v", got)
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
	if got := SurfaceCardCandidates(g); len(got) != 0 {
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
