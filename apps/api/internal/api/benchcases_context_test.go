package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/benchcase"
	"mindimprint/api/internal/store/sqlc"
)

func readingSuiteCase(t *testing.T, id string) benchcase.Case {
	t.Helper()
	for _, c := range BenchCases() {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("missing case %s", id)
	return benchcase.Case{}
}

func TestReadingBenchmarkContainsOneCurrentStepAndHistory(t *testing.T) {
	c := readingCoachCase()
	user := c.Request.Messages[1].Content
	if strings.Count(user, "← **她现在在这一步**") != 1 || strings.Contains(user, "所有步骤都走完了") {
		t.Fatal("long-context fixture has no single active step")
	}
	if !strings.Contains(user, "(focus_block)") || !strings.Contains(user, "(reflect)") {
		t.Fatal("fixture must use registered task kinds")
	}
	if strings.Count(user, "你：")+strings.Count(user, "她：") != readingCoachTurnsWindow || len([]rune(user)) < 8000 {
		t.Fatal("long-context fixture lost the production-sized article or 14-turn history")
	}
}

func TestLiteReadingSuiteHasNineCurrentCases(t *testing.T) {
	var count int
	for _, c := range BenchCases() {
		if c.Suite != liteReadingCoachSuite {
			continue
		}
		count++
		if c.Parse == nil || c.Validate == nil || c.Version != 8 {
			t.Errorf("%s lacks parser, expectation or current fixture version", c.ID)
		}
		if len(c.Request.Messages) != 2 || !strings.Contains(c.Request.Messages[1].Content, "她现在在这一步") {
			t.Errorf("%s does not use the production prompt with one current task", c.ID)
		}
		if strings.Contains(c.ID, "lens") || strings.Contains(c.ID, "connect") || strings.Contains(c.ID, "concept") {
			t.Errorf("obsolete case still registered: %s", c.ID)
		}
	}
	if count != 9 {
		t.Fatalf("suite has %d cases, want 9", count)
	}
}

func TestReadingLabelFixtureUsesCurrentBoardBins(t *testing.T) {
	c := readingSuiteCase(t, "dialogue/lite-reading-coach/label")
	prompt := c.Request.Messages[1].Content
	for _, want := range []string{
		"关键主张：\n> 过去二十年，中国在可再生能源上的投入规模没有先例。",
		"证据：\n> 中国的可再生能源新增装机量连续八年位居世界第一。",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("label fixture lacks %q", want)
		}
	}
	if strings.Contains(prompt, "限制：\n>") {
		t.Fatal("fixture still submits a removed board bin")
	}
}

func TestReadingCompleteFixtureCoversBothAnswerIdeas(t *testing.T) {
	c := readingSuiteCase(t, "dialogue/lite-reading-coach/complete")
	student := c.Request.Messages[1].Content
	for _, want := range []string{"连续八年", "持续领先", "发电能力", "不等于实际发出的电量"} {
		if !strings.Contains(student, want) {
			t.Fatalf("complete fixture lacks %q", want)
		}
	}
}

func TestReadingHelpFixtureUsesTheRealHelpControl(t *testing.T) {
	c := readingSuiteCase(t, "dialogue/lite-reading-coach/help")
	if !strings.Contains(c.Request.Messages[1].Content, "【她按了「给点提示」】") {
		t.Fatal("help fixture must exercise the production help-control path")
	}
}

func TestReadingFixtureExpectationsAreAdvisoryEvidence(t *testing.T) {
	cases := []struct {
		id, raw string
		valid   bool
	}{
		{"dialogue/lite-reading-coach/help", `{"reply":"回到第三段，看作者在统计什么。","advance":"","focusBlock":"b3","tool":"","lens":"","card":null}`, true},
		{"dialogue/lite-reading-coach/help", `{"reply":"装机容量衡量发电能力，不是实际发电量。","advance":"","focusBlock":"","tool":"","lens":"","card":null}`, false},
		{"dialogue/lite-reading-coach/complete", `{"reply":"你已经解释了这两个信息点。","advance":"done","focusBlock":"","tool":"","lens":"","card":null}`, true},
		{"dialogue/lite-reading-coach/complete", `{"reply":"请再想想。","advance":"","focusBlock":"","tool":"","lens":"","card":null}`, false},
		{"dialogue/lite-reading-coach/hunt-no-pick", `{"reply":"请在文章里点出一句原文。","advance":"done","focusBlock":"","tool":"","lens":"","card":null}`, true},
		{"dialogue/lite-reading-coach/label", `{"reply":"这组分类已提交。","advance":"","focusBlock":"","tool":"","lens":"","card":null}`, true},
	}
	for _, tc := range cases {
		c := readingSuiteCase(t, tc.id)
		if err := c.Parse(tc.raw); err != nil {
			t.Fatalf("%s: parse: %v", tc.id, err)
		}
		if got := c.Validate(tc.raw) == nil; got != tc.valid {
			t.Errorf("%s: valid=%t, want %t", tc.id, got, tc.valid)
		}
	}
}

func TestReadingPromptKeepsFourAgreedBehaviors(t *testing.T) {
	for _, want := range []string{
		"直接给当前问题的完整答案", "只补下一层方向、位置或局部词语线索",
		"跳过不附加工具；完成时按当前步骤说明直接交接下一步", "对她已提交的合理分类，不要为了延长这一步而要求重新分类",
	} {
		if !strings.Contains(readingCoachSystem, want) {
			t.Errorf("prompt lost %q", want)
		}
	}
}

func TestReadingPromptPrioritizesSkipHintAndCompletion(t *testing.T) {
	for _, want := range []string{
		"确认跳过当前步，advance=\"skipped\"，card、lens 留空",
		"沿用那张卡，card、lens 留空，不推进",
		"advance=\"done\"；不再追问",
		"解释一个术语时可以引用其他段落，不能因证据不在当前段而要求重做",
		"只给数字、排名或时间却没有解释它统计的对象仍是部分回答",
		"完成或组件完成回灌时只可按下一步说明给一张下一步 card",
	} {
		if !strings.Contains(readingCoachSystem, want) {
			t.Errorf("prompt lost priority rule %q", want)
		}
	}
}

func TestFocusStepInstructionOnlyIntroducesAnUnansweredStep(t *testing.T) {
	tasks := []sqlc.ReadingTask{{Kind: string(taskFocusBlock), Status: "pending"}}
	opening := readingCurrentStepInstruction(tasks, "")
	if !strings.Contains(opening, "她尚未对当前步骤作答") {
		t.Fatalf("opening focus step lost its introduction: %s", opening)
	}
	for _, input := range []string{coachAskHint, "请跳过这一步。", "我已经回答了。"} {
		got := readingCurrentStepInstruction(tasks, input)
		if strings.Contains(got, "她尚未对当前步骤作答") || !strings.Contains(got, "她已经对当前步骤作答或提出操作请求") {
			t.Errorf("input %q got the opening instruction: %s", input, got)
		}
	}
}

func TestReadingPromptPlacesHelpAfterCurrentStepInstruction(t *testing.T) {
	tasks := []sqlc.ReadingTask{{Kind: string(taskFocusBlock), Status: "pending"}}
	prompt := buildReadingCoachPrompt("标题", nil, readingOutline{}, tasks, nil, nil, coachAskHint, nil, "")
	step := strings.Index(prompt, "【本轮推进判据】")
	help := strings.Index(prompt, "【她按了「给点提示」】")
	if step < 0 || help < 0 || step > help {
		t.Fatalf("help must be the later, turn-specific instruction:\n%s", prompt)
	}
}

func TestBareAnswerRequestCannotSettleCurrentStep(t *testing.T) {
	current := &sqlc.ReadingTask{Kind: string(taskFocusBlock), Status: "pending"}
	for _, said := range []string{"我放弃", "请直接告诉我答案。", "给我答案！"} {
		for _, proposed := range []string{"done", "skipped"} {
			if got := protectedReadingCoachAdvance(proposed, current, nil, nil, nil, nil, said); got != "" {
				t.Errorf("%q with proposed %q must remain active, got %q", said, proposed, got)
			}
		}
	}
	if got := protectedReadingCoachAdvance("skipped", current, nil, nil, nil, nil, "我放弃，跳过这一步"); got != "skipped" {
		t.Fatalf("explicit skip must still work, got %q", got)
	}
}

func TestSkippedReadingTurnDoesNotHandOutNextTool(t *testing.T) {
	card := &coachCard{Type: coachCardShortText, Prompt: "你怎么看？"}
	got := enforceSettledReadingTurn(readingCoachReply{Reply: "当前步骤已跳过。", Advance: "skipped", Card: card, Lens: "lens-economics"})
	if got.Card != nil || got.Lens != "" || got.Advance != "skipped" {
		t.Fatalf("skipped step: %+v", got)
	}
}

func TestCompletedReadingTurnCanHandOutNextCard(t *testing.T) {
	card := &coachCard{Type: coachCardShortText, Prompt: "你怎么看？"}
	got := enforceSettledReadingTurn(readingCoachReply{Reply: "下一步请比较作者的依据。", Advance: "done", Card: card})
	if got.Card != card || got.Advance != "done" {
		t.Fatalf("completed handoff: %+v", got)
	}
}
