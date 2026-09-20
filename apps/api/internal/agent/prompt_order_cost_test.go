package agent

import (
	"strings"
	"testing"
)

// 块的顺序在这里是**价格**，不是可读性。通道按公共前缀打折（命中部分按输入价的
// 20%），所以每轮都变的东西一旦排到前面，排在它后面的所有内容就都变成全价。
//
// 实测 2026-09-20（qwen3.8-flash，4,800 token 的真实尺寸，连着两轮）：
// 原来的排法「房间 → 对话 → 状态投影」命中 **0%**；
// 改成「状态投影 → 对话 → 房间」命中 **85%**。
//
// 这条测试守的就是那 85%。读代码看不出这件事 —— 把两段话换个位置，
// 功能一模一样，测试全绿，只是每一轮悄悄贵了一倍。
func TestProjectCoachContextPutsStableBlocksFirst(t *testing.T) {
	history := []ChatTurn{
		{Role: "user", Content: "我写完第二段了"},
		{Role: "assistant", Content: "第二段里那句话是你的理由还是你的例子？"},
	}
	projection := "题目/想法：人工智能会不会让人变懒\n正文：她写的那一大段字。"
	got := BuildProjectCoachContext(history, projection, "段落")

	iProj := strings.Index(got, "项目当前状态")
	iHist := strings.Index(got, "对话（从旧到新）")
	iSurface := strings.Index(got, "学生现在在「")

	for name, idx := range map[string]int{
		"projection": iProj, "history": iHist, "surface": iSurface,
	} {
		if idx < 0 {
			t.Fatalf("%s block missing from the context entirely", name)
		}
	}

	// 投影里有她的正文，是这段 prompt 里最大也最稳的一块 —— 它必须排在
	// 滑动窗口（对话）前面。对话每多两轮就整体错位一次，排在它后面的东西
	// 每一轮都要重新全价买一遍。
	if iProj > iHist {
		t.Errorf("projection (carries her draft) must come BEFORE the sliding "+
			"history window, or her whole draft is re-billed at full price every "+
			"turn. projection@%d history@%d", iProj, iHist)
	}
	// 房间名每次她换页就变，属于易变块，排最后。
	if iSurface < iHist {
		t.Errorf("the active-surface line changes whenever she moves rooms, so it "+
			"belongs after the history, not before it. surface@%d history@%d",
			iSurface, iHist)
	}
}
