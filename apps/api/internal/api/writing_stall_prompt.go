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
//
// appliesTo 是她卡住的那一块在篇里的位置（vocab 的三个位置桶，由
// writingKindAppliesTo 从 kind 算出来）。它决定 helpShow 那一档摆哪几条句式。
func writingHelpModeBlock(mode writingHelpMode, appliesTo, lang, genre string) string {
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
		frames := writingHelpFrames(appliesTo, lang, genre)
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
//
// 🚨 按位置挑，走 vocab.For 而不是 vocab.ForLang。ForLang 不看位置，而每一条
// en_* 的 genre 都是空串（两种文体都用），所以文体那条轴在英文这边等于没有——
// 两件事叠起来的结果是：一个卡在议论文分论点上的学生，拿到的是
// `Opening inside a moment: At ___ , ___ was ___ .` 和
// `Landing the meaning: I still ___ . I know that ___ .`，整整 23 条，
// 而上面那句写着「这一轮给她一句句式」。
//
// 修的是选择器，不是数据：en_* 那几条的 genre 与块引导、深挖两条路共用，
// 在那两条路上它们本来就该两种文体都摆。
func writingHelpFrames(appliesTo, lang, genre string) string {
	var b strings.Builder
	for _, m := range vocab.For(appliesTo, lang, genre) {
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
