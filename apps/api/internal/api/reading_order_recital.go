package api

import "strings"

// reading_order_recital.go —— 发排序板的那一轮，印记不能先把顺序背一遍。
//
// # 为什么存在（2026-09-23，产品负责人第 4 条的另一半）
//
// 她截的那张图，板子上面那一句是：
//
//	接下来做「排出事件时间线」：这件事先后经过发现画作、挂在家中、
//	网友鉴定、拍卖成交几个环节。请把这几件事按发生的先后排在板子上。
//
// 板子打乱了也没用 —— 正确答案已经写在板子上面那一句话里，她照抄就行。
// 乱序（reading_order_shuffle.go）治的是板子，这一条治的是嘴。
//
// # 为什么不是再写一句提示词
//
// 提示词里**已经写了两处**：readingCoachStepRule 的 taskSequence 分支说
// 「给板的那一轮不要说出正确的先后」，reading_genre.go 给体裁的那一段又说了
// 一遍「给板的那一轮不要把正确的先后说出来」。两处都写了，模型照样说。
// memory `prompt-twice-then-make-it-checkable-2026-09-12`：同一条毛病提示词改过
// 两次还在，就停下来找可验判据，别写第三段。这就是那条判据。
//
// # 判据：一句话里同时有「按顺序」的说法和三件以上并列的事
//
// 收得紧，因为它只在**要发排序板的那一轮**查：
//   - 同一句里至少两个「、」（三件以上的事并排），并且
//   - 有一个排顺序的说法（先后 / 依次 / 分别是 / 顺序是 …）。
//
// 只占一样的句子一律放过：
//   - 「报道常常先讲结果，再回头交代经过」—— 有说法，没有并列；
//   - 「这件事牵涉到记者、警方、家属三方」—— 有并列，没有排顺序的说法，
//     它讲的是有谁，不是先后。
//
// 🚨 **只在给板那一轮查。** 她排完之后那一轮讲「原文里的先后依据」是提示词
// 明确要求的教学反馈，在那里查会把做对的事判成漏题
//（memory `detector-must-target-the-real-failure`：判据要盯住真失败，
// 不是它的影子）。

var orderRecitalCues = []string{
	"先后", "依次", "分别是", "分别为", "顺序是", "顺序为",
	"按顺序", "先是", "经过", "流程是", "过程是",
}

// shipsOrderBoard —— 这一轮是不是要把排序板交到她手上。
//
// 两种都算：模型自己给了一块，或者服务端接下来会补一块（needOrderBoard，
// 见 reading_coach.go 里那段「第一块排序板必须真的到她屏幕上」）。补的那一种
// 同样要查，因为说漏嘴的是**这一轮的话**，和板子由谁摆出来无关。
func shipsOrderBoard(p readingCoachReply, needOrderBoard bool) bool {
	if needOrderBoard {
		return true
	}
	return p.Card != nil && p.Card.Type == coachCardOrderEvents
}

// firstOrderAnswerRecital 返回回复里第一句背出顺序的话，没有就返回 ""。
func firstOrderAnswerRecital(reply string) string {
	for _, line := range strings.Split(reply, "\n") {
		for _, sent := range splitCJKSentences(line) {
			if orderSentenceRecitesOrder(sent) {
				return strings.TrimSpace(sent)
			}
		}
	}
	return ""
}

func orderSentenceRecitesOrder(sent string) bool {
	if strings.Count(sent, "、") >= 2 {
		for _, cue := range orderRecitalCues {
			if strings.Contains(sent, cue) {
				return true
			}
		}
	}
	// 英文那边的同一件事：first … then … finally 三个词落在同一句里，
	// 那就是在把顺序念出来。少一个词就放过 —— 「first」单独出现太常见。
	lower := strings.ToLower(sent)
	if strings.Contains(lower, "first") && strings.Contains(lower, "then") &&
		(strings.Contains(lower, "finally") || strings.Contains(lower, "last")) {
		return true
	}
	return false
}

// stripOrderAnswerRecital 把背出顺序的那一句拿掉。
//
// 和 stripProtocolLeak 同一条路：重问过一次还在，就拿掉那一句，而不是丢掉整轮
//（memory `detector-must-target-the-real-failure`：不变量要保住，但她不该看见
// 一个死掉的终端）。拿掉之后剩下的「请把这几件事按发生的先后排在板子上」自己
// 站得住 —— 事情本来就都在板子上，不必在话里再数一遍。
func stripOrderAnswerRecital(reply string) string {
	if firstOrderAnswerRecital(reply) == "" {
		return reply
	}
	lines := strings.Split(reply, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		kept := make([]string, 0, 4)
		for _, sent := range splitCJKSentences(line) {
			if !orderSentenceRecitesOrder(sent) {
				kept = append(kept, sent)
			}
		}
		joined := strings.TrimSpace(strings.Join(kept, ""))
		// 整行只剩空的就整行去掉，但不要留下一个空行。
		if joined == "" && strings.TrimSpace(line) != "" {
			continue
		}
		out = append(out, joined)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
