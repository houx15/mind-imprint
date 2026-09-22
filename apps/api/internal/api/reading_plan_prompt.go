package api

// Prompt assembly for reading_plan.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"strings"

	"mindimprint/api/internal/prompts"
)

const readingPlanSystem = prompts.ReadingPlanSystem

func buildReadingPlanPrompt(lang, title string, blocks []Block) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString("标题：" + t + "\n")
	}
	b.WriteString("学生读这篇用的语言：" + lang + "\n")

	b.WriteString("\n【可选的读法（routineKey 只能从这里挑）】\n")
	for _, r := range readingRoutinesFor(lang) {
		b.WriteString("- routineKey=" + r.Key + " · " + r.Name + " · 体裁=" + strings.Join(r.Genres, ",") +
			" · 适合：" + r.Blurb + "\n")
		for i, s := range r.Steps {
			b.WriteString("    " + itoaSmall(i+1) + ". kind=" + string(s.Kind) + " · " + s.Label + "\n")
		}
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
		if total+len(runes) > readingPlanArticleRuneBudget {
			// Truncate rather than drop: the model still needs to know that a
			// later paragraph EXISTS, or it can never pick it as a focus.
			keep := readingPlanArticleRuneBudget - total
			if keep > 60 {
				b.WriteString(tag + "：" + string(runes[:keep]) + "…（这一段更长，已截断）\n")
				total = readingPlanArticleRuneBudget
			} else {
				b.WriteString(tag + "：（这一段没放进来，但它存在）\n")
			}
			continue
		}
		total += len(runes)
		b.WriteString(tag + "：" + text + "\n")
	}
	b.WriteString("\n输出前检查学生可见的标题、摘要和步骤：用具体的内容或阅读动作命名。步骤用「预测」「比较」「说明」描述阅读动作，并写清相应的对象。\n")
	return b.String()
}
