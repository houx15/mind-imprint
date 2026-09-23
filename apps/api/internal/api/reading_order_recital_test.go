package api

import "strings"

import "testing"

// 产品负责人 2026-09-23 第 4 条的另一半：发板那一轮先把顺序背了一遍。
// 她截图里那一句逐字在下面第一条。

func TestOrderRecitalCatchesTheScreenshot(t *testing.T) {
	leaky := []string{
		"接下来做「排出事件时间线」：这件事先后经过发现画作、挂在家中、网友鉴定、拍卖成交几个环节。请把这几件事按发生的先后排在板子上。",
		"这几件事依次是买下画、挂上墙、被人认出、送去拍卖。",
		"事情的顺序是：他先动手、随后后悔、最后道歉。",
		"分别是发现、犹豫、决定三步。",
		"The story moves first through the discovery, then through the doubt, and finally to the sale.",
	}
	for _, reply := range leaky {
		if got := firstOrderAnswerRecital(reply); got == "" {
			t.Errorf("没抓到背顺序：%s", reply)
		}
	}
}

// 🚨 反方向的用例和正方向一样重要 —— 判错了会把做对的事判成漏题
// （memory: detector-must-target-the-real-failure）。
func TestOrderRecitalLeavesHonestSentencesAlone(t *testing.T) {
	clean := []string{
		// 有排序的说法，没有并列 —— 这是提示词要求的教学反馈本身。
		"报道常常先讲结果，再回头交代经过。",
		"请把这几件事按发生的先后排在板子上。",
		// 有并列，没有排序的说法 —— 它讲的是有谁，不是先后。
		"这件事牵涉到记者、警方、家属三方。",
		"文中出现了动作、神态、语言三种描写。",
		// 单个 first 很常见，不该算。
		"Look at the first paragraph and tell me what the writer notices.",
		"First, read the whole piece once.",
		// 空的。
		"",
	}
	for _, reply := range clean {
		if got := firstOrderAnswerRecital(reply); got != "" {
			t.Errorf("误伤了一句正常的话：%q → %q", reply, got)
		}
	}
}

// 拿掉那一句之后，剩下的话要自己站得住 —— 她不该看见一个空回复。
func TestStripOrderRecitalKeepsTheInstruction(t *testing.T) {
	reply := "接下来做「排出事件时间线」：这件事先后经过发现画作、挂在家中、网友鉴定、拍卖成交几个环节。请把这几件事按发生的先后排在板子上。"
	got := stripOrderAnswerRecital(reply)
	if got == "" {
		t.Fatal("整句都没了")
	}
	if firstOrderAnswerRecital(got) != "" {
		t.Errorf("拿掉之后还在：%q", got)
	}
	if !strings.Contains(got, "排在板子上") {
		t.Errorf("把该留的那一句也拿掉了：%q", got)
	}
	// 没有毛病的回复原样返回。
	clean := "请把这几件事按发生的先后排在板子上。"
	if stripOrderAnswerRecital(clean) != clean {
		t.Error("干净的回复被动了")
	}
}

// 只在发板那一轮查。她排完之后讲先后依据是提示词要求的事，在那里查就是判错。
func TestRecitalOnlyCheckedOnTheTurnThatShipsTheBoard(t *testing.T) {
	withBoard := readingCoachReply{Card: &coachCard{Type: coachCardOrderEvents}}
	if !shipsOrderBoard(withBoard, false) {
		t.Error("模型给了排序板，该算发板那一轮")
	}
	if !shipsOrderBoard(readingCoachReply{}, true) {
		t.Error("服务端接下来要补一块，也该算发板那一轮")
	}
	// 反馈那一轮：没有板，服务端也不打算补。
	feedback := readingCoachReply{Card: &coachCard{Type: coachCardShortText}}
	if shipsOrderBoard(feedback, false) {
		t.Error("反馈那一轮不该被查")
	}
	if shipsOrderBoard(readingCoachReply{}, false) {
		t.Error("没有板的一轮不该被查")
	}
}
