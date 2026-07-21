package agent

import (
	"strings"
	"testing"
)

// TestBuildCoachContextRendersRefeedForCardInstanceAnchor covers N3b Seam B:
// when the winning candidate is the refeed candidate (AnchorKind ==
// "card_instance"), BuildCoachContext renders the card's just-submitted
// contents instead of the graph-node/edges blocks — the coach's one question
// must be about what the student just wrote, not a stale graph neighborhood.
func TestBuildCoachContextRendersRefeedForCardInstanceAnchor(t *testing.T) {
	g := GraphView{Nodes: []GraphNodeView{{ID: "n1", Type: "claim", Author: "student"}}}
	c := Candidate{
		Verb:       "post_intervention",
		AnchorKind: "card_instance",
		AnchorID:   "ci-1",
		Criterion:  "D6",
		Reason:     "学生刚完成了一张工具卡",
		Level:      "I2",
	}
	refeed := &RefeedPayload{
		CardID:   "sift_craap",
		CardName: "SIFT×CRAAP 信息核查",
		Status:   "completed",
		Steps: []RefeedStep{
			{
				Title: "SIFT · 横向找更多来源",
				Answers: []RefeedAnswer{
					{Label: "Stop：你打算用这条信息说明什么？", Value: "证明中国让地球更可持续"},
					{Label: "Find better coverage：更权威的版本怎么说？", Value: "原始研究来自 NASA / Nature Sustainability"},
				},
			},
		},
	}

	ctxStr := BuildCoachContext(g, c, nil, refeed)

	if !strings.Contains(ctxStr, "# 学生刚完成的工具卡") {
		t.Fatalf("missing refeed heading:\n%s", ctxStr)
	}
	if !strings.Contains(ctxStr, "SIFT · 横向找更多来源") {
		t.Fatalf("missing step title:\n%s", ctxStr)
	}
	if !strings.Contains(ctxStr, "Stop：你打算用这条信息说明什么？") || !strings.Contains(ctxStr, "证明中国让地球更可持续") {
		t.Fatalf("missing first answer label/value:\n%s", ctxStr)
	}
	if !strings.Contains(ctxStr, "Find better coverage：更权威的版本怎么说？") || !strings.Contains(ctxStr, "原始研究来自 NASA / Nature Sustainability") {
		t.Fatalf("missing second answer label/value:\n%s", ctxStr)
	}
	if strings.Contains(ctxStr, "当前锚点节点") {
		t.Fatalf("must not render the graph-node heading for a card_instance anchor:\n%s", ctxStr)
	}
}

// TestBuildCoachContextUnchangedForGraphNodeAnchor is the regression pin: an
// ordinary graph_node candidate (every pre-N3b candidate) must render EXACTLY
// as before — byte-identical output whether or not a refeed pointer happens
// to be threaded through (it must be nil for every non-card_instance anchor
// in practice, but the function must not change behavior based on it either
// way for the wrong anchor kind).
func TestBuildCoachContextUnchangedForGraphNodeAnchor(t *testing.T) {
	g := GraphView{Nodes: []GraphNodeView{{ID: "n1", Type: "claim", Author: "student", Text: "中国有治理决心"}}}
	c := Candidate{Verb: "post_intervention", AnchorKind: "graph_node", AnchorID: "n1", Criterion: "D5", Reason: "裸主张", Level: "I2"}
	history := []ChatTurn{{Role: "user", Content: "它想证明中国在认真转型"}}

	before := BuildCoachContext(g, c, history, nil)
	after := BuildCoachContext(g, c, history, nil)
	if before != after {
		t.Fatalf("expected deterministic output, got:\n%s\nthen:\n%s", before, after)
	}
	if !strings.Contains(before, "# 当前锚点节点") {
		t.Fatalf("graph_node anchor must still render the graph-node heading:\n%s", before)
	}
	if strings.Contains(before, "学生刚完成的工具卡") {
		t.Fatalf("graph_node anchor must never render the refeed heading:\n%s", before)
	}
}
