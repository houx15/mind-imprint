package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

// dig_query_test.go — Task A8: 印记 turns the student's (possibly Chinese,
// possibly long-sentence) research question into a short English keyword
// query before OpenAlex ever sees it. Mirrors
// exploration_guide_test.go's stubbed-provider pattern.

func TestComposeDigQuery_ReturnsTrimmedQuery(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "China greening carbon accounting attribution"},
		{Kind: gateway.EventDone},
	}
	prov := gateway.NewStubProvider(script)
	got, _, err := ComposeDigQuery(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, "中国单独 vs 中印合计的口径差异")
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if got != "China greening carbon accounting attribution" {
		t.Fatalf("got %q, want the stub's text trimmed", got)
	}
}

func TestComposeDigQuery_StripsFences(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "```\nChina carbon emissions per capita vs total\n```"},
		{Kind: gateway.EventDone},
	}
	prov := gateway.NewStubProvider(script)
	got, _, err := ComposeDigQuery(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, "中国人均排放 vs 总量的口径")
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if got != "China carbon emissions per capita vs total" {
		t.Fatalf("got %q, want fences stripped and trimmed", got)
	}
}

func TestComposeDigQuery_EmptyResultErrors(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "   "},
		{Kind: gateway.EventDone},
	}
	prov := gateway.NewStubProvider(script)
	got, _, err := ComposeDigQuery(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, "某个问题")
	if err == nil {
		t.Fatalf("want error on empty refined query (caller falls back to raw text), got %q", got)
	}
}
