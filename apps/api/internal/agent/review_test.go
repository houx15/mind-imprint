package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
)

// reviewProvider returns a canned model reply, reusing the same
// gateway.NewStubProvider fake pattern already used by coach_test.go /
// anchors_test.go (scriptedProvider) — no new provider interface.
func reviewProvider(reply string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: reply},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 42, OutputTokens: 17}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestProposeReview_ParsesWorkOrder(t *testing.T) {
	reply := `[
      {"criterion_code":"表E","band":"5–6 段","evidence":"第2段接住反方","missing":"变绿→可持续的跳步没补","fix":"补上可持续的定义"},
      {"criterion_code":"表H","band":"7–8 段","evidence":"结构清楚","missing":"","fix":""}
    ]`
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}, {Code: "表H", Name: "表达与组织"}}
	items, usage, err := ProposeReview(context.Background(), reviewProvider(reply), gateway.Resolved{Provider: "deepseek", Model: "x"}, criteria, []string{"p1", "p2"}, "claims:1 evidence:2")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].CriterionCode != "表E" || items[0].CriterionName != "分析" || items[0].Fix == "" {
		t.Fatalf("item0 = %+v", items[0])
	}
	_ = usage
}

func TestProposeReview_RejectsBannedPhrase(t *testing.T) {
	// A reply whose fix rewrites the student's sentence for her — must be
	// rejected by the enforcement stack (banned-phrasing), not returned.
	reply := `[{"criterion_code":"表E","band":"5–6 段","evidence":"e","missing":"m","fix":"你应该这样写：中国的转型是叠加式的。"}]`
	criteria := []skills.ReviewCriterion{{Code: "表E", Name: "分析"}}
	_, _, err := ProposeReview(context.Background(), reviewProvider(reply), gateway.Resolved{Provider: "deepseek", Model: "x"}, criteria, []string{"p1"}, "")
	if err == nil {
		t.Fatal("expected banned-phrasing rejection, got nil")
	}
	_ = strings.TrimSpace
}
