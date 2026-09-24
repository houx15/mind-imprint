package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/prompts"
)

// 🚨 **提示词里那张体裁清单，必须和闭表一致。**
//
// 2026-09-23 线上走查：加了 poem / classical 两种体裁、两套读法、两块板、
// 两段带读说明、三件工具，全部单测通过，9 格矩阵通过 —— 然后线上拿《江雪》
// 和《咏雪》走一遍，两篇都落到 zh-narrative。
//
// 原因只有一句：`ReadingPlanSystem` 里写着
//
//	genre：**只取 argument / report / narrative / explain**
//
// 模型**根本输不出** poem 和 classical。候选读法那一栏列着它们，
// 看起来接好了；真正决定体裁的那一行却是一张写死的旧清单。
//
// 这就是 AGENTS.md「提示词怎么写」第 6 条记的那个形状：新开一条分支，
// 而判据落在没被覆盖的地方，整套测试照样绿。所以这里把「清单和闭表一致」
// 本身变成判据。

func TestPromptGenreListMatchesTheClosedTable(t *testing.T) {
	all := []string{
		genreArgument, genreReport, genreNarrative, genreExplain,
		genrePoem, genreClassical,
	}
	for _, g := range all {
		if !strings.Contains(prompts.ReadingPlanSystem, g) {
			t.Errorf("体裁 %q 不在排读法的提示词里 —— 模型输不出它，"+
				"这一体裁的读法、板、工具全都到不了学生那里", g)
		}
	}
	// 反方向：提示词里不许出现闭表之外的体裁名。
	// validateGenre 会把它丢掉，于是那一篇静默退回「认不出来」。
	//
	// 🚨 2026-09-24 起这里只剩书信一种。散文进了阅读闭表 —— 产品负责人：
	// 「this is very old... and we are about to do them now.」
	// 书信仍然不收：一封信不是一篇拿来读的文章。
	for _, notRead := range []string{genreLetter} {
		if strings.Contains(prompts.ReadingPlanSystem, `/ `+notRead) {
			t.Errorf("排读法的提示词里出现了写作面的体裁 %q", notRead)
		}
	}
}

// 🚨 光把名字列进去不够：还要说清楚**怎么判**。
//
// 《咏雪》讲的是谢安一家人的一件事 —— 按内容它就是 narrative。
// 把它判成 classical 的依据是**语体**（文言），不是内容。
// 少了这一句，模型会照内容判，而两种都会继续落到 narrative 上。
func TestPromptSaysToJudgePoemAndClassicalByForm(t *testing.T) {
	s := prompts.ReadingPlanSystem
	for _, want := range []string{"先看语体和形式", "不是 narrative"} {
		if !strings.Contains(s, want) {
			t.Errorf("提示词里没有「%s」—— 模型会照内容判，"+
				"一篇文言的故事会继续落到记叙文上", want)
		}
	}
}

// 每一种体裁都要有服务它的读法；反过来，每一套读法服务的体裁也都要在闭表里。
// 两个方向一起查 —— 任何一边漏了，那一体裁的学生拿到的都是别人的读法。
func TestEveryGenreAndRoutineLineUp(t *testing.T) {
	inTable := map[string]bool{}
	for _, g := range []string{
		genreArgument, genreReport, genreNarrative, genreExplain,
		genrePoem, genreClassical, genreProse,
	} {
		inTable[g] = true
		var served bool
		for _, r := range readingRoutines {
			if r.serves(g) {
				served = true
			}
		}
		if !served {
			t.Errorf("没有一套读法服务体裁 %q", g)
		}
	}
	for _, r := range readingRoutines {
		for _, g := range r.Genres {
			if !inTable[g] {
				t.Errorf("读法 %s 服务的体裁 %q 不在闭表里 —— validateGenre 会把它丢掉", r.Key, g)
			}
		}
	}
}
