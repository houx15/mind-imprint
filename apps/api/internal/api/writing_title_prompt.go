package api

// Prompt assembly for writing_title.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"strings"

	"mindimprint/api/internal/prompts"
	"mindimprint/api/internal/store/sqlc"
)

// writingTitleKeywordsSystem asks for keywords — words she already wrote —
// and nothing else.
const writingTitleKeywordsSystem = prompts.WritingTitleKeywordsSystem

func buildWritingTitleKeywordsPrompt(wr sqlc.Writing, draft string) string {
	var b strings.Builder
	b.WriteString(writingLangLine(wr))
	b.WriteString("她写完的文章：\n" + strings.TrimSpace(draft) + "\n")
	return b.String()
}
