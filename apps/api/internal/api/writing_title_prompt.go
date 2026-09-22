package api

// Writing prompt assembly. These functions render selected context without database or model calls.
// Preserve context selection and the order of stable and changing prompt sections.

import (
	"strings"

	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
)

const writingTitleKeywordsSystem = prompts.WritingTitleKeywordsSystem

func buildWritingTitleKeywordsPrompt(wr sqlc.Writing, draft string) string {
	var b strings.Builder
	b.WriteString(writingLangLine(wr))
	b.WriteString("学生写完的文章：\n" + strings.TrimSpace(draft) + "\n")
	return b.String()
}
