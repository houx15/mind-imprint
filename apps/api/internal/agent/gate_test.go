package agent

import (
	"testing"

	"mindimprint/api/internal/skills"
)

func TestEvalMachineItem_NodePresentAndCount(t *testing.T) {
	g := GraphView{Nodes: []GraphNodeView{
		{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"},
	}}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "node_present", Type: "perspective"}, g); !pass {
		t.Fatal("node_present should pass")
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "node_present", Type: "concession"}, g); pass {
		t.Fatal("node_present(concession) should fail")
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "node_count_at_least", Type: "perspective", N: 2}, g); !pass {
		t.Fatal("count>=2 should pass")
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "node_count_at_least", Type: "perspective", N: 3}, g); pass {
		t.Fatal("count>=3 should fail")
	}
}

func TestEvalMachineItem_ArgumentGraphPredicates(t *testing.T) {
	// claim c1 supported by two distinct evidence e1,e2 → healthy.
	// evidence e3 orphaned (no supports edge) → no_orphan_evidence fails.
	g := GraphView{
		Nodes: []GraphNodeView{
			{ID: "c1", Type: "claim"}, {ID: "e1", Type: "evidence"},
			{ID: "e2", Type: "evidence"}, {ID: "e3", Type: "evidence"},
		},
		Edges: []GraphEdgeView{
			{FromKind: "graph_node", FromID: "e1", ToKind: "graph_node", ToID: "c1", Type: "supports"},
			{FromKind: "graph_node", FromID: "e2", ToKind: "graph_node", ToID: "c1", Type: "supports"},
		},
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "no_orphan_evidence"}, g); pass {
		t.Fatal("e3 is orphaned → should fail")
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "no_unsupported_claim"}, g); !pass {
		t.Fatal("c1 has support → should pass")
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "no_single_sourced_claim"}, g); !pass {
		t.Fatal("c1 has two evidence → should pass")
	}

	// A claim with a single supporting evidence fails no_single_sourced_claim.
	single := GraphView{
		Nodes: []GraphNodeView{{ID: "c9", Type: "claim"}, {ID: "e9", Type: "evidence"}},
		Edges: []GraphEdgeView{{FromKind: "graph_node", FromID: "e9", ToKind: "graph_node", ToID: "c9", Type: "supports"}},
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "no_single_sourced_claim"}, single); pass {
		t.Fatal("single-sourced claim → should fail")
	}
}

func TestEvalMachineItem_EverySourceEvaluated(t *testing.T) {
	g := GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}, {ID: "m2", Kind: "article"}},
		Edges: []GraphEdgeView{
			{FromKind: "material", FromID: "m1", ToKind: "graph_node", ToID: "ev1", Type: "evaluated-as"},
		},
	}
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "every_source_evaluated"}, g); pass {
		t.Fatal("m2 not evaluated → should fail")
	}
	g.Edges = append(g.Edges, GraphEdgeView{FromKind: "material", FromID: "m2", ToKind: "graph_node", ToID: "ev2", Type: "evaluated-as"})
	if pass, _ := EvalMachineItemForTest(skills.MachineItem{Kind: "every_source_evaluated"}, g); !pass {
		t.Fatal("all sources evaluated → should pass")
	}
}
