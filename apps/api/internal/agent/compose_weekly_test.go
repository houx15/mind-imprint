package agent_test

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
)

func facts() agent.WeeklyFacts {
	return agent.WeeklyFacts{
		ClassName: "IBDP 一年级 · 研究组", ClassSize: 9, WeekLabel: "第 30 周（7.20–7.26）",
		DepthBuckets: map[string]int{"起步 L1": 2, "发展 L2": 3, "熟练 L3": 2, "优秀 L4": 1},
		RatedCount:   8, AutonomyMean: "2.6", AutonomyDelta: "+0.4",
		Cards: []agent.WeeklyFactCard{
			{UserID: "u1", Name: "周子墨", Kind: "watch", TagCode: "outsourced_judgment", TagLabel: "判断在外包", Evidence: "A 轴 0.5/5。提示词多为「帮我写一段」。"},
		},
	}
}

// composeStub is a gateway.Provider whose Stream emits a scripted weekly-prose
// JSON reply then closes — same scripted-provider shape as
// internal/api/assessment_test.go's assessStubProvider.
func composeStub(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 80, OutputTokens: 40}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestComposeWeeklyReturnsProse(t *testing.T) {
	reply := `{"comment":"这周整体在往会自己想挪。","depthNote":"熟练档多了一人。","autonomyNote":"自主均分小幅上行。","cards":[{"userId":"u1","lead":"连续让 AI 直接给结论","action":"线下问一句这些数据凭什么说明影响。"}]}`
	got, usage, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub", Model: "m", Tier: "flagship"}, facts())
	if err != nil {
		t.Fatalf("ComposeWeekly: %v", err)
	}
	if got.Comment == "" || len(got.Cards) != 1 || got.Cards[0].UserID != "u1" {
		t.Fatalf("prose = %+v", got)
	}
	if usage.InputTokens != 80 || usage.OutputTokens != 40 {
		t.Fatalf("usage = %+v, want the scripted usage event", usage)
	}
}

func TestComposeWeeklyRejectsUnknownUser(t *testing.T) {
	reply := `{"comment":"c","depthNote":"d","autonomyNote":"a","cards":[{"userId":"ghost","lead":"l","action":"x"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — the model must not invent a student")
	}
}

func TestComposeWeeklyRejectsMissingCard(t *testing.T) {
	reply := `{"comment":"c","depthNote":"d","autonomyNote":"a","cards":[]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — every fact-sheet card needs wording")
	}
}

func TestComposeWeeklyRejectsBareInternalCode(t *testing.T) {
	reply := `{"comment":"这个班的 D3 普遍偏弱。","depthNote":"d","autonomyNote":"a","cards":[{"userId":"u1","lead":"l","action":"x"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — 说人话: prose names the behaviour, not the code")
	}
}

func TestComposeWeeklyRejectsDuplicateUser(t *testing.T) {
	f := facts()
	f.Cards = append(f.Cards, agent.WeeklyFactCard{UserID: "u2", Name: "李想", Kind: "praise", TagCode: "grounded_claim", TagLabel: "论证扎实", Evidence: "D2 达到 L4。"})
	reply := `{"comment":"c","depthNote":"d","autonomyNote":"a","cards":[{"userId":"u1","lead":"l","action":"x"},{"userId":"u1","lead":"l2","action":"x2"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, f)
	if err == nil {
		t.Fatal("want rejection — a repeated student is not the same as covering every card")
	}
}

func TestComposeWeeklyRejectsOverlongComment(t *testing.T) {
	reply := `{"comment":"` + strings.Repeat("很", 400) + `","depthNote":"d","autonomyNote":"a","cards":[{"userId":"u1","lead":"l","action":"x"}]}`
	reply += `}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — comment exceeds its length cap")
	}
}

func TestComposeWeeklyRejectsNonJSON(t *testing.T) {
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub("不是 JSON"), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — model output must be JSON")
	}
}

func TestComposeWeeklyReturnsUsageOnRejection(t *testing.T) {
	reply := `{"comment":"c","depthNote":"d","autonomyNote":"a","cards":[]}`
	_, usage, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection")
	}
	if usage.InputTokens != 80 || usage.OutputTokens != 40 {
		t.Fatalf("usage = %+v, want the scripted usage even though output was rejected — the caller still meters the spend", usage)
	}
}

func TestWeeklyFactsPromptCarriesEvidenceVerbatim(t *testing.T) {
	p := agent.WeeklyFactsPrompt(facts())
	if !strings.Contains(p, "A 轴 0.5/5。提示词多为「帮我写一段」。") {
		t.Fatalf("prompt omits the card's verbatim evidence:\n%s", p)
	}
	if strings.Contains(p, "对话轮次") {
		t.Fatalf("prompt must not carry class-level usage counters:\n%s", p)
	}
}
