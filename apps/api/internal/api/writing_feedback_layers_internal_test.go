package api

import (
	"strings"
	"testing"
)

// 缺口 #13：反馈里缺「为什么错」那一层。
//
// 两份 skill 都要求 `错在哪 → 为什么错（规则或母语负迁移）→ 怎么避免` 三层，
// 并且禁止「只给答案」。而议论文式的「这里逻辑不清」正是被明文禁止的那种评语
// （源逐字：`Avoid generic comments such as 逻辑不清楚, 语法有问题, or
// 词汇需要提高 unless you explain exactly where, why, and how to fix it.`）。
func TestFeedbackAsksForAllThreeLayers(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		s := buildWritingCommentSystem(lang, 3, writingKindPoint, helpAsk, genreArgument)
		for _, want := range []string{"错在哪", "为什么错", "怎么避免"} {
			if !strings.Contains(s, want) {
				t.Errorf("%s：三层里少了 %q", lang, want)
			}
		}
		// 三层要落到已有的两个字段上，否则它只是一句漂亮话。
		if !strings.Contains(s, "写进 text") || !strings.Contains(s, "写进 action") {
			t.Errorf("%s：没说清三层分别写进哪个字段", lang)
		}
		// 明文禁的那几句套话。
		for _, banned := range []string{"逻辑不清", "语法有问题", "词汇需要提高"} {
			if !strings.Contains(s, banned) {
				t.Errorf("%s：没有点名禁掉 %q 这类评语", lang, banned)
			}
		}
	}
}

// 缺口 #14：中式英语的检出方法。
//
// 源里称它「批改最增值的类别」，而检出方法是可编码的一句话：
// 把英文回译回汉语，若回译后是通顺的汉语逐字句，多半是中式英语。
func TestChinglishSectionIsEnglishOnly(t *testing.T) {
	en := buildWritingCommentSystem("en", 3, writingKindPoint, helpAsk, genreArgument)
	if !strings.Contains(en, "回译") {
		t.Error("英文批改里没有回译法")
	}
	for _, want := range []string{"I like it very much", "turn on the light", "Although"} {
		if !strings.Contains(en, want) {
			t.Errorf("中式英语那张典型表里少了 %q", want)
		}
	}
	// 🚨 讲解要点：说清英汉差异的**规则**，不是只说「不地道」。
	// 少了这一句，这一节会退化成一张让模型贴标签的清单。
	if !strings.Contains(en, "不够地道") {
		t.Error("没有拦住「只说不地道」这种讲法")
	}

	// 🚨 反方向：一篇中文作文里没有「回译」这回事，中文那一支要逐字节不变。
	zh := buildWritingCommentSystem("zh", 3, writingKindPoint, helpAsk, genreArgument)
	if strings.Contains(zh, "回译") {
		t.Error("中文批改里混进了中式英语那一节")
	}
	if strings.Contains(zh, "turn on the light") {
		t.Error("中文批改里出现了英文例子")
	}
}
