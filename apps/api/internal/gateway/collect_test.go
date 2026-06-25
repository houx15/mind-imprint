package gateway

import (
	"context"
	"testing"
)

func TestCollectAggregates(t *testing.T) {
	p := NewStubProvider([]StreamEvent{
		{Kind: EventTextDelta, TextDelta: "Hel"},
		{Kind: EventTextDelta, TextDelta: "lo"},
		{Kind: EventUsage, Usage: &ChatUsage{InputTokens: 3, OutputTokens: 5}},
		{Kind: EventDone, StopReason: StopStop},
	})
	res, err := Collect(context.Background(), p, Resolved{Provider: "x"}, ChatRequest{})
	if err != nil { t.Fatal(err) }
	if res.Text != "Hello" { t.Fatalf("text %q", res.Text) }
	if res.Usage.InputTokens != 3 || res.Usage.OutputTokens != 5 { t.Fatalf("usage %+v", res.Usage) }
	if res.StopReason != StopStop { t.Fatalf("stop %q", res.StopReason) }
}
