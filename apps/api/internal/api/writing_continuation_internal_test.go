package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 读后续写（2026-09-24 新加）。高考英语写作里它占 25 分、43% 的题量，
// 在这之前这个产品一件都没做 —— 一道续写题会落到记叙文那一支。

// 一段真题形状的题面：前文 + 两个印好的段首句。
const continuationAssigned = `阅读下面材料，根据其内容和所给段落开头语续写两段，使之构成一篇完整的短文。

David had been training for the school marathon for three months. Every morning he
got up before dawn and ran the long road past the old mill, and every evening his
younger brother Toby waited at the gate with a packet of biscuits. Toby had never
missed a single evening. On the day before the race David twisted his ankle on a
loose stone, and that night he sat on the step without saying anything at all while
Toby held out the biscuits and did not know what to say.

Paragraph 1: The next morning David woke to find the house completely silent.
Paragraph 2: When he finally reached the starting line, Toby was already there.`

func TestContinuationRoutesAheadOfNarrativeAndLetter(t *testing.T) {
	cases := []struct {
		name  string
		title string
	}{
		{"中文题面", "读后续写：一次马拉松"},
		{"只写续写两个字", "根据短文内容续写两段"},
		{"英文题面", "Continue the story in two paragraphs"},
		// 🚨 形式判据：两个段首句印在题面上。语料里 43 道读后续写全是这个形状。
		{"靠两个段首句认出来", "Read the passage and finish it. Paragraph 1: He woke early. Paragraph 2: She was waiting."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := writingGenreOf(sqlc.Writing{Title: tc.title}, nil); got != genreContinuation {
				t.Errorf("「%s」判成了 %q，要的是读后续写", tc.title, got)
			}
		})
	}
}

// 🚨 反方向：它不许把别的题抢走。
//
// 续写的题面里一定有一段记叙性的前文，有时前文本身还是一封信 —— 所以它被放在
// 最先查。放最先就有抢别人的风险，这一条守着那个风险。
func TestContinuationDoesNotStealOtherGenres(t *testing.T) {
	cases := map[string]string{
		"一封普通的信":   "给外婆写一封信，说说你最近的生活",
		"记叙文":      "记一件让你难忘的事",
		"议论文":      "学校应不应该允许学生带手机",
		"英文书信":     "Write a letter to your foreign teacher about the English Corner",
		"只提到一个段落":  "Write a paragraph 1 of your essay about pollution",
		"continue 不算": "How can we continue to improve our study habits?",
	}
	for name, title := range cases {
		t.Run(name, func(t *testing.T) {
			if got := writingGenreOf(sqlc.Writing{Title: title}, nil); got == genreContinuation {
				t.Errorf("「%s」被判成了读后续写", title)
			}
		})
	}
}

// 题面齐备时，两个段首句都挑得出来，前文留在 Source 里。
func TestParseContinuationInputsFindsBothOpeners(t *testing.T) {
	got := parseContinuationInputs(continuationAssigned)
	if len(got.Openers) != 2 {
		t.Fatalf("挑出了 %d 个段首句：%q", len(got.Openers), got.Openers)
	}
	if !strings.HasPrefix(got.Openers[0], "The next morning") {
		t.Errorf("第一个段首句不对：%q", got.Openers[0])
	}
	if !strings.HasPrefix(got.Openers[1], "When he finally reached") {
		t.Errorf("第二个段首句不对：%q", got.Openers[1])
	}
	// 前文还在，而且段首句不许留在里面（留着模型会把它当前文的一部分读）。
	if !strings.Contains(got.Source, "David had been training") {
		t.Error("前文没留下")
	}
	if strings.Contains(got.Source, "Paragraph 1:") {
		t.Error("段首句还留在前文里")
	}
	if !got.Ready() {
		t.Error("三样齐备却判成了不齐")
	}
	if continuationGateNote(got) != "" {
		t.Error("依据齐备时还在说判不了")
	}
}

// 🚨 这条是这一档的硬门槛：没有前文就不许判内容。
//
// 源材料 §4.1：「缺前文/段首句时：只能评语言与句子层面，明确提示『内容
// （融洽度/逻辑）无法判档，请补原文』。」
//
// 这正是 [[ai-errors-must-surface-never-fake]] 在教学上的样子 —— 缺了依据就
// 说缺了，不要拿一段像模像样的内容评价糊过去。
func TestContinuationGateRefusesToJudgeContentWithoutTheSource(t *testing.T) {
	cases := map[string]string{
		"什么都没给":      "",
		"只有题目说明":     "根据材料续写两段，词数 150 左右。",
		"只有段首句没有前文": "Paragraph 1: He woke early.\nParagraph 2: She was waiting.",
	}
	for name, assigned := range cases {
		t.Run(name, func(t *testing.T) {
			in := parseContinuationInputs(assigned)
			if in.Ready() {
				t.Fatalf("依据不齐却判成了齐备：%+v", in)
			}
			note := continuationGateNote(in)
			if note == "" {
				t.Fatal("依据不齐却没有提醒")
			}
			// 提醒要说清**判不了什么**，以及**要她补什么** —— 只说「资料不全」
			// 等于把问题丢回给学生。
			for _, want := range []string{"伏笔", "判不了", "补上"} {
				if !strings.Contains(note, want) {
					t.Errorf("提醒里没有 %q：%s", want, note)
				}
			}
		})
	}
}

// 前文太短就当它没给 —— 几十个字的东西是题目说明，不是前文。
func TestContinuationShortBlurbIsNotASourceText(t *testing.T) {
	assigned := "请续写两段。\nParagraph 1: He woke early.\nParagraph 2: She was waiting."
	if parseContinuationInputs(assigned).Ready() {
		t.Error("一句题目说明被当成了前文")
	}
}

// 同一个段号在题面里出现两次（题干一次、答题位一次）只认第一次。
func TestContinuationIgnoresARepeatedOpener(t *testing.T) {
	got := parseContinuationInputs(continuationAssigned +
		"\n\nParagraph 1: The next morning David woke to find the house completely silent.")
	if len(got.Openers) != 2 {
		t.Errorf("重复的段首句被当成了第三段：%q", got.Openers)
	}
}

// 🚨 别的文体上这一块一个字节都不写 —— 否则每一篇作文的 prompt 都变了，
// 而块的顺序是一条成本契约。
func TestContinuationGateWritesNothingForOtherGenres(t *testing.T) {
	assigned := continuationAssigned
	wr := sqlc.Writing{Title: "记一件让你难忘的事", Lang: "zh", AssignedPrompt: &assigned}
	withGate := buildWritingPlanPrompt(wr, nil, nil, "我想写运动会")
	if strings.Contains(withGate, "这道题的依据还不齐") {
		t.Error("记叙文的 prompt 里出现了读后续写的门槛提示")
	}
}

// 读后续写上，依据不齐时那一句要真的装进 prompt。
func TestContinuationGateReachesThePrompt(t *testing.T) {
	assigned := "读后续写：根据材料续写两段。"
	wr := sqlc.Writing{Title: "读后续写", Lang: "en", AssignedPrompt: &assigned}
	got := buildWritingPlanPrompt(wr, nil, nil, "我不知道怎么开头")
	if !strings.Contains(got, "这道题的依据还不齐") {
		t.Errorf("门槛提示没装进 prompt：\n%s", got)
	}
}

// 续写那一档的系统提示词要是续写的，不是记叙文的、更不是议论文的。
func TestContinuationSystemPromptIsItsOwn(t *testing.T) {
	s := writingPlanSystemFor(genreContinuation, "en", "")
	for _, want := range []string{"段首句", "咬", "伏笔"} {
		if !strings.Contains(s, want) {
			t.Errorf("续写的提示词里没有 %q", want)
		}
	}
	if strings.Contains(s, "总—分—总") {
		t.Error("续写的提示词里混进了议论文的篇章结构")
	}
	// 🚨 块的种类和记叙文共用是**故意的**，所以这几个 id 必须在。
	for _, want := range []string{"「scene」", "「turn」", "「feeling」"} {
		if !strings.Contains(s, want) {
			t.Errorf("续写的节点类型表里没有 %q", want)
		}
	}
	if strings.Contains(s, "「thesis」") || strings.Contains(s, "「point」") {
		t.Error("续写的提示词里把中心论点/分论点列成了可选的节点类型")
	}
}
