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

// TestSurfaceCardCandidates_AlreadyEvaluatedSourceIsSilent covers both ways
// a material can already be "spoken for": an existing card_instance->material
// edge (surfaced, maybe still in flight), and a material->evidence
// "evaluated-as" edge (already promoted). Either silences the predicate.
func TestSurfaceCardCandidates_AlreadyEvaluatedSourceIsSilent(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
		Edges: []GraphEdgeView{
			{FromKind: "card_instance", FromID: "ci1", ToKind: "material", ToID: "m1"},
		},
	}
	if got := SurfaceCardCandidates(g); len(got) != 0 {
		t.Fatalf("already-surfaced material must be silent, got %d", len(got))
	}

	g2 := GraphView{
		Materials: []MaterialView{{ID: "m2", Kind: "article"}},
		Edges: []GraphEdgeView{
			{FromKind: "material", FromID: "m2", ToKind: "graph_node", ToID: "e1", Type: "evaluated-as"},
		},
	}
	if got := SurfaceCardCandidates(g2); len(got) != 0 {
		t.Fatalf("already-evaluated material must be silent, got %d", len(got))
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
