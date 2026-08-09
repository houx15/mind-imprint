package agent

import (
	"context"
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
