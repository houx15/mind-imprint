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

func TestCheckGate_MachineClearNeverSolid(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	// evaluate_perspectives needs 2 perspectives (machine) + student items.
	g := GraphView{Nodes: []GraphNodeView{
		{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"},
	}}
	// no recorded non-machine items, not confirmed
	r := CheckGate(sk, "evaluate_perspectives", g, RecordedGate{})
	if r.Status != "machine_clear" {
		t.Fatalf("Status = %q, want machine_clear", r.Status)
	}
	if r.Solid {
		t.Fatal("DEC-3: CheckGate must never report Solid without a recorded confirmation")
	}
	// missing lists the owed student_written items
	if len(r.Missing) == 0 {
		t.Fatal("want student_written items reported as missing")
	}
}

func TestCheckGate_EmptyAndPartial(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	empty := CheckGate(sk, "evaluate_perspectives", GraphView{}, RecordedGate{})
	if empty.Status != "empty" {
		t.Fatalf("Status = %q, want empty", empty.Status)
	}
	partial := CheckGate(sk, "evaluate_perspectives", GraphView{
		Nodes: []GraphNodeView{{ID: "p1", Type: "perspective"}}, // only 1 of 2
	}, RecordedGate{})
	if partial.Status != "partial" {
		t.Fatalf("Status = %q, want partial", partial.Status)
	}
}

// TestCheckGate_EverySourceEvaluatedAttemptedMirrorsArticleOnlyScan covers
// the Important review finding: attemptedFor's every_source_evaluated case
// must mirror evalMachineItem's own scan, which only considers "article"
// materials (m.Kind != "article" { continue }). A material of kind "draft"
// (the student's own draft) must not count as an attempt on this predicate.
func TestCheckGate_EverySourceEvaluatedAttemptedMirrorsArticleOnlyScan(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	findItem := func(items []ItemResult, name string) ItemResult {
		for _, it := range items {
			if it.Name == name {
				return it
			}
		}
		t.Fatalf("item %q not found in report", name)
		return ItemResult{}
	}

	// Only a draft material (no article) → evalMachineItem's article-only
	// scan finds nothing to fail on, so Pass vacuously true; Attempted must
	// be false since no article was ever scanned.
	draftOnly := CheckGate(sk, "evaluate_sources", GraphView{
		Materials: []MaterialView{{ID: "d1", Kind: "draft"}},
	}, RecordedGate{})
	it := findItem(draftOnly.Items, "every_source_evaluated")
	if it.Attempted {
		t.Fatal("draft-only materials → Attempted should be false (no article scanned)")
	}

	// Empty materials → same vacuous-pass, not-attempted case.
	empty := CheckGate(sk, "evaluate_sources", GraphView{}, RecordedGate{})
	it = findItem(empty.Items, "every_source_evaluated")
	if it.Attempted {
		t.Fatal("no materials → Attempted should be false")
	}

	// An article material → genuine attempt, regardless of Pass/Fail.
	withArticle := CheckGate(sk, "evaluate_sources", GraphView{
		Materials: []MaterialView{{ID: "m1", Kind: "article"}},
	}, RecordedGate{})
	it = findItem(withArticle.Items, "every_source_evaluated")
	if !it.Attempted {
		t.Fatal("an article material present → Attempted should be true")
	}
}

func TestCheckGate_SolidOnlyFromRecordedConfirmation(t *testing.T) {
	sk, _ := skills.ByID("writing-project")
	g := GraphView{Nodes: []GraphNodeView{{ID: "p1", Type: "perspective"}, {ID: "p2", Type: "perspective"}}}
	r := CheckGate(sk, "evaluate_perspectives", g, RecordedGate{
		Confirmed: true,
		Items:     map[string]string{"recon_logged": "solid", "sources_per_perspective": "solid"},
	})
	if !r.Solid {
		t.Fatal("recorded confirmation → Solid true")
	}
	if r.Status == "solid" {
		t.Fatal("DEC-3: Status enum never carries solid; Solid is a separate recorded flag")
	}
}
