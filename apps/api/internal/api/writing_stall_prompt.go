package api

// Prompt assembly for writing_stall.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"strings"

	"mindimprint/api/internal/vocab"
)

// writingHelpModeBlock 是加进 prompt 的那一段。
//
// 🚨 helpAsk 返回空串 —— **不做常驻**。2026-09-05 的教训：把这类提示做成
// 每轮都在，模型会一直去处理那条提示、把该做的事挤掉。
func writingHelpModeBlock(mode writingHelpMode, lang, genre string) string {
	switch mode {
	case helpOffer:
		return `

## 🚨 这一处已经说过两轮了，她没有改

再问一遍同一个问题不会有别的结果。**这一轮改成给她两个选项。**

- 把「你觉得这里该补什么」换成「这里有两条路：A……，B……。你想走哪一条？」
- 两个选项都要具体到她可以直接照着写，不要是「补充论据」这种说法。
- 这一轮只给选项，不要再追问。`

	case helpShow:
		var b strings.Builder
		b.WriteString(`

## 🚨 这一处说过三轮了，换选项她也没动

**这一轮给她一句句式**，让她照着填。

🚨 句式不是替她写正文：你给的是带着空格的骨架（「因为……，所以……」），
填什么由她定。绝不要把她那一段替她写出来。`)
		frames := writingHelpFrames(lang, genre)
		if frames != "" {
			b.WriteString("\n\n可以给的句式：\n")
			b.WriteString(frames)
		}
		return b.String()
	}
	return ""
}

// writingHelpFrames 把库里带句式的那几条摆出来，供 helpShow 那一档引用。
//
// 🚨 lang 是参数，不是写死的。2026-09-23 之前这里写死 "zh"，而调用点外面
// 还套着一道 `if lang == "zh"` —— 两处叠在一起，写英文的学生在这条路上
// 一条句式都拿不到，而库里给她备着 23 条。
func writingHelpFrames(lang, genre string) string {
	var b strings.Builder
	for _, m := range vocab.ForLang(lang, genre) {
		for _, p := range m.Patterns {
			b.WriteString("- ")
			b.WriteString(m.Name)
			b.WriteString("：")
			b.WriteString(p.Frame)
			if p.Gloss != "" {
				b.WriteString("（")
				b.WriteString(p.Gloss)
				b.WriteString("）")
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
