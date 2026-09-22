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
		if lang == "zh" {
			frames := writingHelpFrames(genre)
			if frames != "" {
				b.WriteString("\n\n可以给的句式：\n")
				b.WriteString(frames)
			}
		}
		return b.String()
	}
	return ""
}

// writingHelpFrames 把库里带句式的那几条摆出来，供 helpShow 那一档引用。
func writingHelpFrames(genre string) string {
	var b strings.Builder
	for _, m := range vocab.ForLang("zh", genre) {
		for _, p := range m.Patterns {
			b.WriteString("- ")
			b.WriteString(m.Name)
			b.WriteString("：")
			b.WriteString(p.Frame)
			b.WriteString("\n")
		}
	}
	return b.String()
}
