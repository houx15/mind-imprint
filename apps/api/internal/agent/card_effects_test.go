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

func TestGraphEffects_CrossCheck(t *testing.T) {
	spec := cards.Spec{
		ID:           "sift",
		Params:       cards.Params{LateralDimension: "find"},
		GraphEffects: []cards.GraphEffect{{Kind: "cross_check"}},
	}
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "stop", Answer: "标题很夸张", Author: "student"},
		{ID: "a2", MaterialID: "mat-nasa", Dimension: "find", Answer: "NASA 只讲绿化面积", Author: "student"},
		{ID: "a3", MaterialID: "mat-blog", Dimension: "relation", Answer: "限定", Author: "student"},
		{ID: "a4", MaterialID: "mat-blog", Dimension: "trace_origin", Answer: "NASA Earth Observatory 2019", Author: "student"},
		{ID: "a5", MaterialID: "mat-blog", Dimension: "tier_after", Answer: "二手评论", Author: "student"},
		{ID: "a6", MaterialID: "mat-blog", Dimension: "revised_judgment", Answer: "一开始以为是造假，现在看是过度简化的二手转述", Author: "student"},
	}

	nodes, edges := GraphEffects(spec, "mat-blog", anchors)

	if len(nodes) != 1 || nodes[0].Type != "cross_check" {
		t.Fatalf("nodes = %+v, want exactly one cross_check node", nodes)
	}
	if nodes[0].Author != "student" {
		t.Fatalf("cross_check author = %q, want student — the relation is her judgment", nodes[0].Author)
	}
	if nodes[0].Body["relation"] != "限定" {
		t.Fatalf("relation = %v, want 限定 (the student's own choice)", nodes[0].Body["relation"])
	}
	if nodes[0].Body["trace_origin"] != "NASA Earth Observatory 2019" {
		t.Fatalf("trace_origin = %v", nodes[0].Body["trace_origin"])
	}
	if nodes[0].Body["tier_after"] != "二手评论" {
		t.Fatalf("tier_after = %v", nodes[0].Body["tier_after"])
	}
	// revised_judgment is the student's own written 修正后的判断 — added to the
	// SIFT card after Task 5's brief was drafted, so it must still be folded in.
	if nodes[0].Body["revised_judgment"] != "一开始以为是造假，现在看是过度简化的二手转述" {
		t.Fatalf("revised_judgment = %v", nodes[0].Body["revised_judgment"])
	}

	// Two edges: the checked source -> the cross_check -> the lateral source.
	if len(edges) != 2 {
		t.Fatalf("edges = %+v, want 2", edges)
	}
	if edges[0].Type != "cross-checked-by" || edges[0].FromID != "mat-blog" || edges[0].ToID != "$new:0" {
		t.Fatalf("edge[0] = %+v, want mat-blog --cross-checked-by--> $new:0", edges[0])
	}
	// The placeholder on the edge SOURCE is the one an implementer is likely to
	// get wrong; CompleteCard resolves both endpoints (card_lifecycle.go).
	if edges[1].Type != "cites" || edges[1].FromID != "$new:0" || edges[1].ToID != "mat-nasa" {
		t.Fatalf("edge[1] = %+v, want $new:0 --cites--> mat-nasa", edges[1])
	}
}

func TestGraphEffects_CrossCheck_NeverPromotesTheLateralSource(t *testing.T) {
	spec := cards.Spec{
		ID:           "sift",
		Params:       cards.Params{LateralDimension: "find"},
		GraphEffects: []cards.GraphEffect{{Kind: "cross_check"}},
	}
	anchors := []Anchor{
		{ID: "a1", MaterialID: "mat-blog", Dimension: "stop", Answer: "夸张", Author: "student"},
		{ID: "a2", MaterialID: "mat-nasa", Dimension: "find", Answer: "NASA 讲绿化", Author: "student"},
	}
	nodes, _ := GraphEffects(spec, "mat-blog", anchors)
	for _, n := range nodes {
		if n.Type == "evidence" {
			t.Fatal("a cross-check must never promote the lateral source to evidence — it has been evaluated by nobody")
		}
	}
}

func TestToulminGraphEffect(t *testing.T) {
	spec := toulminSpec(t) // helper from card_completion_test.go (same package)
	anchors := []Anchor{
		{Dimension: "claim", Answer: "中国的政策在净效果上让全球更可持续。", Author: "student"},
		{Dimension: "warrant", Answer: "从植被数据到可持续判断的推理链条。", Author: "student"},
		{Dimension: "warrant", MaterialID: "m_nasa", Author: "student"},
		{Dimension: "evidence", Answer: "NASA 观测显示中国主导了全球变绿增量。", Author: "student"},
		{Dimension: "evidence", MaterialID: "m_nasa", Author: "student"},
		{Dimension: "counter", Answer: "反方最强点：碳排放总量全球第一。", Author: "student"},
		{Dimension: "concession", Answer: "承认排放第一，但人均与历史累积远低。", Author: "student"},
		{Dimension: "concession", MaterialID: "m_bp", Author: "student"},
	}

	nodes, edges := GraphEffects(spec, "", anchors)

	// Five nodes, one per slot, all student-authored, body carries the text.
	if len(nodes) != 5 {
		t.Fatalf("nodes = %d, want 5", len(nodes))
	}
	idxByType := map[string]int{}
	for i, n := range nodes {
		if n.Author != "student" {
			t.Fatalf("node %s author = %q, want student", n.Type, n.Author)
		}
		if n.Body["text"] == "" || n.Body["text"] == nil {
			t.Fatalf("node %s has empty text", n.Type)
		}
		idxByType[n.Type] = i
	}
	for _, want := range []string{"claim", "warrant", "evidence", "counter", "concession"} {
		if _, ok := idxByType[want]; !ok {
			t.Fatalf("missing node type %q", want)
		}
	}

	// Exactly one supports edge, evidence -> claim, both placeholder endpoints.
	supports := 0
	for _, e := range edges {
		if e.Type != "supports" {
			continue
		}
		supports++
		if e.FromID != mintRef(idxByType["evidence"]) || e.ToID != mintRef(idxByType["claim"]) {
			t.Fatalf("supports edge = %s->%s, want evidence->claim placeholders", e.FromID, e.ToID)
		}
		if e.FromKind != "graph_node" || e.ToKind != "graph_node" {
			t.Fatalf("supports endpoints must be graph_node")
		}
	}
	if supports != 1 {
		t.Fatalf("supports edges = %d, want 1", supports)
	}

	// One cites edge per source anchor (warrant m_nasa, evidence m_nasa,
	// concession m_bp), each graph_node -> material with a real material id.
	cites := map[string]int{}
	for _, e := range edges {
		if e.Type != "cites" {
			continue
		}
		if e.FromKind != "graph_node" || e.ToKind != "material" {
			t.Fatalf("cites endpoints wrong: %s->%s", e.FromKind, e.ToKind)
		}
		cites[e.ToID]++
	}
	if cites["m_nasa"] != 2 || cites["m_bp"] != 1 {
		t.Fatalf("cites = %v, want m_nasa:2 m_bp:1", cites)
	}
}

func TestGraphEffectsPerspectivesMintsOneNodePerCompleteRow(t *testing.T) {
	spec := cards.Spec{
		Params:       cards.Params{Cols: []cards.Axis{{ID: "position"}, {ID: "grounds"}}},
		GraphEffects: []cards.GraphEffect{{Kind: "perspectives"}},
	}
	anchors := []Anchor{
		{Quote: "政府", Dimension: "position", Answer: "治理有决心"},
		{Quote: "政府", Dimension: "grounds", Answer: "植树与限排政策"},
		{Quote: "环保组织", Dimension: "position", Answer: "进展不足"}, // incomplete → skipped
	}
	nodes, edges := GraphEffects(spec, "", anchors)
	if len(edges) != 0 {
		t.Fatalf("perspectives mints no edges, got %d", len(edges))
	}
	if len(nodes) != 1 {
		t.Fatalf("want 1 node (only the complete row), got %d", len(nodes))
	}
	n := nodes[0]
	if n.Type != "perspective" || n.Author != "student" {
		t.Fatalf("node type/author = %s/%s", n.Type, n.Author)
	}
	if n.Body["text"] != "政府" {
		t.Fatalf("body text = %v, want 政府", n.Body["text"])
	}
	cells, ok := n.Body["cells"].(map[string]string)
	if !ok || cells["position"] != "治理有决心" || cells["grounds"] != "植树与限排政策" {
		t.Fatalf("body cells = %v", n.Body["cells"])
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
