package api

import (
	"strings"
	"testing"
)

// The prompt must hand the model the paragraph ORDINAL, not just the id.
//
// A production walk caught the coach saying 「第三段的关键单词我给你点开了」
// while focusBlock came back b4 — so the tool opened on the fourth paragraph
// while the sentence named the third. The model was being asked to map
// b1/b2/… onto 第几段 in its head on every turn, and it split.
//
// These are cheap string assertions on purpose: the failure they guard is a
// prompt regression (someone reverts to writing bare ids), which no
// model-level test would catch reliably and no schema check can see at all.
func TestReadingPrompt_LabelsParagraphsWithTheirOrdinal(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "第一段的正文。"},
		{ID: "b2", Text: "第二段的正文。"},
		{ID: "b3", Text: "第三段的正文。"},
	}

	for _, tc := range []struct {
		name   string
		prompt string
	}{
		{"coach", buildReadingCoachPrompt("标题", blocks, nil, nil, "")},
		{"plan", buildReadingPlanPrompt("zh", "标题", blocks)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, want := range []string{"b1（第1段）", "b2（第2段）", "b3（第3段）"} {
				if !strings.Contains(tc.prompt, want) {
					t.Fatalf("prompt is missing %q — the model is back to inferring the ordinal from the id:\n%s", want, tc.prompt)
				}
			}
		})
	}
}

// An empty paragraph is skipped in the prompt but still occupies a position in
// the article she is looking at, so the ordinals must NOT close up over it —
// otherwise 「第3段」 in the prompt is 第4段 on her screen.
func TestReadingBlockTag_CountsSkippedParagraphs(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "有内容。"},
		{ID: "b2", Text: "   "},
		{ID: "b3", Text: "也有内容。"},
	}
	prompt := buildReadingCoachPrompt("", blocks, nil, nil, "")

	if strings.Contains(prompt, "b2") {
		t.Fatalf("the empty paragraph should not be in the prompt at all:\n%s", prompt)
	}
	if !strings.Contains(prompt, "b3（第3段）") {
		t.Fatalf("b3 must stay 第3段 — renumbering it to 第2段 would point her at the wrong paragraph:\n%s", prompt)
	}
}
