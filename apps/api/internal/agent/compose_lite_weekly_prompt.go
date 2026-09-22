package agent

// Prompt assembly for compose_lite_weekly.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/prompts"

	"fmt"

	"strings"

	"mindimprint/api/internal/liteweekly"
)

const liteStudentWeeklySystemPrompt = prompts.LiteStudentWeeklySystemPrompt

const liteClassWeeklySystemPrompt = prompts.LiteClassWeeklySystemPrompt

func liteStudentWeeklyUserPrompt(s liteweekly.StudentWeek, weekLabel string, cards []liteweekly.Card) string {
	var b strings.Builder
	fmt.Fprintf(&b, "学生：%s\n", s.Name)
	fmt.Fprintf(&b, "事实：%s\n", liteweekly.FactsText(s, weekLabel))
	b.WriteString("卡片：")
	if len(cards) == 0 {
		b.WriteString("无")
	}
	for _, c := range cards {
		fmt.Fprintf(&b, "\n- %s · %s · %s", c.Code, c.Label, c.Evidence)
	}
	return b.String()
}

// liteClassWeeklyUserPrompt is the class facts text followed by the flagged
// students' IDs and the words the prose may quote: titles as 《》, her words
// (金句 and new keywords) as 「」. IDs are kept out of the facts text so their
// digits do not widen the digit check.
func liteClassWeeklyUserPrompt(facts string, flagged []liteweekly.StudentWeek) string {
	var b strings.Builder
	b.WriteString(facts)
	b.WriteString("\n学生名单：")
	if len(flagged) == 0 {
		b.WriteString("无")
	}
	for _, s := range flagged {
		parts := []string{"userId=" + s.UserID, "姓名=" + s.Name}
		for _, it := range s.Finished {
			parts = append(parts, "完成《"+it.Title+"》")
		}
		for _, it := range s.Stalled {
			parts = append(parts, "停滞《"+it.Title+"》")
		}
		// Keywords are her words (in Corpus), bracketed so a keyword with
		// digits (5G) copied into the prose is a verified span, not bare digits.
		for _, k := range s.NewKeywords {
			parts = append(parts, "新关键词 「"+k+"」")
		}
		for _, m := range s.Moments {
			parts = append(parts, "「"+m.Quote+"」（《"+m.ItemTitle+"》）")
		}
		b.WriteString("\n- " + strings.Join(parts, "；"))
	}
	return b.String()
}
