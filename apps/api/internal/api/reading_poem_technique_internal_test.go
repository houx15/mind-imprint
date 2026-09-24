package api

import (
	"strings"
	"testing"
)

// 诗歌：手法的边界，和「评价的尺子不通用」。
//
// 两条都来自产品负责人 2026-09-24 贴的那份诗歌鉴赏框架，而语料里**没有**：
// distilled/poetry-reading.md §7.1 逐字记着「源里没有任何一句区分它们」，
// 而且量过一份语料里「对比」出现 32 次、「烘托」6 次、「衬托」3 次，
// **混着用，没有定义**。
//
// 「体裁标准不通用」那一条同一份文件标为「O 独有且最容易被实现漏掉的一条」。
func TestPoemCoachSeparatesTheFourTechniques(t *testing.T) {
	s := buildGenreCoachSection(genrePoem)
	if s == "" {
		t.Fatal("诗词没有带读说明")
	}
	// 四个词都要出现，而且各自带着自己的判据。
	for word, mark := range map[string]string{
		"渲染": "只用于写景",
		"烘托": "正衬",
		"衬托": "反衬",
		"对比": "不分主次",
	} {
		if !strings.Contains(s, word) {
			t.Errorf("四个手法里少了 %q", word)
			continue
		}
		if !strings.Contains(s, mark) {
			t.Errorf("%q 没有给出判据（找不到 %q）", word, mark)
		}
	}
	// 🚨 最要紧的一条：说不准的时候**不要硬安名字**。
	//
	// 少了它，这一节只会让模型更熟练地贴标签 —— 而这一段本来的骨头是
	// 「为什么好比用了什么重要」，硬套名字正好把那条骨头抽掉。
	if !strings.Contains(s, "不要硬安一个名字") {
		t.Error("没有拦住「说不准也要安一个手法名」")
	}
}

// 评价的尺子跟着诗体走。
//
// 🚨 这和「按诗体看安排」不是一回事：那一条说的是**分析重点**
// （绝句看起承转合、律诗看中间两联），这一条说的是**评价标准**
// —— 同一处写法在绝句里是长处，在律诗里可能就是短处。
func TestPoemCoachSaysTheStandardItselfDiffers(t *testing.T) {
	s := buildGenreCoachSection(genrePoem)
	for _, want := range []string{"绝句贵含蓄", "律诗以工整为上"} {
		if !strings.Contains(s, want) {
			t.Errorf("没有说清体裁标准不通用（找不到 %q）", want)
		}
	}
	// 已有的那条「分析重点」不许被这次改动顶掉。
	if !strings.Contains(s, "起承转合") || !strings.Contains(s, "中间两联") {
		t.Error("按诗体看安排那一条被改没了")
	}
}

// 反方向：这两条只在诗词上，别的体裁不许拿到。
//
// 一篇记叙文的带读说明里冒出「绝句贵含蓄」，比不给更糟。
func TestTechniqueBoundariesStayOnPoems(t *testing.T) {
	for _, genre := range []string{genreNarrative, genreReport, genreExplain, genreClassical, genreProse, genreArgument, ""} {
		s := buildGenreCoachSection(genre)
		for _, leaked := range []string{"绝句贵含蓄", "只用于写景"} {
			if strings.Contains(s, leaked) {
				t.Errorf("体裁 %q 上漏进了诗词的 %q", genre, leaked)
			}
		}
	}
}
