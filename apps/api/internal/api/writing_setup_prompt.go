package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"strings"

	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
)

const writingOpeningSystem = prompts.WritingOpeningSystem

func writingOpeningSystemFor(wr sqlc.Writing) string {
	if wr.Origin == "brought" {
		return writingBroughtOpeningSystem
	}
	if !writingIsAssigned(wr) {
		return writingOpeningSystem
	}
	return strings.NewReplacer(
		openingTopicOwn, openingTopicAssigned,
		openingRestateOwn, openingRestateAssigned,
	).Replace(writingOpeningSystem)
}

const writingBroughtOpeningSystem = prompts.WritingBroughtOpeningSystem

func buildBroughtOpeningPrompt(wr sqlc.Writing, body string) string {
	var b strings.Builder
	b.WriteString("题目：" + wr.Title + "\n")
	b.WriteString(writingLangLine(wr))
	if wr.TargetWords != nil {
		b.WriteString(writingLengthLine(wr, "学生定的目标篇幅"))
	}
	b.WriteString("\n【学生带来的全文】\n")
	body = strings.TrimSpace(body)
	if body == "" {
		b.WriteString("（正文是空的。）\n")
	} else {
		b.WriteString(cutRunes(body, broughtOpeningDraftRunes) + "\n")
	}
	return b.String()
}
func buildWritingOpeningPrompt(wr sqlc.Writing, msgs []sqlc.AtomMessage) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	b.WriteString(writingLangLine(wr))
	if wr.TargetWords != nil {
		b.WriteString(writingLengthLine(wr, "学生定的目标篇幅"))
	} else {
		b.WriteString("学生尚未设置篇幅，本轮无需询问。\n")
	}
	b.WriteString("\n【学生自己说过的话】\n")
	any := false
	for _, m := range msgs {
		if m.Role != "student" {
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString("- " + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（学生还没说什么，只有上面那个题目。）\n")
	}
	return b.String()
}
