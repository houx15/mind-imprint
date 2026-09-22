package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

func writingPointsCheckBlock(wr sqlc.Writing, rows []sqlc.WritingOutline) string {
	off := writingPointsOffThesis(wr, rows)
	if len(off) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n【分论点与中心观点的文字匹配提示】\n")
	b.WriteString("以下句子与中心观点的关键词重合较少。这只是文字匹配结果，不能据此判断跑题。请结合上下文判断它们是否在解释中心观点；意思相关时无需重复关键词或改写。\n")
	for _, text := range off {
		b.WriteString("- 「" + text + "」\n")
	}
	b.WriteString("如果确实缺少逻辑联系，可请学生解释这条理由与中心观点的关系，不要求把指定词语塞进句子。计划已可开始写作或学生要求动笔时，不把措辞修改当作前置条件。\n")
	return b.String()
}
func writingPointAnglesBlock(wr sqlc.Writing, rows []sqlc.WritingOutline, need int) string {
	if wr.Lang != "zh" {
		return ""
	}
	var have int
	for _, row := range rows {
		if writingKindOf(row) == writingKindPoint && strings.TrimSpace(row.Text) != "" {
			have++
		}
	}
	if have >= need {
		return ""
	}
	return `
【当前还需要补充分论点】
可根据现有内容，从「是什么」「为什么」「怎么办」「会怎样」中选择一个相关角度帮助学生继续构思。
- 是什么：说明概念的含义或特点。
- 为什么：解释观点成立的理由。
- 怎么办：讨论可采取的行动。
- 会怎样：分析可能的结果。
本轮已选定补充分论点时，再参考已有分论点的组织方式，选择一个角度帮助学生构思相关且不重复的理由。当前仍在讨论其他内容时，将此提示留到后续。
`
}
