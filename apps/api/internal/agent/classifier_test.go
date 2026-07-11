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
