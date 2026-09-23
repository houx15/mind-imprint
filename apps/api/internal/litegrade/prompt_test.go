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
		// 2026-09-23: the countable-facts block (internal/textstat) — the
		// instruction not to recompute and not to surface the raw numbers to
		// the student.
		"## 事实", "不用你重新数一遍", "不要把这里的具体数字写进给学生看的内容里",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("system prompt lacks %q", want)
		}
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
		fmt.Sprintf("词汇多样度（不重复词数 / 总词数）：%.0f%%", s.TypeTokenRatio*100),
		fmt.Sprintf("平均句长：%.1f 词", s.MeanSentenceLength),
		fmt.Sprintf("复杂句占比：%.0f%%", s.ComplexSentenceRatio*100),
		fmt.Sprintf("连接词密度：每句 %.1f 个", s.ConnectiveDensity),
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

func TestRetryNudgeListsReasons(t *testing.T) {
	n := RetryNudge([]Reason{{Code: ReasonQuoteNotInBody, Where: "第 1 条意见", Detail: "雨一直下。"}, {Code: ReasonNoGoodPoint}})
	for _, want := range []string{"第 1 条意见的引文不在正文中：「雨一直下。」", "没有优点意见", "完整的 JSON"} {
		if !strings.Contains(n, want) {
			t.Errorf("nudge lacks %q:\n%s", want, n)
		}
	}
}
