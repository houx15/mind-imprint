package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func placementProvider(reply string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 12}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestSuggestBestQuestion_PicksAValidLead(t *testing.T) {
	in := SuggestPlacementInput{
		Title:     "Urban tree canopy and heat",
		Questions: []PlacementQuestion{{ID: "q1", Text: "树冠能降温吗"}, {ID: "q2", Text: "政策成本"}},
	}
	out, _, err := SuggestBestQuestion(context.Background(), placementProvider(`{"leadId":"q1","reason":"直接回答降温机制"}`),
		gateway.Resolved{Provider: "deepseek", Model: "x"}, in)
	if err != nil {
		t.Fatal(err)
	}
	if out.LeadID != "q1" || out.Reason == "" {
		t.Fatalf("out = %+v, want leadId q1 + reason", out)
	}
}

func TestSuggestBestQuestion_DropsUnknownLeadToEmpty(t *testing.T) {
	in := SuggestPlacementInput{Title: "X", Questions: []PlacementQuestion{{ID: "q1", Text: "a"}}}
	// Model hallucinates an id not in the set → must fall back to "" (未归类).
	out, _, err := SuggestBestQuestion(context.Background(), placementProvider(`{"leadId":"nope","reason":"r"}`),
		gateway.Resolved{Provider: "deepseek", Model: "x"}, in)
	if err != nil {
		t.Fatal(err)
	}
	if out.LeadID != "" {
		t.Fatalf("leadId = %q, want empty (unknown id dropped)", out.LeadID)
	}
}

func TestSuggestBestQuestion_NullIsNone(t *testing.T) {
	in := SuggestPlacementInput{Title: "X", Questions: []PlacementQuestion{{ID: "q1", Text: "a"}}}
	out, _, err := SuggestBestQuestion(context.Background(), placementProvider(`{"leadId":null,"reason":"都不太贴"}`),
		gateway.Resolved{Provider: "deepseek", Model: "x"}, in)
	if err != nil {
		t.Fatal(err)
	}
	if out.LeadID != "" {
		t.Fatalf("leadId = %q, want empty", out.LeadID)
	}
}
