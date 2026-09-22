package api

// Prompt assembly for writing_points_check.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"strings"

	"mindimprint/api/internal/store/sqlc"
)

// writingPointsCheckBlock 是加进立题 prompt 的那一段。
//
// 空集就返回空串 —— 见文件头那段「数出来是空的就一个字都不加」。
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

// writingPointAnglesBlock —— 拟写分论点的四个角度。
//
// 讲义（四）第三节：并列式议论文的分论点，从「是什么 / 为什么 /
// 怎么样（怎么办）/ 会怎样」四个角度里**选定一个**，整篇用同一个角度。
//
// 🚨 只在她的分论点还不够的时候加。够了之后再摆一张「可以从哪几个角度想」
// 的表，是在她已经想好之后教她怎么想 —— 那一轮该做的是别的事。
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
【她还要再想一条分论点，这四个角度里选一个】
一篇文章的几条分论点最好都从**同一个角度**切进去，读者才觉得它们是一套的：

- **是什么**：这个词到底指什么。（诗意地栖居，是远离喧嚣独自成长）
- **为什么**：为什么该这样。（诗意地栖居，可以让我们的心飞得更高）
- **怎么办**：要做到得怎么做。（诗意地栖居，需要我们积极乐观地面对生活）
- **会怎样**：这样做了会带来什么。（这样做的人，日子会变成什么样）

看她已经写的那几条是从哪个角度切的，请她照着同一个角度再想一条。
🚨 一次只问一个问题，不要把四个角度都摆给她让她挑。
`
}
