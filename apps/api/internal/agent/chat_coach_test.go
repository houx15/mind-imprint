package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestProposeChatReplyAccepts(t *testing.T) {
	prov := scriptedProvider("你为什么觉得它证明了你的观点？") // reuse coach_test.go's stub helper
	out, usage, err := ProposeChatReply(context.Background(), prov, gateway.Resolved{Provider: "deepseek", Model: "x"},
		[]ChatTurn{{Role: "user", Content: "这篇报道证明了我的观点"}}, "", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out.Type != "reply" || out.Body == "" {
		t.Fatalf("want reply with body, got %+v", out)
	}
	if usage.OutputTokens == 0 && usage.InputTokens == 0 {
		t.Fatalf("usage not populated")
	}
}

func TestProposeChatReplyBannedPhrasingRejects(t *testing.T) {
	prov := scriptedProvider("你应该这样写：中国让地球更可持续。") // ghostwriting guard
	_, usage, err := ProposeChatReply(context.Background(), prov, gateway.Resolved{Provider: "deepseek", Model: "x"},
		[]ChatTurn{{Role: "user", Content: "帮我写"}}, "", "")
	if err == nil {
		t.Fatalf("banned phrasing should reject")
	}
	if usage.OutputTokens == 0 && usage.InputTokens == 0 {
		t.Fatalf("rejected reply must still be metered")
	}
}
