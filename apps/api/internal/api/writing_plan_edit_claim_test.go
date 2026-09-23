package api

import (
	"strings"
	"testing"
)

// 产品负责人 2026-09-23 第 6 条：
//
//	「during writing, if we talked with AI that we want to change the central
//	  topic texts. the mindmap is not modified.」
//
// 🚨 这一条**不是「让印记去改图」**。规划这一路改不动任何既有节点，
// 那是 writing_plan.go 文件头的第一条硬规则（「只加，不改不删……那是她自己
// 的编辑权」），而且模型连节点 id 都拿不到。
//
// 真正坏掉的是：她和印记谈好了，然后什么都没发生。一句做不到的承诺比一句
// 「这个你自己在图上点一下就能改」糟得多。这几条钉的就是那句承诺不许出现。

func TestPlanReplyMayNotClaimItEditedTheMap(t *testing.T) {
	claiming := []string{
		"好，我已经把中心论点改成「读书要读慢」了。",
		"我帮你把那一条换成了「慢读才能发现问题」。",
		"已将中心论点更新为「读书应当放慢速度」。",
		"我来把它改成更准的一句。",
		"这条我替你改好了，已经修改为「慢下来才记得住」。",
		"好的，我把中心论点调整为「读书要读慢」。",
	}
	for _, reply := range claiming {
		if got := writingPlanClaimsAnEdit(reply); got == "" {
			t.Errorf("没抓到「我已经改好了」：%s", reply)
		}
	}
}

// 🚨 反方向和正方向一样重要：判错了会把**该说的话**判成失败，
// 而该说的话正是这次要它说的那一句
//（memory: detector-must-target-the-real-failure）。
func TestPlanReplyEditClaimLeavesHonestSentencesAlone(t *testing.T) {
	fine := []string{
		// 这正是这次希望它说的那一句 —— 把改法交给她。
		"你想换一句的话，在图上点一下「中心论点」那一条就能改。",
		"可以把中心论点改得更准一些：「读书要读慢」。",
		"要不要把它换成一句更具体的？",
		// 有「已经」，说的不是改图。
		"我已经记下你说的这一条了。",
		"你已经说清楚了三条理由。",
		// 有「改」，是请她改正文，不是改图。
		"这一段可以改成先写事再写你的判断。",
		// 空的。
		"",
	}
	for _, reply := range fine {
		if got := writingPlanClaimsAnEdit(reply); got != "" {
			t.Errorf("误伤了一句正常的话：%q → %q", reply, got)
		}
	}
}

// 🚨 按句判，不按整段判：一段里分别出现「已经」和「改成」多半是两件事。
func TestPlanEditClaimIsJudgedSentenceBySentence(t *testing.T) {
	reply := "我已经记下你说的三条理由了。\n你想让中心论点更准的话，可以把它改成一句更具体的说法。"
	if got := writingPlanClaimsAnEdit(reply); got != "" {
		t.Errorf("两句话被当成一句判了：%q", got)
	}
}

// 重问那一句要说**真正发生的那件事**，否则模型对不上号就原样再回一遍
//（memory: judge-the-delivered-artifact-2026-09-21）。
func TestEditClaimNudgeSaysWhatActuallyHappened(t *testing.T) {
	for _, want := range []string{"只能新增节点", "在图上点那一条就能改", "不说自己改过"} {
		if !strings.Contains(writingPlanEditClaimNudge, want) {
			t.Errorf("重问那一句里缺「%s」", want)
		}
	}
}
