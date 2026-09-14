package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestProposeChatReplyAccepts(t *testing.T) {
	prov := scriptedProvider("你为什么觉得它证明了你的观点？") // reuse coach_test.go's stub helper
	out, usage, err := ProposeChatReply(context.Background(), prov, gateway.Resolved{Provider: "deepseek", Model: "x"},
		[]ChatTurn{{Role: "user", Content: "这篇报道证明了我的观点"}}, "", "", "")
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

// TestBuildChatContextIncludesCompletedCardSummary covers N3b Seam B's chat
// variant at the pure-function level: a non-empty cardSummary must surface
// under its own heading, and cardSummary == "" must render byte-identical to
// what BuildChatContext produced before this task added the argument — the
// binding constraint that this shared-signature change must not perturb the
// no-completed-card path.
func TestBuildChatContextIncludesCompletedCardSummary(t *testing.T) {
	history := []ChatTurn{
		{Role: "user", Content: "这篇报道证明了我的观点"},
		{Role: "assistant", Content: "你为什么这么想？"},
	}
	threadSummary := "https://nasa.gov/x"
	flag := "学生贴进了一个来源链接，并把它当成论据——这是做「信源辨识（CRAAP）」的时机。"

	// Reconstructs the pre-Task-6 3-argument BuildChatContext's exact output
	// for these inputs, independent of the new implementation.
	var want strings.Builder
	want.WriteString("对话（从旧到新）：\n")
	want.WriteString("- 学生：这篇报道证明了我的观点\n")
	want.WriteString("- 你：你为什么这么想？\n")
	want.WriteString("\n此对话中已有的材料：" + threadSummary + "\n")
	want.WriteString("\n刚刚发生的思考时机：" + flag + "\n")
	want.WriteString("\n现在，简明回应学生最新的发言；需要解释时可用两三句，最多一个问题，允许不提问。")

	if got := BuildChatContext(history, threadSummary, "", flag); got != want.String() {
		t.Fatalf("cardSummary==\"\" must render byte-identical to the pre-Task-6 output:\nwant:\n%s\ngot:\n%s", want.String(), got)
	}

	cardSummary := "卡片：SIFT×CRAAP 信息核查\n## SIFT · 横向找更多来源\n- Stop：证明中国让地球更可持续"
	withCard := BuildChatContext(history, threadSummary, cardSummary, flag)
	if !strings.Contains(withCard, "\n此对话中已完成的工具卡："+cardSummary+"\n") {
		t.Fatalf("want the completed-card summary under its own heading, got:\n%s", withCard)
	}
}

func TestProposeChatReplyBannedPhrasingRejects(t *testing.T) {
	prov := scriptedProvider("你应该这样写：中国让地球更可持续。") // ghostwriting guard
	_, usage, err := ProposeChatReply(context.Background(), prov, gateway.Resolved{Provider: "deepseek", Model: "x"},
		[]ChatTurn{{Role: "user", Content: "帮我写"}}, "", "", "")
	if err == nil {
		t.Fatalf("banned phrasing should reject")
	}
	if usage.OutputTokens == 0 && usage.InputTokens == 0 {
		t.Fatalf("rejected reply must still be metered")
	}
}
