package api

import (
	"strings"
	"testing"
)

// 🚨 产品负责人 2026-09-20 报的第 2 条，附截图：卡片标题写着「哪两句分别给出了
// 这两个关键词？」，底下四个选项，点一句就交上去了 —— 题目要两句，卡片只收一句。
// 她怎么点都是错的。
func TestCardCannotAskForTwoAnswersOnASinglePickCard(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "我确信敬业乐业四个字，是人类生活的不二法门。"},
		{ID: "b2", Text: "但必先有业，才有可敬、可乐的主体，理至易明。"},
	}
	opts := []coachCardOption{
		{BlockID: "b1", Quote: "我确信敬业乐业四个字，是人类生活的不二法门。"},
		{BlockID: "b2", Quote: "但必先有业，才有可敬、可乐的主体，理至易明。"},
	}
	for _, prompt := range []string{
		"哪两句分别给出了这两个关键词？",
		"这两个关键词各自出现在哪一句？",
		"哪几句在说作者自己的判断？",
	} {
		card, why := validateCoachCardWhy(&coachCard{Type: coachCardChooseSpan, Prompt: prompt, Options: opts}, blocks)
		if card != nil || why != cardRejectAsksMultiple {
			t.Errorf("%q 应该被退回重问，得到 card=%+v why=%q", prompt, card, why)
		}
	}
	// 只问一处的题照常通过。
	card, why := validateCoachCardWhy(&coachCard{
		Type: coachCardChooseSpan, Prompt: "哪一句是作者自己的判断？", Options: opts,
	}, blocks)
	if card == nil || why != cardOK {
		t.Fatalf("只问一句的题被拦下了：why=%q", why)
	}
	// 🚨 板不在此列：一块板上每一张都要摆，「分别属于哪一类」正是它要问的。
	if _, why := validateCoachCardWhy(&coachCard{
		Type: coachCardLabelRoles, Prompt: "下面这几句分别属于哪个角色？", Options: opts,
	}, blocks); why == cardRejectAsksMultiple {
		t.Error("一块板被当成了只收一个答案的卡片")
	}
	// short_text 也不在：她自己打字，一句里写两处是她的自由。
	if _, why := validateCoachCardWhy(&coachCard{
		Type: coachCardShortText, Prompt: "这两个关键词分别是什么意思？",
	}, blocks); why == cardRejectAsksMultiple {
		t.Error("要她自己写的一道题被当成了单选")
	}
	// 退回去的时候要告诉它怎么改，光给理由不够（cardFixIt 顶上那段）。
	if fix := cardFixIt[cardRejectAsksMultiple]; !strings.Contains(fix, "一句") {
		t.Errorf("没有给模型一句能照着改的话：%q", fix)
	}
	// 这一条要走重来一次那条路 —— 不然她这一轮屏幕上一张卡都没有。
	if !readingCoachReplyNeedsRetry(readingCoachReply{cardWhy: cardRejectAsksMultiple}) {
		t.Error("题目装不下的那张卡没有触发重来一次")
	}
}
