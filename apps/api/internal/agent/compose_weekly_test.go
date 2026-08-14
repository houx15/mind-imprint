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
		Cards: []agent.WeeklyFactCard{
			{UserID: "u1", Name: "周子墨", Kind: "watch", TagCode: "stuck_no_output", TagLabel: "有对话没产出", Evidence: "本周 12 轮对话，但没有完成能力报告。"},
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
	reply := `{"comment":"这周整体在往会自己想挪。","cards":[{"userId":"u1","lead":"连续让 AI 直接给结论","action":"线下问一句这些数据凭什么说明影响。"}]}`
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
	reply := `{"comment":"c","cards":[{"userId":"ghost","lead":"l","action":"x"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — the model must not invent a student")
	}
}

func TestComposeWeeklyRejectsMissingCard(t *testing.T) {
	reply := `{"comment":"c","cards":[]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — every fact-sheet card needs wording")
	}
}

// TestComposeWeeklyRejectsBlankCardWording covers a card whose userId is
// present but whose lead/action are both empty strings — a distinct hole
// from TestComposeWeeklyRejectsMissingCard (which only covers the wholesale
// "cards":[] case): the length cap is trivially satisfied by "", so without
// an explicit non-empty check this reply would pass validation and render a
// blank card on the teacher's screen.
func TestComposeWeeklyRejectsBlankCardWording(t *testing.T) {
	reply := `{"comment":"c","cards":[{"userId":"u1","lead":"","action":""}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — a card with a userId but no wording is a card left without wording")
	}
}

// TestComposeWeeklyRejectsHalfBlankCardLead and
// TestComposeWeeklyRejectsHalfBlankCardAction cover a half-written card —
// only one of lead/action empty. A blank lead or a blank action is still a
// blank line on the teacher's screen, so both fields must be checked
// independently rather than just "cards[i] == zero value".
func TestComposeWeeklyRejectsHalfBlankCardLead(t *testing.T) {
	reply := `{"comment":"c","cards":[{"userId":"u1","lead":"","action":"线下问一句这些数据凭什么说明影响。"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — an empty lead is a half-blank card")
	}
}

func TestComposeWeeklyRejectsHalfBlankCardAction(t *testing.T) {
	reply := `{"comment":"c","cards":[{"userId":"u1","lead":"连续让 AI 直接给结论","action":""}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — an empty action is a half-blank card")
	}
}

func TestComposeWeeklyRejectsBareInternalCode(t *testing.T) {
	reply := `{"comment":"这个班的 D3 普遍偏弱。","cards":[{"userId":"u1","lead":"l","action":"x"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — 说人话: prose names the behaviour, not the code")
	}
}

// TestComposeWeeklyRejectsLowercaseBareCode covers the first evasion the
// reviewer reproduced: bareCode's original character class was uppercase
// only, so a lowercase "d3" sailed through untouched.
func TestComposeWeeklyRejectsLowercaseBareCode(t *testing.T) {
	reply := `{"comment":"这个班的d3普遍偏弱。","cards":[{"userId":"u1","lead":"l","action":"x"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — lowercase must not evade the bare-code check")
	}
}

// TestComposeWeeklyRejectsSpacedBareCode covers the second evasion: a space
// between the letter and the digit ("D 3") didn't match the original
// no-whitespace pattern.
func TestComposeWeeklyRejectsSpacedBareCode(t *testing.T) {
	reply := `{"comment":"这个班的D 3普遍偏弱。","cards":[{"userId":"u1","lead":"l","action":"x"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err == nil {
		t.Fatal("want rejection — a space between the letter and the digit must not evade the bare-code check")
	}
}

// TestComposeWeeklyAllowsProseWithoutBareCode is the negative case for both
// evasion tests above: ordinary prose that never mentions an axis code at
// all must still pass. Guards against a fix that's blunt enough to reject
// everything.
func TestComposeWeeklyAllowsProseWithoutBareCode(t *testing.T) {
	reply := `{"comment":"这周整体表现平稳，没有需要特别关注的地方。","cards":[{"userId":"u1","lead":"连续让 AI 直接给结论","action":"线下问一句这些数据凭什么说明影响。"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, facts())
	if err != nil {
		t.Fatalf("ComposeWeekly: %v — legitimate prose with no bare code must pass", err)
	}
}

func TestComposeWeeklyRejectsDuplicateUser(t *testing.T) {
	f := facts()
	f.Cards = append(f.Cards, agent.WeeklyFactCard{UserID: "u2", Name: "李想", Kind: "praise", TagCode: "grounded_claim", TagLabel: "论证扎实", Evidence: "D2 达到 L4。"})
	reply := `{"comment":"c","cards":[{"userId":"u1","lead":"l","action":"x"},{"userId":"u1","lead":"l2","action":"x2"}]}`
	_, _, err := agent.ComposeWeekly(context.Background(), composeStub(reply), gateway.Resolved{Provider: "stub"}, f)
	if err == nil {
		t.Fatal("want rejection — a repeated student is not the same as covering every card")
	}
}

func TestComposeWeeklyRejectsOverlongComment(t *testing.T) {
	reply := `{"comment":"` + strings.Repeat("很", 400) + `","cards":[{"userId":"u1","lead":"l","action":"x"}]}`
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
	reply := `{"comment":"c","cards":[]}`
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
	if !strings.Contains(p, "本周 12 轮对话，但没有完成能力报告。") {
		t.Fatalf("prompt omits the card's verbatim evidence:\n%s", p)
	}
	if strings.Contains(p, "对话轮次") {
		t.Fatalf("prompt must not carry class-level usage counters:\n%s", p)
	}
}
