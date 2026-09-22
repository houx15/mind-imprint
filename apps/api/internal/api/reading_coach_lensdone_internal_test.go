package api

// reading_coach_lensdone_internal_test.go — the prompt half of 「透镜应用完毕
// 之后，没有响应，没有推进到下一步」.
//
// What is worth pinning here is NOT that a section renders (reading the code
// tells you that). It is the two invariants that are invisible on the page
// and that a later edit would break without any test going red:
//
//  1. A completed lens must SUPPRESS the 「她刚点了「开始」」 fallback. That
//     line is the room's landmine: it is emitted whenever studentText is
//     empty, and a lens completion carries no typed text. Leaving it in makes
//     印记 re-introduce the whole reading plan the instant she finishes a lens
//     — the same failure the PBL room shipped and had to fix
//     ([[pbl-online-e2e-2026-09-02]]: an empty-text turn after finishing a
//     tool made 印记 repeat itself verbatim and re-summon the tool she had
//     just done).
//  2. The finding must be labelled as 印记's OWN prior words. It comes from
//     agent.EvaluateSelection, not from her mouth, and a prompt that lets it
//     read as hers teaches the model to quote it back to her as something she
//     said — the same R4 confusion the report has its own guards for.

import (
	"strings"
	"testing"
)

func lensDoneBlocks() []Block {
	return []Block{
		{ID: "b1", Text: "中国的光伏装机量在过去十年增长了二十倍。"},
		{ID: "b2", Text: "但人均排放仍低于多数发达国家。"},
	}
}

func TestLensDoneSuppressesThePressedStartFallback(t *testing.T) {
	// 🚨 The whole point. She typed nothing, but she did not press 开始.
	done := &readingLensDone{
		CardName: "溯源体检",
		Quote:    "但人均排放仍低于多数发达国家。",
		Finding:  "这句把总量和人均分开了，是一次口径切换。",
	}
	prompt := buildReadingCoachPrompt("标题", lensDoneBlocks(), readingOutline{}, nil, nil, nil, "", done, "")

	if strings.Contains(prompt, "学生刚点了「开始」") {
		t.Fatalf("a finished lens must never look like 开始 — 印记 would re-introduce the plan:\n%s", prompt)
	}
	if !strings.Contains(prompt, "【学生刚做完一副透镜】") {
		t.Fatalf("missing the lens-done section:\n%s", prompt)
	}
	if !strings.Contains(prompt, "学生已完成透镜选句") {
		t.Fatalf("the empty-text branch must say what actually happened:\n%s", prompt)
	}
}

func TestLensDoneAttributesTheFindingToTheCoach(t *testing.T) {
	// The finding is EvaluateSelection's output — 印记's own earlier words.
	// Only the quote is hers.
	done := &readingLensDone{
		CardName: "溯源体检",
		Quote:    "但人均排放仍低于多数发达国家。",
		Finding:  "这句把总量和人均分开了。",
	}
	prompt := buildReadingCoachPrompt("标题", lensDoneBlocks(), readingOutline{}, nil, nil, nil, "", done, "")

	i := strings.Index(prompt, "【学生刚做完一副透镜】")
	if i < 0 {
		t.Fatalf("missing section:\n%s", prompt)
	}
	section := prompt[i:]
	if !strings.Contains(section, "学生自己在文章里找的那一句") {
		t.Errorf("the quote must be marked as HERS:\n%s", section)
	}
	if !strings.Contains(section, "不属于学生的作答，不据此推断学生已有的认识") {
		t.Errorf("the finding must be marked as the coach's own words:\n%s", section)
	}
}

func TestLensDoneWithNoQuoteIsNotACompletedLens(t *testing.T) {
	// `clean()` is what stops the room buying a flagship turn to say "nice
	// work" about nothing — and, just as importantly, what keeps the
	// 「她刚点了「开始」」 branch reachable for the turn that really IS 开始.
	for _, tc := range []struct {
		name string
		done *readingLensDone
	}{
		{"nil", nil},
		{"empty quote", &readingLensDone{CardName: "溯源体检"}},
		{"whitespace quote", &readingLensDone{CardName: "溯源体检", Quote: "   \n "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.done.clean() {
				t.Fatalf("clean() must reject %s", tc.name)
			}
			prompt := buildReadingCoachPrompt("标题", lensDoneBlocks(), readingOutline{}, nil, nil, nil, "", tc.done, "")
			if strings.Contains(prompt, "【学生刚做完一副透镜】") {
				t.Errorf("no section for %s:\n%s", tc.name, prompt)
			}
			if !strings.Contains(prompt, "学生刚点了「开始」") {
				t.Errorf("the real 开始 turn must keep its own line for %s:\n%s", tc.name, prompt)
			}
		})
	}
}

func TestLensDoneStillYieldsToWhatSheTyped(t *testing.T) {
	// She may finish a lens AND say something in the same beat. What she
	// typed is the more direct thing and keeps 【她刚刚说的】; the lens still
	// gets its own section, so neither is dropped.
	done := &readingLensDone{CardName: "溯源体检", Quote: "但人均排放仍低于多数发达国家。"}
	prompt := buildReadingCoachPrompt("标题", lensDoneBlocks(), readingOutline{}, nil, nil, nil, "我觉得这句在换口径", done, "")

	if !strings.Contains(prompt, "我觉得这句在换口径") {
		t.Errorf("her own words must survive:\n%s", prompt)
	}
	if !strings.Contains(prompt, "【学生刚做完一副透镜】") {
		t.Errorf("the lens section must survive alongside her words:\n%s", prompt)
	}
	if strings.Contains(prompt, "学生已完成透镜选句") {
		t.Errorf("the no-text explanation must not appear when she DID type:\n%s", prompt)
	}
}
