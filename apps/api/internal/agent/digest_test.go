package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestComposeDigestMerge_EmptyTurnsNoCall(t *testing.T) {
	prov := &countingProvider{inner: gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "should not be called"},
		{Kind: gateway.EventDone},
	})}
	prose, _, err := ComposeDigestMerge(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, "prior", nil)
	if err != nil {
		t.Fatalf("empty turns should not error: %v", err)
	}
	if prov.calls != 0 {
		t.Fatalf("want zero provider calls on empty turns, got %d", prov.calls)
	}
	if prose != "" {
		t.Fatalf("want empty prose on empty turns, got %q", prose)
	}
}

func TestComposeDigestMerge_MergesTurns(t *testing.T) {
	prov := &countingProvider{inner: gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "学生确认 NASA 的 one-third 是 China+India combined，不是 China alone。"},
		{Kind: gateway.EventDone},
	})}
	prose, _, err := ComposeDigestMerge(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"},
		"", []DigestTurn{
			{Role: "user", Content: "公众号说地球变绿是真的吗？"},
			{Role: "assistant", Content: "先追公众号背后的上游来源。"},
		})
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if prov.calls != 1 {
		t.Fatalf("want exactly 1 provider call, got %d", prov.calls)
	}
	if prose == "" {
		t.Fatalf("want non-empty digest prose")
	}
}
