package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

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

## 同一修改建议已连续出现两轮

先核对当前文字是否仍需这项修改；确有需要时，本轮提供两个不同的修改方法，帮助学生选择。

- 结合原文说明两种修改方法及各自用途，再问学生想采用哪一种。
- 每个选项说明具体修改对象与动作，不提供可直接复制的正文。
- 这一轮只给选项，不要再追问。`

	case helpShow:
		var b strings.Builder
		b.WriteString(`

## 同一修改建议已连续出现三轮

先核对当前文字是否仍需这项修改；确有需要时，本轮提供一句带空格的通用句式，由学生填写内容。

句式只表示内容之间的关系，保留待填写部分（「因为……，所以……」），
内容由学生填写，不代写段落。`)
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
