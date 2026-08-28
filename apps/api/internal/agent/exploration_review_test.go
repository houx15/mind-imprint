package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// exploration_review_test.go — bug report 2026-08-28 §2. A student who has
// collected papers into 未归类 and asks 印记 to "理一理文献" must have those
// papers actually reach the model. They used to be structurally invisible: the
// caller projected 子问题-edge targets only, and 未归类 sources hang under no
// question by definition.

func newRecordingProvider(text string) *gateway.StubProvider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 40, OutputTokens: 20}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func promptOf(t *testing.T, prov *gateway.StubProvider) string {
	t.Helper()
	for _, m := range prov.LastRequest.Messages {
		if m.Role == gateway.RoleUser {
			return m.Content
		}
	}
	t.Fatalf("no user message in the recorded request")
	return ""
}

func TestReviewExploration_CarriesUnfiledPapers(t *testing.T) {
	prov := newRecordingProvider("《NASA 卫星植被覆盖数据》应该挂到第一个问题下。")
	_, _, err := ReviewExploration(context.Background(), prov, gateway.Resolved{Provider: "stub"}, ExplorationReviewInput{
		Question: "中国是否让地球变得更可持续？",
		SubQuestions: []ExplorationReviewSubQuestion{
			{Text: "碳排放趋势如何？", Papers: []ExplorationReviewPaper{{Title: "已归位的一篇", Nature: "support"}}},
		},
		Unfiled: []ExplorationReviewPaper{
			{Title: "NASA 卫星植被覆盖数据", Nature: "support", Argument: "中国在变绿", Finding: "2001–2020 覆盖率上升"},
			{Title: "一篇还没标注的", Nature: ""},
		},
	})
	if err != nil {
		t.Fatalf("ReviewExploration err = %v", err)
	}
	p := promptOf(t, prov)
	if !strings.Contains(p, "未归类的材料") {
		t.Errorf("prompt is missing the 未归类 block:\n%s", p)
	}
	for _, want := range []string{"NASA 卫星植被覆盖数据", "一篇还没标注的", "已归位的一篇"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt is missing %q:\n%s", want, p)
		}
	}
	// Unfiled papers count toward the total the model is told about — a "共 1 篇"
	// footer under three papers is exactly how the old projection made the
	// review answer "材料还太少".
	if !strings.Contains(p, "共 3 篇材料") {
		t.Errorf("total should count unfiled papers, got:\n%s", p)
	}
}

// A project whose warren was never proposal-seeded has no questions at all.
// The review must still run on the collected material (and be told the map is
// empty) rather than send a blank map and get "材料还太少" back.
func TestReviewExploration_NoQuestionsStillReviewsTheMaterial(t *testing.T) {
	prov := newRecordingProvider("可以立这两个问题…")
	_, _, err := ReviewExploration(context.Background(), prov, gateway.Resolved{Provider: "stub"}, ExplorationReviewInput{
		Unfiled: []ExplorationReviewPaper{{Title: "第一篇"}, {Title: "第二篇"}},
	})
	if err != nil {
		t.Fatalf("ReviewExploration err = %v", err)
	}
	p := promptOf(t, prov)
	if !strings.Contains(p, "学生还没立下问题") {
		t.Errorf("prompt should say the student has no question yet:\n%s", p)
	}
	if !strings.Contains(p, "还没有任何问题节点") {
		t.Errorf("prompt should flag the empty map:\n%s", p)
	}
	if !strings.Contains(p, "第一篇") || !strings.Contains(p, "第二篇") {
		t.Errorf("prompt should still carry the collected material:\n%s", p)
	}
}

// The system prompt must actually ASK for the placement suggestion — carrying
// the papers without the instruction leaves the student with prose that never
// answers "where does this one go?".
func TestExplorationReviewSystem_AsksWhereUnfiledPapersBelong(t *testing.T) {
	if !strings.Contains(explorationReviewSystem, "未归类的材料") {
		t.Errorf("system prompt never mentions 未归类")
	}
	if !strings.Contains(explorationReviewSystem, "铁律②") {
		t.Errorf("system prompt must keep the advisory-only constraint")
	}
}
