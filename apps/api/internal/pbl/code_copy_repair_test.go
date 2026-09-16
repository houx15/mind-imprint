package pbl

import (
	"context"
	"encoding/json"
	"mindimprint/api/internal/gateway"
	"strings"
	"testing"
)

func TestCodeCompositionRepairsMissingCopyOnceAndMetersBothCalls(t *testing.T) {
	script := func(body string) []gateway.StreamEvent {
		b, _ := json.Marshal(map[string]string{"html": "<html><body>" + body + "</body></html>"})
		n := 3
		return []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: string(b)}, {Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 10, OutputTokens: 20, ReasoningTokens: &n}}, {Kind: gateway.EventDone}}
	}
	for _, success := range []bool{true, false} {
		second := "still missing"
		if success {
			second = "我的原文"
		}
		provider := gateway.NewSequenceStubProvider(script("missing"), script(second))
		got, usage, err := GenerateHeroCode(context.Background(), provider, gateway.Resolved{}, CreativeDirection{}, "test", "", "", &SiteContent{About: []string{"我的原文"}})
		if (err == nil) != success || provider.Calls != 2 {
			t.Fatalf("success=%v calls=%d err=%v", success, provider.Calls, err)
		}
		if !success && got != "" {
			t.Fatal("invalid page escaped")
		}
		if usage.InputTokens != 20 || usage.OutputTokens != 40 || usage.ReasoningTokens == nil || *usage.ReasoningTokens != 6 {
			t.Fatalf("lost usage: %+v", usage)
		}
		request := provider.Requests[1]
		if !strings.Contains(request.Messages[len(request.Messages)-1].Content, "我的原文") || request.ResponseFormat != gateway.ResponseFormatJSONObject {
			t.Fatal("repair lacks exact missing copy or output constraint")
		}
	}
}
