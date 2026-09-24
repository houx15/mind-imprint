package api

// writing_chinglish_live_test.go —— 中式英语那一节发给真模型跑一遍。
//
// 跑法：
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveChinglish -v -count=1
//
// 🚨 为什么要用真模型：源里把这一类称作「批改最增值的类别」，而它的价值全在
// **讲透**那一步 —— 说清英语和汉语在这一处的规则差别。一句「不够地道」
// 在数据上和一句讲透了的解释长得一模一样（都是一条 issue、都有 quote、
// 都有 action），离线分不开。
//
// 判据只钉一件事：**她那句照中文直译的英文被认出来了**。挑哪一句、
// 怎么讲是模型的判断，而「整段扫过去一句都没认出来」才是真失败
// （[[detector-must-target-the-real-failure]]）。

import (
	"strings"
	"testing"
)

// 一段照中文语序拼出来的英文，四处典型的中式英语各占一句：
//
//	I very like …        → I like … very much（程度副词的位置）
//	open the light       → turn on the light（搭配）
//	learn knowledge      → acquire / gain knowledge（搭配）
//	Although … but …     → 两个连词只能留一个（英语不双用）
const liveChinglishParagraph = `I very like our school library. Although it is small, but I go there every day.
When I arrive, I open the light and sit near the window. I can learn many knowledge there.
Welcome to here if you want a quiet place.`

func TestLiveChinglishIsNamedNotJustLabelled(t *testing.T) {
	points := oneCommentTurn(t, "en", liveChinglishParagraph)
	if len(points) == 0 {
		t.Fatal("一条都没活下来 —— 英文这一侧等于没有反馈")
	}

	all := ""
	for _, p := range points {
		all += p.Quote + "\n" + p.Text + "\n" + p.Action + "\n"
	}

	// 至少认出其中一处。四句里任意一处的**正确说法**或者那句原话被点到都算。
	hit := false
	for _, w := range []string{
		"very much", "turn on", "acquire", "gain knowledge",
		"Although", "although", "welcome here", "Welcome",
		"I very like", "open the light", "learn many knowledge",
	} {
		if strings.Contains(all, w) {
			hit = true
			break
		}
	}
	if !hit {
		t.Errorf("四处典型的中式英语一处都没认出来：\n%s", all)
	}

	// 🚨 反方向：认出来了但只说「不地道」，等于没讲。
	// 源里的讲解要点逐字是「指出**英汉差异的具体规则**，而不是只说『不地道』」。
	for _, lazy := range []string{"不够地道", "不地道", "not idiomatic"} {
		for _, p := range points {
			if strings.Contains(p.Text, lazy) && len([]rune(p.Text)) < 30 {
				t.Errorf("只说了一句 %q 就完了，没讲规则：%s", lazy, p.Text)
			}
		}
	}
}
