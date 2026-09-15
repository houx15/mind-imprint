package litegrade

import (
	"strings"
	"testing"

	"mindimprint/api/internal/liteassign"
)

func TestSystemPromptCarriesTheRubric(t *testing.T) {
	in := testInput()
	in.Rubric.Focus = "重点看论证"
	in.SymptomCatalog = "【第 1 层 · 立意】\n- topic_without_question（只有主题，没有问题）：…\n"
	p := SystemPrompt(in)
	for _, want := range []string{"内容", "结构", "语言", "书写规范", "A+ A A- B+ B B- C+ C C- D", "重点看论证", "topic_without_question", "3 到 5 条", "用中文写", "「」"} {
		if !strings.Contains(p, want) {
			t.Errorf("system prompt lacks %q", want)
		}
	}
	in.Rubric = liteassign.Rubric{Scale: liteassign.ScalePoints, Max: 20, Dimensions: []liteassign.RubricDimension{{Name: "Argument", Note: "evidence"}}}
	in.Lang = "en"
	p = SystemPrompt(in)
	for _, want := range []string{"0 到 20 的整数", "Argument：evidence", "用英文写"} {
		if !strings.Contains(p, want) {
			t.Errorf("points/en prompt lacks %q", want)
		}
	}
	if strings.Contains(p, "%!") {
		t.Fatalf("format verbs leaked: %s", p)
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

func TestRetryNudgeListsReasons(t *testing.T) {
	n := RetryNudge([]Reason{{Code: ReasonQuoteNotInBody, Where: "第 1 条意见", Detail: "雨一直下。"}, {Code: ReasonNoGoodPoint}})
	for _, want := range []string{"第 1 条意见的引文不在正文中：「雨一直下。」", "没有优点意见", "完整的 JSON"} {
		if !strings.Contains(n, want) {
			t.Errorf("nudge lacks %q:\n%s", want, n)
		}
	}
}
