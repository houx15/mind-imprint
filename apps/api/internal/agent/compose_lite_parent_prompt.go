package agent

// Prompt assembly for compose_lite_parent.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"strings"

	"mindimprint/api/internal/prompts"
)

// liteParentSystemPromptTemplate is the parent report system prompt. {sections}
// is replaced with the comma-joined liteparent.SectionsWithFacts. The
// sentences after 套话 come from plan 4 Ruling 3, plan 3 Ruling 16 and plan 4
// Ruling 9.
const liteParentSystemPromptTemplate = prompts.LiteParentSystemPromptTemplate

// liteParentPronounRule tells the model which pronoun it may use for her.
// pronoun is 她 or 他; anything else means the teacher has not set a gender,
// and then the prose uses neither: a guess from her name was wrong in
// production.
func liteParentPronounRule(pronoun string) string {
	if pronoun == "她" || pronoun == "他" {
		return "指这名学生时，代词只用「" + pronoun + "」。"
	}
	return "学生的性别未设置：不用「他」「她」指这名学生，重复学生姓名，或者写「孩子」。"
}

func liteParentSystemPrompt(sections []string, pronoun string) string {
	return strings.Replace(liteParentSystemPromptTemplate, "{sections}", strings.Join(sections, ","), 1) +
		liteParentPronounRule(pronoun)
}
