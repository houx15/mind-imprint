package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func stubMomentProvider(id string) *countingProvider {
	return &countingProvider{inner: gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: id},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 2}},
		{Kind: gateway.EventDone},
	})}
}

var allEligible = []Moment{MomentFactOpinion, MomentOverclaim, MomentOneSided}

func TestProposeCoachCard_SummonOnMoment(t *testing.T) {
	prov := stubMomentProvider("fact_opinion")
	p, usage, err := ProposeCoachCard(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"},
		"我觉得中国显然让地球更可持续了，这是显然的事实", allEligible)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if p == nil || p.CardID != "fact-opinion-value" {
		t.Fatalf("want fact-opinion-value proposal, got %+v", p)
	}
	if p.NudgeText == "" {
		t.Fatalf("proposal should carry a student-facing nudge")
	}
	if usage.InputTokens == 0 {
		t.Fatalf("a real classify call must report usage to meter")
	}
}

func TestProposeCoachCard_NoneNoProposal(t *testing.T) {
	prov := stubMomentProvider("none")
	p, _, err := ProposeCoachCard(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"},
		"我在读一篇关于碳排放的文章，做点笔记", allEligible)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if p != nil {
		t.Fatalf("none moment must yield no proposal, got %+v", p)
	}
}

func TestProposeCoachCard_ShortTextNoCall(t *testing.T) {
	prov := stubMomentProvider("fact_opinion")
	p, _, err := ProposeCoachCard(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, "嗯", allEligible)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if p != nil || prov.calls != 0 {
		t.Fatalf("short text must not classify (no spend): proposal=%+v calls=%d", p, prov.calls)
	}
}

func TestProposeCoachCard_EmptyEligibleNoCall(t *testing.T) {
	prov := stubMomentProvider("fact_opinion")
	p, _, err := ProposeCoachCard(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"},
		"我觉得中国显然让地球更可持续了，这是显然的事实", nil)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if p != nil || prov.calls != 0 {
		t.Fatalf("empty eligible must not classify (no spend): proposal=%+v calls=%d", p, prov.calls)
	}
}
