package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestComposeAIUseSeed_EmptyRecordNoCall(t *testing.T) {
	cp := &countingProvider{inner: gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "should not be called"},
		{Kind: gateway.EventDone},
	})}
	u, n, _, err := ComposeAIUseSeed(context.Background(), cp, gateway.Resolved{Provider: "fake", Model: "m"}, AIUseRecordView{})
	if err != nil {
		t.Fatalf("empty record should not error: %v", err)
	}
	if cp.calls != 0 {
		t.Fatalf("empty record → zero provider calls, got %d", cp.calls)
	}
	if u != "" || n != "" {
		t.Fatalf("empty record → empty draft, got used=%q not=%q", u, n)
	}
}

func TestComposeAIUseSeed_SeedsFromRecord(t *testing.T) {
	cp := &countingProvider{inner: gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"used_for":"用 AI 澄清检索词、核对来源功能","not_used_for":"没有让 AI 代写正文或预测分数"}`},
		{Kind: gateway.EventDone},
	})}
	u, n, _, err := ComposeAIUseSeed(context.Background(), cp, gateway.Resolved{Provider: "fake", Model: "m"},
		AIUseRecordView{CoachTurns: 12, CardsProposed: 2, CardsAccepted: 1, LLMCallsByPurpose: map[string]int{"coach": 12}})
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if cp.calls != 1 {
		t.Fatalf("want exactly 1 provider call, got %d", cp.calls)
	}
	if u == "" || n == "" {
		t.Fatalf("expected a seeded draft, got used=%q not=%q", u, n)
	}
}
