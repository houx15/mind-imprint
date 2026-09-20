package api

import (
	"encoding/json"
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// ---------------------------------------------------------------------------
// 求助入口：一张卡片底下那颗「给点提示」
// ---------------------------------------------------------------------------

// coachAskHint 是卡片底下那颗按钮按下去说的话。前端同名常量：CoachCard.tsx 的
// COACH_ASKS。
//
// 🚨 只剩这一颗了。「示范一下」2026-09-20 被产品负责人撤掉，理由是它做的事本身
// 就不成立：「示范着就把答案示范出去了，而且在论文里不一定能找到第二个可以示范
// 的位置（很可能不准）」。系统说明里「给我看范例」仍然算明确索答 —— 她打字说出来
// 的时候照给，但不再摆一颗按钮请她按。
const coachAskHint = "给点提示"

// coachHintCap —— 同一张卡片上最多给几次提示。
//
// 产品负责人 2026-09-20：「建议每个卡片的『给点提示』，只限定交互三轮就变灰，
// 以免学生反复交互不走流程。」前端按同一个数把按钮置灰（CoachCard.tsx 的
// COACH_HINT_CAP），这里是她绕过按钮直接打字时的那一道。
const coachHintCap = 3

// isCoachHelpAsk —— 这一轮是不是她按了卡片底下那颗按钮。
func isCoachHelpAsk(s string) bool {
	return strings.TrimSpace(s) == coachAskHint
}

// coachHintRound —— 她这一轮按的是第几次提示，按**当前这张卡片**算（1 起）。
//
// 🚨 从后往前数到最近一条带卡片的 印记 消息为止：换了一张卡，梯子就从头开始。
// 不这么数的话，一篇文章读到后面每张新卡开局就是「最后一级提示」。
func coachHintRound(msgs []sqlc.AtomMessage) int {
	n := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role == "ai" {
			if len(m.Payload) > 0 {
				var p coachMessagePayload
				if err := json.Unmarshal(m.Payload, &p); err == nil && p.Card != nil {
					break
				}
			}
			continue
		}
		if m.Role == "student" && isCoachHelpAsk(m.Content) {
			n++
		}
	}
	return n + 1
}

// helpRequestSection —— 她按了「给点提示」的那一轮，多给模型一句怎么接。
//
// 系统说明里那道梯子说的是「每轮一级」，而模型看不见她按了几次 —— 实测它每次都
// 从第一级重新开始，于是同一张卡片上按三次提示拿到的是三句差不多的话。级数在这里
// 数出来（coachHintRound），直接写进这一轮的上文。
//
// 🚨 这一节只管**说什么**。「这一轮不许发新卡片、不许推进」那件事不靠这几句话
// 保证 —— 提示词里的软话跨不过代码里的硬判据，postReadingCoachTurn 里有一道闸把
// 这一轮的卡片和 advance 直接拿掉。
func helpRequestSection(studentText string, round int, open *coachCard) string {
	if !isCoachHelpAsk(studentText) {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n【她按了「给点提示」】\n")
	switch {
	case round <= 1:
		b.WriteString("这是这张卡片上的第 1 次提示：只给**方向** —— 这一步该往哪儿想、" +
			"该看哪一段，不要说出任何一句原文里的具体判断。\n")
	case round == 2:
		b.WriteString("这是这张卡片上的第 2 次提示：给**位置** —— 指到具体的一段" +
			"（或板上的某一张），说清楚要看的是它的哪个部分，仍然不给答案。\n")
	case round == coachHintCap:
		b.WriteString("这是这张卡片上的**最后一次**提示：给**局部线索** —— 指出那句话里的" +
			"一两个词，说它们为什么要紧，仍然不要把整句答案说出来。\n")
	default:
		b.WriteString("提示已经给满三次了，不要再给新的提示。用一句最具体的指令把这一步说清楚" +
			"（她要做的动作 + 第几段），然后停。\n")
	}
	// 🚨 「卡片还开着」只在**真的**开着的时候说。她也可能在没有卡片的时候打出
	// 这四个字 —— 那时这句话是假的，而模型会照着它去指一张屏幕上没有的卡
	// （[[full-loop-two-entrances-2026-09-08]] 那一类）。
	if open != nil {
		b.WriteString("🚨 她屏幕上那张卡片还开着，题目和她写了一半的草稿都留着 —— " +
			"**不要再发新卡片**，不要替她把题做掉，advance 留空。\n")
	} else {
		b.WriteString("🚨 这一轮不推进，advance 留空。\n")
	}
	if round >= coachHintCap && openCardIsOpenForm(open) {
		b.WriteString("🚨 她在这张卡片上按了三次提示还没动。这张卡要她自己写／自己回文章里去指，" +
			"这一轮可以换一个问法把同一件事变成一道选择题：card 的 type 用 choose_span，" +
			"问的仍然是**同一件事**，选项从文章里逐字抄 2 到 4 句（跨段落取）。" +
			"换了之后她原来那张卡会标成「已替换」，她答完这道选择题会回到原题。\n")
	}
	return b.String()
}

// openCardIsOpenForm —— 她屏幕上那张卡是不是「开放题」（要她自己写、或者自己回
// 文章里去指）。只有这两种才值得降成选择题：几句话摆在面前的卡片本来就是选择题，
// 再换一张只是把同一道题又问了一遍。
func openCardIsOpenForm(c *coachCard) bool {
	if c == nil {
		return false
	}
	return c.Type == coachCardShortText || c.Type == coachCardPickInArticle
}
