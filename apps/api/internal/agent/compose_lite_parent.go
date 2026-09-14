package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteparent"
	"mindimprint/api/internal/liteweekly"
)

// liteParentSectionMax is the rune cap on each section of a parent report.
const liteParentSectionMax = 400

// liteParentSystemPromptTemplate is the parent report system prompt. {sections}
// is replaced with the comma-joined liteparent.SectionsWithFacts. The last
// three sentences come from plan 4 Ruling 3 and plan 3 Ruling 16.
const liteParentSystemPromptTemplate = `你在为一名学生的家长写学习报告，由老师审阅后发出。只使用给出的事实，不补充事实，不评价学生的人格。输出 JSON，键为 {sections}（只输出这些键），值为该部分的正文，每部分不超过 400 字。overview 概括这段时间做了什么；next 给出 1 到 3 条家长在家可以配合的具体做法。引用学生原话时用「」并逐字照抄给出的金句。不使用事实里没有的数字。不写其他学生的名字。说明文，不用比喻、抒情和套话。作品标题用《》，只有学生原话用「」。数字一律用阿拉伯数字。不做加减和单位换算，数字照抄给出的事实。`

func liteParentSystemPrompt(sections []string) string {
	return strings.Replace(liteParentSystemPromptTemplate, "{sections}", strings.Join(sections, ","), 1)
}

// ComposeLiteParentReport drafts the sections of a parent report from f. The
// user message is liteparent.FactsText(f), verbatim. Invalid output gets one
// retry with the validation error; every call is returned in the attempts so
// the caller can record its usage. On failure it returns an error and no
// sections, never substitute prose.
//
// otherNames are the student's classmates; the prose may not name them.
func ComposeLiteParentReport(ctx context.Context, prov gateway.Provider, r gateway.Resolved, f liteparent.Facts, otherNames []string) (map[string]string, []Attempt, error) {
	sections := liteparent.SectionsWithFacts(f)
	msgs := []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: liteParentSystemPrompt(sections)},
		{Role: gateway.RoleUser, Content: liteparent.FactsText(f)},
	}
	out, attempts, err := composeLiteWeekly(ctx, prov, r, msgs, func(p map[string]string) error {
		return validateLiteParentReport(p, f, sections, otherNames)
	})
	if err != nil {
		return nil, attempts, fmt.Errorf("agent: lite parent report: %w", err)
	}
	return out, attempts, nil
}

// validateLiteParentReport checks the keys, lengths and prose of one reply.
func validateLiteParentReport(p map[string]string, f liteparent.Facts, sections []string, otherNames []string) error {
	want := make(map[string]bool, len(sections))
	for _, k := range sections {
		want[k] = true
	}
	var extra []string
	for k := range p {
		if !want[k] {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		return fmt.Errorf("unexpected section: %s", strings.Join(extra, ", "))
	}
	for _, k := range sections {
		v, ok := p[k]
		if !ok {
			return fmt.Errorf("missing section: %s", k)
		}
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("section %s is empty", k)
		}
		if n := utf8.RuneCountInString(v); n > liteParentSectionMax {
			return fmt.Errorf("section %s has %d characters, over the %d limit", k, n, liteParentSectionMax)
		}
	}
	// The digit set is everything the prompt shows the model that can carry a
	// digit: the facts text (the whole user message) and her name. The system
	// prompt's own numbers (400 字, 1 到 3 条) are instructions, not facts.
	check := liteweekly.ProseCheck{
		Corpus:     liteparent.Corpus(f),
		Titles:     liteparent.TitleCorpus(f),
		FactsText:  liteparent.FactsText(f) + "\n" + f.StudentName,
		OtherNames: otherNames,
		SelfName:   f.StudentName,
	}
	// Each section is checked on its own so a quote mark opened in one section
	// cannot pair with a close in the next.
	for _, k := range sections {
		if err := liteweekly.CheckProse(p[k], nil, check); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	return nil
}
