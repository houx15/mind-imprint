package api

// reading_questions_internal_test.go — validateReadingQuestions is
// unexported, so its tests live here (package api) alongside
// reading_brief_internal_test.go and reading_coach_internal_test.go's own
// unexported-symbol tests, rather than in package api_test where the HTTP
// tests live (reading_coach_test.go and friends cannot see unexported
// symbols — this has bitten prior tasks in this plan).

import "testing"

func TestValidateReadingQuestions(t *testing.T) {
	body := "中国的碳排放总量位居世界第一。\n\n但人均排放仍低于多数发达国家。"
	got := validateReadingQuestions([]readingQuestionDraft{
		{Text: "人均排放和总量，哪个更该被用来衡量责任？", AnchorQuote: "人均排放仍低于多数发达国家"},
		{Text: "你怎么看待环保？", AnchorQuote: "环境保护很重要"}, // not in the article — dropped
		{Text: "总量第一意味着什么？", AnchorQuote: "碳排放总量位居世界第一"},
		{Text: "  ", AnchorQuote: "中国的碳排放总量"}, // no question — dropped
	}, body)
	if len(got) != 2 {
		t.Fatalf("kept %d, want 2: %+v", len(got), got)
	}
}

// Fewer than two survivors means the model produced generalities. A thin,
// generic suggestion is worse than no suggestion — so show nothing.
func TestValidateReadingQuestionsNeedsTwo(t *testing.T) {
	body := "中国的碳排放总量位居世界第一。"
	got := validateReadingQuestions([]readingQuestionDraft{
		{Text: "总量第一意味着什么？", AnchorQuote: "碳排放总量位居世界第一"},
		{Text: "你觉得环保重要吗？", AnchorQuote: "环保重要"},
	}, body)
	if len(got) != 0 {
		t.Fatalf("want none when fewer than two survive, got %d", len(got))
	}
}
