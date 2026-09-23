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
		// 🚨 分论点够了，这一栏就换成**下一件该谈的事**，而不是消失。
		//
		// 产品负责人 2026-09-23 第 7 条：「our AI always puts all students'
		// talked points as one 分论点, but actually some of them should be,
		// e.g. ending, they would be different part of the article……
		// the discussion is also not focused.」
		//
		// 她说的两件事在这里是同一件：这一栏在分论点不够的时候每一轮都在推
		// 「再来一条分论点」，够了之后**什么都不说**，而开始写作的条件里又写着
		// 「开头与结尾尚未确定，不单独作为不能开始写作的理由」——
		// 于是结尾从头到尾没有一处请模型谈它，学生说的每一句都只能落成分论点。
		return writingClosingAnglesBlock(rows)
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

// writingClosingAnglesBlock —— 分论点够了之后那一栏。
//
// 图上已经有结尾了就什么都不说：这一栏的作用是**点名下一件该谈的事**，
// 不是每一轮都提醒她结尾的存在。
func writingClosingAnglesBlock(rows []sqlc.WritingOutline) string {
	for _, row := range rows {
		if writingKindOf(row) == writingKindClosing && strings.TrimSpace(row.Text) != "" {
			return ""
		}
	}
	return `
【分论点已经够了，下一件可以谈结尾】
结尾是文章里和分论点并列的一块，不是又一条理由。学生这一轮说的话是在收束
全篇时，kind 用 closing，不要当成分论点加到中间。
需要谈结尾时，从这几个角度里挑一个：回到中心论点并把它说得比开头更准、
说明读者读完应该带走的判断、指出这件事接下来可以从哪里做起。
学生这一轮在谈别的内容时，把这一条留到后面。
`
}
