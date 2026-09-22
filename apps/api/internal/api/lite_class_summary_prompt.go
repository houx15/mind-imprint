package api

// Prompt assembly for lite_class_summary.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"
)

// liteClassSummarySystemPrompt asks for plain 说明文, one to three sentences,
// no invented facts. AGENTS.md 界面文案 rule 10: no metaphor, no 抒情副词, no
// exclamation marks outside a real milestone (this is neither).
const liteClassSummarySystemPrompt = prompts.LiteClassSummarySystemPrompt
