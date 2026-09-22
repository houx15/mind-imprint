package api

// Prompt assembly for reading_questions.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"strings"

	"mindimprint/api/internal/prompts"
)

const readingQuestionsSystem = prompts.ReadingQuestionsSystem

// buildReadingQuestionsPrompt hands the model the title and the article,
// tagged the same way buildReadingPlanPrompt (reading_plan.go) tags it —
// both the id it must eventually be found under and the ordinal it may speak
// in prose — so the same truncation-with-a-marker discipline applies here
// too: a paragraph past the budget is still named as existing, never simply
// dropped.
func buildReadingQuestionsPrompt(title string, blocks []Block) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("标题：" + t + "\n")
	}
	b.WriteString("\n【文章，按段落】\n")
	total := 0
	for i, blk := range blocks {
		text := strings.TrimSpace(blk.Text)
		if text == "" {
			continue
		}
		tag := readingBlockTag(i, blk.ID)
		runes := []rune(text)
		if total+len(runes) > readingQuestionsArticleRuneBudget {
			keep := readingQuestionsArticleRuneBudget - total
			if keep > 60 {
				b.WriteString(tag + "：" + string(runes[:keep]) + "…（这一段更长，已截断）\n")
				total = readingQuestionsArticleRuneBudget
			} else {
				b.WriteString(tag + "：（这一段没放进来，但它存在）\n")
			}
			continue
		}
		total += len(runes)
		b.WriteString(tag + "：" + text + "\n")
	}
	return b.String()
}
