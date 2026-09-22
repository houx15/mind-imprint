package agent

// Prompt assembly for reading_card_example.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/cards"
	"strings"
)

func buildCardExamplePrompt(spec cards.Spec) string {
	var b strings.Builder
	b.WriteString("你是一名批判性阅读教练。学生自己选了一副思维透镜「" + spec.Name + "」，想看看它能怎么用在这篇文章上。\n")
	if lens := spec.ReadingLens; lens != nil {
		// A lens is purpose-built for the pick-one-sentence mechanic — steer the
		// model with its own task/hint/focus so it reliably finds a groundable
		// single sentence (the whole reason lenses replaced the tool cards here).
		b.WriteString("这副透镜是做什么的：" + spec.Purpose + "\n")
		b.WriteString("要挑什么样的句子：" + lens.TaskPrompt + "\n")
		b.WriteString("怎么找：" + lens.SelectionHint + "\n")
		b.WriteString("示范要突出什么：" + lens.ExampleFocus + "\n")
	} else {
		what := spec.Purpose
		if spec.TriggerCondition != "" {
			what = spec.TriggerCondition + "。" + what
		}
		b.WriteString("这副透镜是做什么的：" + what + "\n")
	}
	b.WriteString("请从文章里挑出恰好一句最能示范这副透镜的原句，并用不超过两句话解释为什么这句适合——克制、贴合原文，不要替她下最终结论，只是给她一个示范起点。\n")
	b.WriteString("只输出 JSON：{\"block_id\":\"示范句所在的 block id\",\"quote\":\"该 block 里的一句原文，必须逐字来自原文\",\"why\":\"不超过两句话的中文解释\"}。不要输出任何多余文字。")
	return b.String()
}

func renderCardExampleArticle(blocks []MaterialBlock) string {
	var b strings.Builder
	b.WriteString("文章：\n")
	total := 0
	for _, blk := range blocks {
		runes := []rune(blk.Text)
		if total+len(runes) > cardExampleArticleRuneBudget {
			b.WriteString("[" + blk.ID + "] （这一段没放进来，但它存在）\n")
			continue
		}
		total += len(runes)
		b.WriteString("[" + blk.ID + "] " + blk.Text + "\n")
	}
	return b.String()
}
