package api

import (
	"strings"
	"testing"
)

// G21「材料不足时指出最小缺口或协助缩小主张。」
//
// 🚨 2026-09-24 查出来的：整个仓库里 `缩小主张` 和 `最小缺口` 一个字都没有。
// 原来只有一条出路 —— 「去查资料」。
//
// 这条为什么要紧：一个学生写「手机让中学生成绩下降」，手上只有自己的经历。
// 让她一直去找研究，是在教她拿材料去撑一句过大的话；而这句话真正的毛病是
// **它比她知道的范围大**。把话缩到她说得准的那一部分，是这里该教的另一件事。
func TestBothExitsAreOfferedWhenMaterialIsThin(t *testing.T) {
	for _, tc := range []struct {
		lang  string
		wants []string
	}{
		{"zh", []string{"最小缺口", "缩小主张", "限定人群"}},
		{"en", []string{"最小缺口", "缩小 thesis"}},
	} {
		t.Run(tc.lang, func(t *testing.T) {
			s := writingPlanSystemFor(genreArgument, tc.lang, "")
			for _, want := range tc.wants {
				if !strings.Contains(s, want) {
					t.Errorf("%s 议论文的提示词里没有 %q", tc.lang, want)
				}
			}
			// 两条路要**一起**给，不是二选一由印记替她定。
			if !strings.Contains(s, "由学生决定") {
				t.Errorf("%s：没写清改不改由学生决定", tc.lang)
			}
		})
	}
}

// 🚨 缩小主张和 hedging 不是一回事，英文那一份要把它们分开说。
//
// hedging（some / often / in my experience）改的是语气，缩小主张改的是
// 这句话管到哪里。混成一件事，学生会以为加个 some 就够了。
func TestNarrowingIsNotConfusedWithHedging(t *testing.T) {
	s := writingPlanSystemFor(genreArgument, "en", "")
	if !strings.Contains(s, "hedging") {
		t.Fatal("英文那份里没有 hedging —— 这条测试的前提没了")
	}
	if !strings.Contains(s, "不是一回事") {
		t.Error("没有把缩小 thesis 和 hedging 分开说")
	}
}

// 🚨 反方向，而且这一条是写 G21 的时候**被测试抓出来的**：
//
// SlotMaterial 原来只按语言登记、不按文体，所以每一种文体拿到的都是议论文那份
// 「根据观点选择材料」。议论文和记叙文大体用得上（记叙文也要挑经历），
// 但读后续写和概要写作是**错的** —— 续写的材料就是前文，概要根本没有自己的
// 材料。一个被告知「去找一条研究来支撑」的概要学生，照做就会写出一篇一定
// 扣分的概要。
//
// 所以这两档现在各有自己的材料那一节。这条测试钉的是「议论文那一套没有漏
// 过去」，以及「它们自己那一句在」。
func TestContinuationAndSummaryDoNotGetArgumentMaterialGuidance(t *testing.T) {
	for _, tc := range []struct {
		genre string
		want  string
	}{
		{genreContinuation, "这一篇的材料就是前文"},
		{genreSummary, "概要没有自己的材料"},
		// 2026-09-25 补：应用文的证据是细节与画面不是出处（源逐字
		// 「CRAAP 式溯源在这一面不适用」）；散文的材料是她自己看见的。
		{genreLetter, "这一档的材料是细节，不是出处"},
		{genreProse, "散文的材料是她自己看见的和想起来的"},
	} {
		for _, lang := range []string{"zh", "en"} {
			s := writingPlanSystemFor(tc.genre, lang, "")
			if !strings.Contains(s, tc.want) {
				t.Errorf("%s/%s：没装上自己那一节材料（找不到 %q）", tc.genre, lang, tc.want)
			}
			for _, leaked := range []string{"缩小主张", "缩小 thesis", "根据观点选择材料"} {
				if strings.Contains(s, leaked) {
					t.Errorf("%s/%s：漏进了议论文的材料指导 %q", tc.genre, lang, leaked)
				}
			}
		}
	}
}

// 议论文和记叙文仍然拿着那一份 —— 这次没有顺手改它们。
// 记叙文也要挑经历，「找材料」这件事在它身上是成立的。
func TestArgumentAndNarrativeKeepTheSharedMaterialBlock(t *testing.T) {
	for _, genre := range []string{genreArgument, genreNarrative} {
		if s := writingPlanSystemFor(genre, "zh", ""); !strings.Contains(s, "根据观点选择材料") {
			t.Errorf("%s 的材料那一节丢了", genre)
		}
	}
}
