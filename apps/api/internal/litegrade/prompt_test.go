package litegrade

import (
	"fmt"
	"strings"
	"testing"

	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/textstat"
)

func TestSystemPromptCarriesTheRubric(t *testing.T) {
	in := testInput()
	in.Rubric.Focus = "重点看论证"
	in.SymptomCatalog = "【第 1 层 · 立意】\n- topic_without_question（只有主题，没有问题）：…\n"
	p := SystemPrompt(in)
	for _, want := range []string{
		"内容", "结构", "语言", "书写规范", "A+ A A- B+ B B- C+ C C- D", "重点看论证", "topic_without_question", "3 到 5 条", "用中文写", "「」",
		// 2026-09-23: points[].dimension / .symptom — the teacher-facing
		// 依据 modal's two provenance fields.
		"每条再给一个 dimension", "issue 再给一个 symptom", "不要新造一个 id",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("system prompt lacks %q", want)
		}
	}
	// 🚨 那几个可数指标是**每个学生都不一样**的，不能待在系统提示词里：
	// 一个易变块插在中间，整班批改就丢掉了共用前缀的缓存命中
	// （AGENTS.md「prompt 块的顺序是一条成本契约」）。它们在 UserPrompt 里。
	for _, unwanted := range []string{"多样度", "平均句长", "连接词密度", "不用你重新数"} {
		if strings.Contains(p, unwanted) {
			t.Errorf("per-student facts must not be in the SYSTEM prompt: %q", unwanted)
		}
	}
	// 同一份 rubric、同一种语言、不同的学生 ⇒ 系统提示词**逐字相同**。
	other := in
	other.Body = "完全不同的另一位学生交上来的正文，长度和用词都不一样。"
	other.Title = "另一个标题"
	other.VersionNumber = 7
	if SystemPrompt(other) != p {
		t.Fatal("the system prompt must be byte-identical across students in one assignment")
	}
	if !strings.Contains(p, `"action":null,"dimension":"…"`) || !strings.Contains(p, `"action":"…","dimension":"…","symptom":"…"`) {
		t.Fatalf("output skeleton must show dimension/symptom on the good and issue examples: %s", p)
	}
	in.Rubric = liteassign.Rubric{Scale: liteassign.ScalePoints, Max: 20, Dimensions: []liteassign.RubricDimension{{Name: "Argument", Note: "evidence"}}}
	in.Lang = "en"
	p = SystemPrompt(in)
	for _, want := range []string{"0 到 20 的整数", `name 写 "Argument"；这一维看：evidence`, `"dimensions":[{"name":"Argument","grade":"…","comment":"…"}]`, "用英文写"} {
		if !strings.Contains(p, want) {
			t.Errorf("points/en prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "%!") {
		t.Fatalf("format verbs leaked: %s", p)
	}
}

func TestRetryNudgeExplainsAnUnparseableReply(t *testing.T) {
	got := RetryNudge([]Reason{{Code: ReasonUnparseable, Detail: "invalid character ':' after array element"}})
	for _, want := range []string{"回复不是有效的 JSON", "invalid character ':' after array element", "数组里只能放"} {
		if !strings.Contains(got, want) {
			t.Fatalf("nudge lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(RetryNudge([]Reason{{Code: ReasonEmptyText, Where: "总评"}}), "数组里只能放") {
		t.Fatal("the array hint belongs to unparseable replies only")
	}
}

func TestUserPromptLabelsTheTeachersText(t *testing.T) {
	in := testInput()
	in.TargetWords = 800
	p := UserPrompt(in)
	if !strings.Contains(p, "作业题目（老师布置，不是学生的原文）：\n"+testPrompt) {
		t.Fatalf("assigned prompt not labelled as the teacher's: %s", p)
	}
	if !strings.Contains(p, "学生正文（第 1 版）：\n"+testBody) || !strings.Contains(p, "目标字数：800") {
		t.Fatalf("body/target missing: %s", p)
	}
	in.AssignedPrompt = ""
	if strings.Contains(UserPrompt(in), "作业题目") {
		t.Fatal("no prompt line for a writing that is not homework")
	}
}

// TestFactsBlockReflectsTextstat pins the countable-facts block to
// internal/textstat's own numbers for in.Body/in.Lang, so a change to the
// Sprintf formatting (wrong verb, wrong field, stale rounding) shows up here
// instead of only being caught by eye in a captured prompt.
func TestFactsBlockReflectsTextstat(t *testing.T) {
	in := testInput()
	s := textstat.Compute(in.Body, in.Lang)
	want := []string{
		fmt.Sprintf("用字多样度（不重复字数 / 总字数，中文按字计）：%.0f%%", s.TypeTokenRatio*100),
		fmt.Sprintf("平均句长：%.1f 字", s.MeanSentenceLength),
		fmt.Sprintf("含从句的句子占比（按连词词表匹配）：%.0f%%", s.ComplexSentenceRatio*100),
		fmt.Sprintf("连接词密度（按连接词词表匹配）：每句 %.1f 个", s.ConnectiveDensity),
	}
	got := factsBlock(in)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("factsBlock lacks %q, got:\n%s", w, got)
		}
	}
	// A different body must change the block — this is Input.Body-derived,
	// not a static string.
	in2 := in
	in2.Body = "这是一段完全不同的正文，用来确认事实块会跟着正文变化，而不是写死的。"
	if factsBlock(in2) == got {
		t.Fatal("factsBlock did not change with a different Body")
	}
}

// 🚨 中文那一路的 token 是**字**，不是词（textstat.wordTokens 照
// agent.CountWords，每个汉字一个 token）。两种语言的标签必须分开写，
// 否则那两个数字看起来可比，而它们不可比。
func TestFactsBlockLabelsTheUnitPerLanguage(t *testing.T) {
	zh := factsBlock(testInput())
	if !strings.Contains(zh, "中文按字计") || !strings.Contains(zh, "平均句长：") || strings.Contains(zh, "词汇多样度") {
		t.Fatalf("zh facts must be labelled in 字, got:\n%s", zh)
	}
	if !strings.Contains(zh, " 字\n") {
		t.Fatalf("zh mean sentence length must be labelled 字, got:\n%s", zh)
	}
	in := testInput()
	in.Lang = "en"
	en := factsBlock(in)
	if !strings.Contains(en, "词汇多样度（不重复词数 / 总词数）") || !strings.Contains(en, " 词\n") {
		t.Fatalf("en facts must be labelled in 词, got:\n%s", en)
	}
}

// 🚨 统计块住在**用户**消息里，而且排在她的正文后面 —— 它是每个学生都不一样
// 的东西（prompts.GradingFactsBlock 的注释说明了这条成本契约）。
func TestUserPromptCarriesTheFactsBlock(t *testing.T) {
	in := testInput()
	p := UserPrompt(in)
	for _, want := range []string{"这篇的几项统计", "你不用重新数", "不要把这里的具体数字写进给学生看的内容里", "平均句长："} {
		if !strings.Contains(p, want) {
			t.Errorf("user prompt lacks %q:\n%s", want, p)
		}
	}
	if strings.Index(p, "学生正文") > strings.Index(p, "这篇的几项统计") {
		t.Fatal("the facts block must come after her body, not before it")
	}
	if strings.Contains(p, "%!") {
		t.Fatalf("format verbs leaked: %s", p)
	}
	// 「不是估计」不能回来：这两项是按词表匹配的近似值，不是句法分析。
	if strings.Contains(p, "不是估计") {
		t.Fatal("ComplexSentenceRatio / ConnectiveDensity are lexical proxies; do not call them exact")
	}
}

func TestRetryNudgeListsReasons(t *testing.T) {
	n := RetryNudge([]Reason{{Code: ReasonQuoteNotInBody, Where: "第 1 条意见", Detail: "雨一直下。"}, {Code: ReasonNoGoodPoint}})
	for _, want := range []string{"第 1 条意见的引文不在正文中：「雨一直下。」", "没有优点意见", "完整的 JSON"} {
		if !strings.Contains(n, want) {
			t.Errorf("nudge lacks %q:\n%s", want, n)
		}
	}
}
