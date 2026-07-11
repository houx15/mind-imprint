package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// constSim is a fixed-value Similarity stub — no real embedding call.
type constSim float64

func (c constSim) Cosine(a, b string) float64 { return float64(c) }

func coachFixture() (GraphView, Candidate) {
	g := GraphView{
		Nodes: []GraphNodeView{
			{ID: "n1", Type: "claim", Author: "student", Text: "中国的经济转型正在让地球更可持续"},
		},
	}
	c := Candidate{
		Verb:       "post_intervention",
		AnchorKind: "graph_node",
		AnchorID:   "n1",
		Criterion:  "D5",
		Reason:     "claim has no supporting evidence",
		Level:      "I2",
	}
	return g, c
}

func scriptedProvider(text string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

var testResolved = gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "coach"}

func TestProposeIntervention_AnchoredQuestion(t *testing.T) {
	g, c := coachFixture()
	body := "这条主张现在还没有素材支撑——它的证据是什么？"
	prov := scriptedProvider(body)

	out, verdict, err := ProposeIntervention(context.Background(), prov, testResolved, g, c, constSim(0.99))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Type != "question" {
		t.Fatalf("want type=question, got %q", out.Type)
	}
	if out.Anchor.Kind != "graph_node" || out.Anchor.ID != "n1" {
		t.Fatalf("unexpected anchor: %+v", out.Anchor)
	}
	if out.Criterion != "D5" {
		t.Fatalf("want criterion=D5, got %q", out.Criterion)
	}
	if out.Body != body {
		t.Fatalf("want body %q, got %q", body, out.Body)
	}
	if verdict == "" {
		t.Fatal("expected a non-empty verdict")
	}
}

func TestProposeIntervention_DeclarativeEchoIsIntercepted(t *testing.T) {
	g, c := coachFixture()
	// A real Chinese declarative echo ending in the full-width "。" (no "？") —
	// enforcement's isDeclarative flags it, and constSim(0.99) puts it well above
	// the echo threshold against the anchored node's own text.
	prov := scriptedProvider("中国的经济转型正在让地球更可持续。")

	out, verdict, err := ProposeIntervention(context.Background(), prov, testResolved, g, c, constSim(0.99))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != "intercept" {
		t.Fatalf("want verdict=intercept, got %q", verdict)
	}
	if !strings.HasSuffix(out.Body, "？") {
		t.Fatalf("want rewritten body to end in a full-width question mark, got %q", out.Body)
	}
}

func TestProposeIntervention_BannedPhraseIsRejected(t *testing.T) {
	g, c := coachFixture()
	prov := scriptedProvider("你有没有考虑过其他角度？")

	_, _, err := ProposeIntervention(context.Background(), prov, testResolved, g, c, constSim(0.99))
	if err == nil {
		t.Fatal("expected the banned-phrase output to be rejected before persist")
	}
}
