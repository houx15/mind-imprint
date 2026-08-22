package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestQuestionCardTurn_MidConversation(t *testing.T) {
	prov := stubProviderText(`{"narrate":"用你自己的话说说，你对这个题目的理解是？","suggestedObjective":"","done":false}`)
	out, usage, err := QuestionCardTurn(context.Background(), prov, gateway.Resolved{Provider: "stub"}, QuestionCardInput{
		Title: "中国是否让地球更可持续", Objective: "",
		History: []ChatTurn{{Role: "user", Content: "帮我想想这题"}},
	})
	if err != nil {
		t.Fatalf("QuestionCardTurn err = %v", err)
	}
	if out.Done {
		t.Fatal("mid-conversation reply must not be done")
	}
	if out.Narrate == "" {
		t.Fatal("narrate required")
	}
	if usage.OutputTokens == 0 {
		t.Error("usage should be reported for metering")
	}
}

func TestQuestionCardTurn_FinalFillsObjective(t *testing.T) {
	prov := stubProviderText(`{"narrate":"很好，这就是你的研究问题。","suggestedObjective":"太阳能装机增长在多大程度上降低了中国单位GDP碳排放","done":true}`)
	out, _, err := QuestionCardTurn(context.Background(), prov, gateway.Resolved{Provider: "stub"}, QuestionCardInput{Title: "x"})
	if err != nil {
		t.Fatalf("final turn err = %v", err)
	}
	if !out.Done || !strings.Contains(out.SuggestedObjective, "太阳能") {
		t.Fatalf("out = %+v, want done + objective", out)
	}
}

func TestQuestionCardTurn_EmptyNarrateErrors(t *testing.T) {
	prov := stubProviderText(`{"narrate":"","suggestedObjective":"","done":false}`)
	if _, _, err := QuestionCardTurn(context.Background(), prov, gateway.Resolved{Provider: "stub"}, QuestionCardInput{}); err == nil {
		t.Fatal("expected error on empty narrate (caller degrades to fallback)")
	}
}

func TestQuestionCardTurn_ParsesFenced(t *testing.T) {
	prov := stubProviderText("```json\n{\"narrate\":\"你和这个话题有什么交集？\",\"suggestedObjective\":null,\"done\":false}\n```")
	out, _, err := QuestionCardTurn(context.Background(), prov, gateway.Resolved{Provider: "stub"}, QuestionCardInput{})
	if err != nil {
		t.Fatalf("fenced err = %v", err)
	}
	if out.Narrate == "" || out.Done {
		t.Fatalf("out = %+v", out)
	}
}

// TestQuestionCardTurn_AssistantHistoryRefedAsJSON guards the root cause of the
// "canned fallback repeats forever" bug: prior assistant turns are stored as
// PLAIN narrate prose, and feeding them back verbatim taught deepseek-v4-pro to
// abandon the JSON contract and answer in prose too → every turn from #2 on
// failed to parse → the caller leaked its canned opener. The fix rewraps each
// assistant turn as its JSON envelope so the model stays in-contract.
func TestQuestionCardTurn_AssistantHistoryRefedAsJSON(t *testing.T) {
	prov := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"narrate":"继续问一个问题？","suggestedObjective":"","done":false}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 12}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
	if _, _, err := QuestionCardTurn(context.Background(), prov, gateway.Resolved{Provider: "stub"}, QuestionCardInput{
		Title: "中国是否让地球更可持续",
		History: []ChatTurn{
			{Role: "user", Content: "我不太确定这个题目要研究什么"},
			{Role: "assistant", Content: "没关系，我们慢慢来。用你自己的话说说你的理解？"},
			{Role: "user", Content: "中国算好还是不好呢"},
		},
	}); err != nil {
		t.Fatalf("QuestionCardTurn err = %v", err)
	}
	var assistant *gateway.ChatMessage
	for i := range prov.LastRequest.Messages {
		if prov.LastRequest.Messages[i].Role == gateway.RoleAssistant {
			assistant = &prov.LastRequest.Messages[i]
			break
		}
	}
	if assistant == nil {
		t.Fatal("expected an assistant message in the replayed history")
	}
	// The assistant turn must be re-fed as a JSON object carrying its narrate,
	// NOT the bare prose — that is what keeps the model emitting JSON.
	var env struct {
		Narrate string `json:"narrate"`
	}
	if err := json.Unmarshal([]byte(assistant.Content), &env); err != nil {
		t.Fatalf("assistant history not JSON-wrapped: %q (%v)", assistant.Content, err)
	}
	if !strings.Contains(env.Narrate, "慢慢来") {
		t.Fatalf("JSON envelope lost the narrate: %q", assistant.Content)
	}
}
