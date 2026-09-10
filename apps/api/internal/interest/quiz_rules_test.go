package interest

import (
	"strings"
	"testing"
)

// quiz_rules_test.go —— 兴趣测试和采集共用什么、不共用什么。
//
// 🚨 这个文件挡的是一件**读代码看不出来**的事：两条 prompt 拼在一起长得很像，
// 而其中一段判据放在测试里是反的。
//
// 2026-09-11 实测（真模型，同一条真实作答，各三次）：
//
//	复用采集的判据   0/3 长出词
//	换成测试自己的   3/3（decision-making，evidence 是她的原话）
//
// 学生那边看到的差别，是「这次没有长出关键词」和一个词。

func quizSystem(t *testing.T) string {
	t.Helper()
	system, _ := Attempt{
		Navigator: "腹黑军师",
		Work:      "《进击的巨人》里的利威尔",
		Reason:    "他经历了很多痛苦，但在关键时刻依然保持理智，做出自己的选择。",
		Hook:      HookCharacter,
	}.Clean().BuildQuizPrompt()
	return system
}

// 🚨 这一条是那个 bug 本身。采集那条判据的核心是「材料的话题不算」，而测试里
// 没有材料 —— 她挑的作品和她写的理由都是她自己敲的。照搬过来，模型会把她的作答
// 当成「别人给的材料」整份丢掉。
func TestQuizPromptDoesNotCarryTheHarvestOnlyRule(t *testing.T) {
	system := quizSystem(t)
	for _, banned := range []string{"不是这篇材料的话题", "文章讲了什么不算"} {
		if strings.Contains(system, banned) {
			t.Errorf("兴趣测试的 prompt 里混进了只属于采集的判据：%q\n"+
				"测试里没有「材料」，这条会让模型把她自己的作答丢掉（实测 0/3）", banned)
		}
	}
}

// 反过来，测试必须真的告诉模型「她挑的东西就是根据」。
func TestQuizPromptSaysHerOwnChoiceIsTheEvidence(t *testing.T) {
	system := quizSystem(t)
	if !strings.Contains(system, "她挑的东西和她给的理由，本身就是根据") {
		t.Error("兴趣测试的 prompt 没有告诉模型：她自己挑的作品与理由就是根据")
	}
}

// 分开的**只有那一段**。词表、字段要求、JSON 形状必须仍然和采集是同一套 ——
// 否则同一棵树上会挂着两种质量的词，而那正是当初决定共用的理由。
func TestQuizAndHarvestStillShareEverythingElse(t *testing.T) {
	quiz := quizSystem(t)
	harvest, _ := BuildHarvestPrompt("reading", "标题", "她写的一段话。")

	shared := []string{
		"你只能从下面这张表里选，不能自己造词",          // 闭表这条规矩
		`{"keywords":[{"id":"","note":"","evidence":""}]}`, // JSON 形状
		"id：**上表里的 id 原样照抄**",                       // 字段要求
		"evidence：**她自己写的原话**",                       // 逐字摘录这条
		"宁可只给一个，也不要凑满三个",
	}
	for _, frag := range shared {
		if !strings.Contains(quiz, frag) {
			t.Errorf("兴趣测试的 prompt 丢了本该共用的一段：%q", frag)
		}
		if !strings.Contains(harvest, frag) {
			t.Errorf("采集的 prompt 丢了本该共用的一段：%q", frag)
		}
	}

	// 词表两边必须是同一份 —— 手抄两份一定会漂。
	const anInterestID = "decision-making"
	if !strings.Contains(quiz, anInterestID) || !strings.Contains(harvest, anInterestID) {
		t.Error("两条 prompt 没有共用同一张候选词表")
	}
}

// 采集那一份得留着它自己的判据 —— 它在采集的处境里是对的：那里真的有一篇材料，
// 而「文章提到游戏 ≠ 她对游戏感兴趣」正是要挡的事。
func TestHarvestPromptKeepsItsOwnRule(t *testing.T) {
	harvest, _ := BuildHarvestPrompt("reading", "标题", "她写的一段话。")
	if !strings.Contains(harvest, "不是这篇材料的话题") {
		t.Error("采集的 prompt 丢了「材料的话题不算」这条 —— 它在采集里是对的")
	}
}
