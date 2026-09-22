package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"strings"

	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

const deepenSystem = prompts.DeepenSystem

func buildDeepenBrief(wr sqlc.Writing, outline []sqlc.WritingOutline, block sqlc.WritingOutline, snippetText string, guideQuestions []string, studentText, piece string) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标篇幅"))
	if strings.TrimSpace(piece) != "" {
		b.WriteString(piece)
	} else {
		b.WriteString("\n【整篇的结构】\n")
		for _, s := range outline {
			indent := strings.Repeat("  ", int(s.Depth))
			role := writingKindLabel(writingKindOf(s), s.Source)
			if role == "" {
				role = "（未命名的块）"
			}
			line := indent + "- " + role
			if t := strings.TrimSpace(s.Text); t != "" {
				line += "：" + t
			} else {
				line += "：（还没写）"
			}
			if s.ID == block.ID {
				line += "   ← **学生现在停在这一块**"
			}
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n【学生现在停住的这一块】\n")
	b.WriteString("这一块的作用：" + writingKindLabel(writingKindOf(block), block.Source) + "\n")
	if t := strings.TrimSpace(block.Text); t != "" {
		b.WriteString("学生给这一块定的要点：" + t + "\n")
	} else {
		b.WriteString("学生还没给这一块定要点。\n")
	}
	if t := strings.TrimSpace(snippetText); t != "" {
		b.WriteString("学生已经写下的段落内容：\n" + t + "\n")
	} else {
		b.WriteString("这一段还是空的。\n")
	}
	if writingAsksUsToDoIt(studentText) {
		b.WriteString(writingRefusalBlock)
	}

	if len(guideQuestions) > 0 {
		b.WriteString("\n【已经给学生看过的引导问题——不要在这个对话里重复问这些】\n")
		for _, q := range guideQuestions {
			b.WriteString("- " + q + "\n")
		}
	}
	b.WriteString("\n【可用的方法】（示例须取自此表，使用其他题目的材料说明方法）\n")
	for _, m := range vocab.For(writingKindAppliesTo(writingKindOf(block)), wr.Lang, writingGenreOf(wr, outline)) {
		b.WriteString("- id=" + m.ID + " · " + m.Label() + "：" + m.Definition + "\n")
	}
	return b.String()
}
