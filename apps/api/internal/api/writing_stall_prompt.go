package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"strings"

	"mindimprint/api/internal/vocab"
)

func writingHelpModeBlock(mode writingHelpMode, lang, genre string) string {
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
