package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestComposeReadingTakeaway_SeedsOnlySynthesis(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"new_leads":["中国人均排放 vs 总量的口径"],"proposal_impact":"作为让步段的反例证据"}`},
		{Kind: gateway.EventDone},
	}
	prov := gateway.NewStubProvider(script)
	in := ReadingTakeawayInput{
		Brief: ReadingBrief{Reason: "验证反例", PhaseTag: "反例检验"},
		Record: TakeawayRecord{
			Findings:    []string{"中国碳排放总量全球第一"},
			Credibility: Credibility{Verdict: "strong", Why: "NASA 一手数据"},
			KeyQuotes:   []KeyQuote{{Quote: "China emits the most", Why: "直接反例"}},
		},
	}
	leads, impact, _, err := ComposeReadingTakeawaySuggestions(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, in)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if len(leads) == 0 || impact == "" {
		t.Fatalf("expected seeded synthesis, got leads=%v impact=%q", leads, impact)
	}
}

func TestComposeReadingTakeaway_EmptyRecordErrors(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{}`},
		{Kind: gateway.EventDone},
	}
	prov := gateway.NewStubProvider(script)
	_, _, _, err := ComposeReadingTakeawaySuggestions(context.Background(), prov,
		gateway.Resolved{Provider: "fake", Model: "m"}, ReadingTakeawayInput{})
	if err == nil {
		t.Fatalf("empty record (nothing to organize) should error, not fabricate")
	}
}
