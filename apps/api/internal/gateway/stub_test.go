package gateway

import (
	"context"
	"testing"
)

func TestStubProviderReplaysScript(t *testing.T) {
	script := []StreamEvent{
		{Kind: EventTextDelta, TextDelta: "你好"},
		{Kind: EventTextDelta, TextDelta: "，我们一起核查。"},
		{Kind: EventToolUse, ToolUse: &StreamToolUse{ID: "tc_1", Name: "summon_card", ArgsJSON: `{"card_id":"sift_craap","reason":"r","nudge_text":"n"}`}},
		{Kind: EventUsage, Usage: &ChatUsage{InputTokens: 120, OutputTokens: 45}},
		{Kind: EventDone, StopReason: StopToolCall},
	}
	p := NewStubProvider(script)
	ch, err := p.Stream(context.Background(), Resolved{}, ChatRequest{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var got []StreamEvent
	for ev := range ch {
		got = append(got, ev)
	}
	if len(got) != len(script) {
		t.Fatalf("got %d events, want %d", len(got), len(script))
	}
	if got[0].TextDelta != "你好" {
		t.Fatalf("first delta = %q", got[0].TextDelta)
	}
	if got[2].ToolUse == nil || got[2].ToolUse.Name != "summon_card" {
		t.Fatalf("tool use missing")
	}
	if got[4].StopReason != StopToolCall {
		t.Fatalf("stop = %q", got[4].StopReason)
	}
}

func TestStubProviderRespectsCancel(t *testing.T) {
	p := NewStubProvider([]StreamEvent{
		{Kind: EventTextDelta, TextDelta: "a"},
		{Kind: EventTextDelta, TextDelta: "b"},
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch, err := p.Stream(ctx, Resolved{}, ChatRequest{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	// Channel must close without panicking even if no event is consumed.
	for range ch {
	}
}
