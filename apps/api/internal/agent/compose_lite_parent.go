package agent

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/liteparent"
	"mindimprint/api/internal/liteweekly"
	"mindimprint/api/internal/liteworkspace"
)

// liteParentSectionMax is the rune cap on each section of a parent report.
const liteParentSectionMax = 400

// liteParentSystemPromptTemplate is the parent report system prompt. {sections}
// is replaced with the comma-joined liteparent.SectionsWithFacts. The
// sentences after 套话 come from plan 4 Ruling 3, plan 3 Ruling 16 and plan 4
// Ruling 9.
const liteParentSystemPromptTemplate = `你在为一名学生的家长写学习报告，由老师审阅后发出。只使用给出的事实，不补充事实，不评价学生的人格。输出 JSON，键为 {sections}（只输出这些键），值为该部分的正文，每部分不超过 400 字。overview 概括这段时间做了什么；next 给出 1 到 3 条家长在家可以配合的具体做法。引用学生原话时用「」并逐字照抄给出的金句。不使用事实里没有的数字。不写其他学生的名字。说明文，不用比喻、抒情和套话。作品标题用《》，只有学生原话用「」。数字一律用阿拉伯数字。不做加减和单位换算，数字照抄给出的事实。列举多条时不编号，每条单独一行。`

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

// ComposeLiteParentReport drafts the sections of a parent report from f. The
// user message is liteparent.FactsText(f), verbatim. Invalid output gets one
// retry with the validation error; every call is returned in the attempts so
// the caller can record its usage. On failure it returns an error and no
// sections, never substitute prose.
//
// otherNames are the student's classmates; the prose may not name them.
// pronoun is the one the prose may use for her (see liteParentPronounRule).
func ComposeLiteParentReport(ctx context.Context, prov gateway.Provider, r gateway.Resolved, f liteparent.Facts, otherNames []string, pronoun string) (map[string]string, []Attempt, error) {
	sections := liteparent.SectionsWithFacts(f)
	msgs := []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: liteParentSystemPrompt(sections, pronoun)},
		{Role: gateway.RoleUser, Content: liteparent.FactsText(f)},
	}
	out, attempts, err := composeLiteWeekly(ctx, prov, r, msgs, func(p map[string]string) error {
		if err := validateLiteParentReport(p, f, sections, otherNames); err != nil {
			return err
		}
		return checkLiteParentPronouns(p, sections, pronoun)
	})
	if err != nil {
		return nil, attempts, fmt.Errorf("agent: lite parent report: %w", err)
	}
	return out, attempts, nil
}

// checkLiteParentPronouns fails a draft that calls her 他 or 她 when the
// teacher set another gender or none. The error goes back to the model on the
// retry, so it is written for the model.
func checkLiteParentPronouns(p map[string]string, sections []string, pronoun string) error {
	var allowed liteworkspace.PronounsAllowed
	switch pronoun {
	case "她":
		allowed.Allow(liteworkspace.GenderFemale)
	case "他":
		allowed.Allow(liteworkspace.GenderMale)
	}
	for _, k := range sections {
		if reason := liteworkspace.PronounProblem(p[k], allowed); reason != "" {
			return fmt.Errorf("%s: %s", k, reason)
		}
	}
	return nil
}

// validateLiteParentReport checks the keys, lengths and prose of one reply.
// It normalises p's keys in place first (trim + lowercase), so the sections
// the composer returns and the caller stores carry the normalised keys.
func validateLiteParentReport(p map[string]string, f liteparent.Facts, sections []string, otherNames []string) error {
	if err := normalizeSectionKeys(p); err != nil {
		return err
	}
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
	return CheckLiteParentSections(p, sections, f, otherNames)
}

// CheckLiteParentSections runs the prose checks on each section in sections,
// reading its text from p: quotes and titles come from the facts, digits come
// from the facts, no Chinese-numeral counts, no classmate's name. It does not
// check keys or lengths. Drafting (validateLiteParentReport) and the teacher
// workspace's revise_section both call it, so a revised section passes the
// same checks a drafted one does.
//
// f is the facts the text may use (the visible facts); otherNames are the
// student's classmates. The error starts with the failing section's key.
func CheckLiteParentSections(p map[string]string, sections []string, f liteparent.Facts, otherNames []string) error {
	// The digit set is the counts and dates the prompt shows the model, plus
	// her name: liteparent.DigitFactsText, which leaves out the digits inside
	// quotes, titles and the class name (plan 4 Ruling 18 C). The system
	// prompt's own numbers (400 字, 1 到 3 条) are instructions, not facts.
	check := liteweekly.ProseCheck{
		Corpus:     liteparent.Corpus(f),
		Titles:     liteparent.TitleCorpus(f),
		FactsText:  liteparent.DigitFactsText(f) + "\n" + f.StudentName,
		OtherNames: otherNames,
		SelfName:   f.StudentName,
	}
	// Each section is checked on its own so a quote mark opened in one section
	// cannot pair with a close in the next. For the check only (the stored
	// section is unchanged):
	//   - list numbering at the start of a line is removed: it is not a fact;
	//   - the class name outside quotes and titles becomes a newline, the way
	//     CheckProse removes her name, so naming 高一（3）班 does not read as
	//     the count 3.
	for _, k := range sections {
		text := stripListMarkers(p[k])
		if f.ClassName != "" {
			text = replaceOutsideSpans(text, f.ClassName, "\n")
		}
		if err := checkChineseCounts(text); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
		if err := liteweekly.CheckProse(text, nil, check); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	return nil
}

// normalizeSectionKeys rewrites p's keys in place as trimmed lowercase, so
// "Overview " is "overview". Two keys that normalise to the same key are an
// error rather than one silently replacing the other.
func normalizeSectionKeys(p map[string]string) error {
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	norm := make(map[string]string, len(p))
	for _, k := range keys {
		n := strings.ToLower(strings.TrimSpace(k))
		if _, dup := norm[n]; dup {
			return fmt.Errorf("duplicate section: %s", n)
		}
		norm[n] = p[k]
	}
	clear(p)
	for k, v := range norm {
		p[k] = v
	}
	return nil
}

// chineseCountRe matches a count written in Chinese numerals: a run of 一 to
// 十 (and 两) holding at least one numeral other than 一, then an optional 个,
// then a counter word. 一 alone is left alone: 一篇观点相反的文章 and 一起 are
// ordinary Chinese, not counts.
var chineseCountRe = regexp.MustCompile(`[一二两三四五六七八九十]*[二两三四五六七八九十][一二两三四五六七八九十]*个?(?:篇|份|个|天|次|周|项|条|本|小时|分钟|轮|位|名)`)

// checkChineseCounts rejects a Chinese-numeral count outside quotes and
// titles. The digit check only sees Arabic digits, so 三篇 would otherwise
// pass whatever the facts say; the prompt asks for Arabic digits throughout.
// Text inside 「」, “” and 《》 is skipped: her quotes and titles may contain
// such words, and CheckProse verifies those spans against the facts.
func checkChineseCounts(text string) error {
	for _, seg := range outsideSpans(text) {
		if m := chineseCountRe.FindString(seg); m != "" {
			return fmt.Errorf("chinese numeral count: %s", m)
		}
	}
	return nil
}

// spanClose maps each opening mark CheckProse pairs to its closing mark.
var spanClose = map[rune]rune{'「': '」', '“': '”', '《': '》'}

// walkSpans splits s into the text outside bracket spans and the spans
// themselves (marks included), in order. An opening mark with no close makes
// the rest of s outside text; CheckProse reports that mark as unclosed.
func walkSpans(s string, outside, span func(string)) {
	runes := []rune(s)
	seg := 0
	for i := 0; i < len(runes); i++ {
		closeCh, ok := spanClose[runes[i]]
		if !ok {
			continue
		}
		j := i + 1
		for j < len(runes) && runes[j] != closeCh {
			j++
		}
		if j >= len(runes) {
			break
		}
		outside(string(runes[seg:i]))
		span(string(runes[i : j+1]))
		seg = j + 1
		i = j
	}
	outside(string(runes[seg:]))
}

// outsideSpans returns the pieces of s outside bracket spans.
func outsideSpans(s string) []string {
	var out []string
	walkSpans(s, func(t string) { out = append(out, t) }, func(string) {})
	return out
}

// replaceOutsideSpans replaces old with repl outside bracket spans; a span's
// text stays as written, so a verified quote still matches its corpus.
func replaceOutsideSpans(s, old, repl string) string {
	var b strings.Builder
	walkSpans(s, func(t string) { b.WriteString(strings.ReplaceAll(t, old, repl)) }, func(t string) { b.WriteString(t) })
	return b.String()
}

// stripListMarkers removes a list marker from the start of each line, after
// optional spaces: 1–2 ASCII or full-width digits followed by . ． 、 ) or ）,
// or the digits wrapped as (n) or （n）, plus the spaces after the marker.
// Digits anywhere else stay: 完成 3 篇 keeps its 3, and a line starting 2026年
// or 12月 is not a marker. A digit right after . or ． (1.5) means a number, not
// a marker, so that line is left as it is.
func stripListMarkers(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		rest := strings.TrimLeft(line, " \t　")
		if after, ok := cutListMarker([]rune(rest)); ok {
			lines[i] = strings.TrimLeft(string(after), " \t　")
		}
	}
	return strings.Join(lines, "\n")
}

// cutListMarker reports whether r starts with a list marker and returns the
// runes after it.
func cutListMarker(r []rune) ([]rune, bool) {
	isDigit := func(c rune) bool { return (c >= '0' && c <= '9') || (c >= '０' && c <= '９') }
	digits := func(from int) int { // count of 1–2 digits at from; 0 if none or more than 2
		n := 0
		for from+n < len(r) && isDigit(r[from+n]) {
			n++
		}
		if n > 2 {
			return 0
		}
		return n
	}
	if len(r) == 0 {
		return nil, false
	}
	if r[0] == '(' || r[0] == '（' {
		n := digits(1)
		if n == 0 || 1+n >= len(r) || (r[1+n] != ')' && r[1+n] != '）') {
			return nil, false
		}
		return r[2+n:], true
	}
	n := digits(0)
	if n == 0 || n >= len(r) {
		return nil, false
	}
	switch r[n] {
	case '、', ')', '）':
		return r[n+1:], true
	case '.', '．':
		if n+1 < len(r) && isDigit(r[n+1]) {
			return nil, false
		}
		return r[n+1:], true
	}
	return nil, false
}
