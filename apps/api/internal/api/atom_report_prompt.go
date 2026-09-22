package api

// Prompt assembly for atom_report.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"

	"strings"

)

// liteReportSystem — 印记 writing to the student about her own session. No
// score, no grade, no rank, no comparison to anyone, no praise inflation
// (铁律②): this is a record of what she did, not a verdict on it. Voice per
// the standing rule — real specifics, never a clipped AI-shrug line.
const liteReportSystem = prompts.LiteReportSystem

// buildReportPrompt hands the model the one thing it is allowed to draw
// moments from: corpus.Text, exactly as report_facts.go assembled it (her
// own words only, in fragment order). Nothing from the article, nothing
// AI-authored, is reachable here — see report_facts.go's file comment.
// `turns` 是编号过的对话，只用来挑编号 —— 它**不进 corpus**，所以它里面的
// 句子仍然无法成为金句：validateMoments 验的是 corpus.Text，而那里面只有她
// 自己写下的材料。这是 R4 那道墙没有被这次改动碰到的原因。
func buildReportPrompt(kind, title, turns string, corpus reportCorpus) string {
	var b strings.Builder
	b.WriteString("类型：")
	if kind == "writing" {
		b.WriteString("写作")
	} else {
		b.WriteString("阅读")
	}
	b.WriteString("\n")
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("题目：" + t + "\n")
	}
	b.WriteString("\n【她自己写下的材料】\n")
	b.WriteString(corpus.Text)
	b.WriteString("\n")
	if strings.TrimSpace(turns) != "" {
		b.WriteString("\n【对话记录（只用来挑编号，不要从这里引句子）】\n")
		b.WriteString(turns)
		b.WriteString("\n")
	}
	return b.String()
}
