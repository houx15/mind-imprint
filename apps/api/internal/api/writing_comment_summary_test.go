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

// 🚨 2026-09-21 线上走查：真模型绕过了那张词表。
//
// 通篇审阅回来的总评是「这两段各写了一边的事，但通篇**找不到**一句是你自己的
// 判断」—— 说的和「缺少一个核心论点」是同一件事，一个标记都没踩上。
func TestSummaryAbsence_CatchesTheSynonymsTheModelActuallyUsed(t *testing.T) {
	for _, s := range []string{
		"这两段各写了一边的事，但通篇找不到一句是你自己的判断。",
		"全文看不到一个属于你自己的判断。",
		"这两段里没有一句是你的主张。",
	} {
		if m := summaryClaimsAbsence(s); m == "" {
			t.Errorf("没抓住这句在说「她缺了什么」：%q", s)
		}
	}
	// 🚨 光秃秃的「没有」不收 —— 「这一段没有问题」是句好话，
	// 收了它每一轮都要多跑一次模型。
	for _, s := range []string{
		"这一段没有问题，可以往下走了。",
		"观点句和材料句都齐了，这一段站得住。",
	} {
		if m := summaryClaimsAbsence(s); m != "" {
			t.Errorf("这句是好话，不该触发重试（抓到 %q）：%q", m, s)
		}
	}
}

// 说了这篇有问题，却一条 point 都不给。
//
// 这条比词表硬：它不问那句话是怎么写的，只问「你说有问题，问题在哪句」。
func TestVerdictWithoutAnyPointIsIncoherent(t *testing.T) {
	none := []CommentPoint{}
	onlyGood := []CommentPoint{{Kind: "good", Text: "这个例子具体。"}}
	withIssue := []CommentPoint{{Kind: "issue", Symptom: "claim_not_stated", Text: "…", Action: "…"}}

	if writingHasIssue(none) {
		t.Error("空的 points 不该算「有一条 issue」")
	}
	if writingHasIssue(onlyGood) {
		t.Error("只有一条肯定不该算「有一条 issue」")
	}
	if !writingHasIssue(withIssue) {
		t.Error("有一条 issue 却没认出来")
	}
}
