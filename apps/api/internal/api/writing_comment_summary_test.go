package api

import "testing"

// 线上真的那两句总评，一字不改地摆在这儿。
//
// 两句都来自 2026-09-12 第二十八轮走查，中文那一路的两次通篇审阅
// （writing_comment.source_text 分别是她 529 字和 818 字那一版）。
// 第二句是假的：她那一版正文里就写着那个问句。她连着两步在说这件事。
func TestSummaryClaimsAbsence_CatchesTheTwoRealOnes(t *testing.T) {
	for _, s := range []string{
		"观察很具体，但文章停在了“告诉大家有浪费”这一步，缺少对素材的分析和解释。",
		"你收集了很多具体事例，但全文缺少一个核心问题来统领这些材料。",
	} {
		if got := summaryClaimsAbsence(s); got == "" {
			t.Errorf("没抓到这句总评里的「缺失」claim：%s", s)
		}
	}
}

// 说这篇稿子现在在哪儿的总评，照常放过。
func TestSummaryClaimsAbsence_LetsThroughAJudgementWithoutAnAbsenceClaim(t *testing.T) {
	for _, s := range []string{
		"这一稿已经立住了一个判断，底下三处现场观察都在替它说话。",
		"你把两个人的餐盘摆在一起比，差别本身就把话说清楚了。",
		"This draft has one clear claim, and the canteen scene carries it.",
	} {
		if got := summaryClaimsAbsence(s); got != "" {
			t.Errorf("把一句正常的总评判成了「缺失」claim（命中 %q）：%s", got, s)
		}
	}
}

// 英文那一路同样要认 —— 一篇英文稿子拿到的是 IELTS 那张表，总评也是英文的。
func TestSummaryClaimsAbsence_English(t *testing.T) {
	for _, s := range []string{
		"Your observations are vivid, but the piece lacks a central question.",
		"The examples are concrete; what is missing is an explanation of why they matter.",
		"Strong detail throughout, though it fails to connect the two scenes.",
	} {
		if got := summaryClaimsAbsence(s); got == "" {
			t.Errorf("英文总评里的「缺失」claim 没抓到：%s", s)
		}
	}
}
