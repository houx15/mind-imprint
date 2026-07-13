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
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 42, OutputTokens: 17}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

var testResolved = gateway.Resolved{Provider: "deepseek", Model: "deepseek-chat", Tier: "coach"}

func TestProposeIntervention_AnchoredQuestion(t *testing.T) {
	g, c := coachFixture()
	body := "这条主张现在还没有素材支撑——它的证据是什么？"
	prov := scriptedProvider(body)

	out, verdict, usage, err := ProposeIntervention(context.Background(), prov, testResolved, g, c, nil, constSim(0.99))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Fatal("expected non-zero usage from a successful call")
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

	out, verdict, _, err := ProposeIntervention(context.Background(), prov, testResolved, g, c, nil, constSim(0.99))
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

	_, _, usage, err := ProposeIntervention(context.Background(), prov, testResolved, g, c, nil, constSim(0.99))
	if err == nil {
		t.Fatal("expected the banned-phrase output to be rejected before persist")
	}
	// A rejected output still cost real money — the model call happened
	// before enforcement ran. Usage must survive the error return so the
	// caller can still meter it.
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Fatal("expected non-zero usage even when the output is rejected")
	}
}

func TestBuildCoachContext_IncludesChatHistory(t *testing.T) {
	g := GraphView{Nodes: []GraphNodeView{{ID: "n1", Type: "claim", Author: "student", Text: "中国有治理决心"}}}
	c := Candidate{Verb: "post_intervention", AnchorKind: "graph_node", AnchorID: "n1", Criterion: "D5", Reason: "裸主张", Level: "I2"}
	history := []ChatTurn{{Role: "user", Content: "它想证明中国在认真转型"}, {Role: "assistant", Content: "那要连到哪条主张？"}}
	ctxStr := BuildCoachContext(g, c, history)
	if !strings.Contains(ctxStr, "它想证明中国在认真转型") {
		t.Fatalf("coach context missing the student turn:\n%s", ctxStr)
	}
	// Empty history is transparent (no history section / no crash).
	_ = BuildCoachContext(g, c, nil)
}
