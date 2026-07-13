package agent

import (
	"testing"

	"mindimprint/api/internal/cards"
)

func TestGraphEffects_PromoteMintsEvidenceNodeAndEdge(t *testing.T) {
	spec := craapSpecFixture()
	spec.GraphEffects = []cards.GraphEffect{
		{Kind: "promote", From: "material", To: "evidence", With: "source_quality"},
	}

	nodes, edges := GraphEffects(spec, "material-1", completeAnchors())

	if len(nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(nodes))
	}
	n := nodes[0]
	if n.Type != "evidence" {
		t.Fatalf("node.Type = %q, want evidence", n.Type)
	}
	if n.Author != "student" {
		t.Fatalf("node.Author = %q, want student", n.Author)
	}
	sq, ok := n.Body["source_quality"]
	if !ok {
		t.Fatal("node.Body missing source_quality")
	}
	sqMap, ok := sq.(map[string]string)
	if !ok || len(sqMap) == 0 {
		t.Fatalf("source_quality = %#v, want non-empty per-dimension map", sq)
	}
	if sqMap["authority"] == "" {
		t.Fatal("source_quality[authority] empty")
	}
	// The risk_note anchor is the student's own written judgment (作用与风险),
	// required for completion (field_written_by) but not one of the
	// spec.Params.Tags dimensions — it must still land in source_quality so
	// the dossier projection (studio/projection.go riskNote) can read it.
	const wantRiskNote = "仍需留意样本口径是否一致"
	if sqMap["risk_note"] != wantRiskNote {
		t.Fatalf("source_quality[risk_note] = %q, want %q (the student's own risk_note anchor)", sqMap["risk_note"], wantRiskNote)
	}

	if len(edges) != 1 {
		t.Fatalf("edges = %d, want 1", len(edges))
	}
	e := edges[0]
	if e.Type != "evaluated-as" {
		t.Fatalf("edge.Type = %q, want evaluated-as", e.Type)
	}
	if e.FromKind != "material" || e.FromID != "material-1" {
		t.Fatalf("edge.From = %s/%s, want material/material-1", e.FromKind, e.FromID)
	}
	if e.ToKind != "graph_node" || e.ToID == "" {
		t.Fatalf("edge.To = %s/%s, want graph_node/<placeholder>", e.ToKind, e.ToID)
	}
}

func TestGraphEffects_Idempotent(t *testing.T) {
	spec := craapSpecFixture()
	spec.GraphEffects = []cards.GraphEffect{
		{Kind: "promote", From: "material", To: "evidence", With: "source_quality"},
	}
	n1, e1 := GraphEffects(spec, "material-1", completeAnchors())
	n2, e2 := GraphEffects(spec, "material-1", completeAnchors())
	if len(n1) != len(n2) || len(e1) != len(e2) {
		t.Fatalf("effects not deterministic: (%d,%d) vs (%d,%d)", len(n1), len(e1), len(n2), len(e2))
	}
	if n1[0].Type != n2[0].Type || e1[0].ToID != e2[0].ToID {
		t.Fatal("effects not idempotent across calls")
	}
}

func TestGraphEffects_EmptyRiskNoteAnswerOmittedFromSourceQuality(t *testing.T) {
	spec := craapSpecFixture()
	spec.GraphEffects = []cards.GraphEffect{
		{Kind: "promote", From: "material", To: "evidence", With: "source_quality"},
	}
	anchors := completeAnchors()
	for i := range anchors {
		if anchors[i].Dimension == "risk_note" {
			anchors[i].Answer = ""
		}
	}

	nodes, _ := GraphEffects(spec, "material-1", anchors)
	sqMap := nodes[0].Body["source_quality"].(map[string]string)
	if _, present := sqMap["risk_note"]; present {
		t.Fatalf("source_quality[risk_note] = %q, want key absent when the anchor's answer is empty", sqMap["risk_note"])
	}
}

func TestGraphEffects_NoPromoteEffectsAreEmpty(t *testing.T) {
	spec := craapSpecFixture() // no GraphEffects set on the fixture
	nodes, edges := GraphEffects(spec, "material-1", completeAnchors())
	if len(nodes) != 0 || len(edges) != 0 {
		t.Fatalf("nodes/edges = %d/%d, want 0/0 (spec has no promote effect)", len(nodes), len(edges))
	}
}

func TestConsolidationPayload_NonEmptyFramework(t *testing.T) {
	spec := craapSpecFixture()
	spec.Consolidation = "reveal_framework_after_completion"
	payload := ConsolidationPayload(spec)
	if len(payload) == 0 {
		t.Fatal("payload should be non-empty when spec.Consolidation is set")
	}
}

func TestConsolidationPayload_EmptyWhenNoConsolidation(t *testing.T) {
	spec := craapSpecFixture()
	spec.Consolidation = ""
	payload := ConsolidationPayload(spec)
	if len(payload) != 0 {
		t.Fatalf("payload = %#v, want empty when spec.Consolidation is unset", payload)
	}
}
