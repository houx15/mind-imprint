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
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "Hello" {
		t.Fatalf("text %q", res.Text)
	}
	if res.Usage.InputTokens != 3 || res.Usage.OutputTokens != 5 {
		t.Fatalf("usage %+v", res.Usage)
	}
	if res.StopReason != StopStop {
		t.Fatalf("stop %q", res.StopReason)
	}
}

func TestCollectPreservesHistoricalIncompleteTerminalSemantics(t *testing.T) {
	p := NewStubProvider([]StreamEvent{{Kind: EventTextDelta, TextDelta: "partial"}, {Kind: EventDone, StopReason: StopOther, Incomplete: true}})
	res, err := Collect(context.Background(), p, Resolved{}, ChatRequest{})
	if err != nil || res.Text != "partial" || res.StopReason != StopOther {
		t.Fatalf("Collect result = %#v err=%v", res, err)
	}
}

// TestCollectCarriesToolArguments — a tool call without its arguments is one
// the caller cannot execute, and nothing in the result says the arguments went
// missing. Both paths that build a ChatResult (this one and complete.go's)
// parse them, so a tool loop keeps working when the route changes channel.
func TestCollectCarriesToolArguments(t *testing.T) {
	p := NewStubProvider([]StreamEvent{
		{Kind: EventToolUse, ToolUse: &StreamToolUse{ID: "call_1", Name: "set_fields", ArgsJSON: `{"title":"气候变化议论文","tier":3}`}},
		{Kind: EventDone, StopReason: StopToolCall},
	})
	res, err := Collect(context.Background(), p, Resolved{Provider: "x"}, ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(res.ToolCalls))
	}
	tc := res.ToolCalls[0]
	if tc.ID != "call_1" || tc.Name != "set_fields" {
		t.Fatalf("tool call identity = %+v", tc)
	}
	if tc.Args["title"] != "气候变化议论文" {
		t.Fatalf("args = %+v, want the title carried through", tc.Args)
	}
}

// TestCollectKeepsAToolCallWithUnparseableArguments — a truncated argument
// string must not lose the call itself: the caller still sees which tool the
// model reached for, reports the missing argument and gets another round.
func TestCollectKeepsAToolCallWithUnparseableArguments(t *testing.T) {
	p := NewStubProvider([]StreamEvent{
		{Kind: EventToolUse, ToolUse: &StreamToolUse{ID: "call_1", Name: "set_fields", ArgsJSON: `{"title":"气候`}},
		{Kind: EventDone, StopReason: StopToolCall},
	})
	res, err := Collect(context.Background(), p, Resolved{Provider: "x"}, ChatRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ToolCalls) != 1 || res.ToolCalls[0].Name != "set_fields" {
		t.Fatalf("tool calls = %+v, want the call kept with empty args", res.ToolCalls)
	}
	if len(res.ToolCalls[0].Args) != 0 {
		t.Fatalf("args = %+v, want empty", res.ToolCalls[0].Args)
	}
}
