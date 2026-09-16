package liteworkspace

import (
	"fmt"
	"regexp"
	"strings"
)

// labels.go turns the wire values the workspace tools write and read
// (assignment kind, reading source, tier, article slug) into the Chinese
// words the teacher already knows from the assignment form. The AI system
// prompt and the reply text must carry only these words, never the wire
// values: a teacher who has never seen "reading" or a slug should not have
// to decode one out of the model's answer (spec §12.1).
//
// The three label tables below are copied from the teacher form, not
// invented here. They have to read identically to what she already sees —
// TestLabelsMatchTheWebTables pins the two sides together so one cannot
// drift without failing a test.

// kindLabels mirrors KIND_OPTIONS in apps/lite-web/src/teacher/AssignmentForm.tsx.
var kindLabels = map[string]string{
	"reading": "阅读",
	"writing": "写作",
	"project": "项目",
}

// KindLabel turns an assignment kind into the word the teacher sees on the
// form. An unrecognised kind (should not happen — set_fields only accepts
// the closed set above) falls back to the raw value rather than an empty
// string, so a bug here shows up as an odd word instead of a blank line.
func KindLabel(kind string) string {
	if label, ok := kindLabels[kind]; ok {
		return label
	}
	return kind
}

// sourceLabels mirrors SOURCE_OPTIONS in apps/lite-web/src/teacher/AssignmentForm.tsx.
var sourceLabels = map[string]string{
	"library":      "分级阅读库",
	"url":          "链接",
	"text":         "正文",
	"file":         "上传文件",
	"personalized": "个性化",
}

// SourceLabel turns a reading source into the word the teacher sees on the
// form. Same fallback reasoning as KindLabel.
func SourceLabel(source string) string {
	if label, ok := sourceLabels[source]; ok {
		return label
	}
	return source
}

// tierNames mirrors TIER_NAMES in apps/lite-web/src/teacher/assignmentLogic.ts.
var tierNames = []string{"入门", "基础", "进阶", "高阶", "原文"}

// TierLabel turns a tier into the word the teacher sees on the form. A nil
// tier means her own level, on a library reading and a personalized pick
// alike — the same rule tierLabel() in assignmentLogic.ts applies. A tier
// outside 1..len(tierNames) (should not happen; the library only defines
// five levels) falls back to "第 N 档" instead of panicking on the index.
func TierLabel(tier *int) string {
	if tier == nil {
		return "按学生水平"
	}
	i := *tier
	if i >= 1 && i <= len(tierNames) {
		return tierNames[i-1]
	}
	return fmt.Sprintf("第 %d 档", i)
}

// slugPattern is a lower-case, hyphenated word — the shape every article
// slug in the library has, and the shape almost nothing else the model
// writes has (a plain English word has no hyphen; a sentence has spaces and
// punctuation). One hyphen at least is required so a normal English word
// used as homework content, such as "reading", never matches.
var slugPattern = regexp.MustCompile(`[a-z0-9]+(?:-[a-z0-9]+)+`)

// ReplaceSlugs is the backstop for §12.1's rule 3: after the model has
// answered, every word in its reply and its choice labels that is a known
// article slug is swapped for《that article's Chinese title》. It is a
// backstop, not the first line of defence — liteWorkspaceCardState (in
// apps/api/internal/api) keeps slugs out of what the model reads in the
// first place, so this only catches a slug the model echoes back anyway
// (search_library and set_material's tool results still carry the real
// slug, because the tool arguments ARE wire values).
//
// title looks a candidate word up; it returns ok=false for anything that is
// not a real slug, which is how "well-known" and other ordinary hyphenated
// English survive untouched — the closed set of real slugs decides, not the
// shape of the word.
//
// A slug already sitting inside《》 is not wrapped a second time: the match
// is widened to swallow the existing brackets before the title is spliced
// in, so a slug that arrives pre-quoted ends up in one pair of brackets, not
// two.
func ReplaceSlugs(text string, title func(slug string) (string, bool)) string {
	matches := slugPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	const openQuote, closeQuote = "《", "》"
	var b strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		slug := text[start:end]
		zhTitle, ok := title(slug)
		if !ok {
			continue
		}
		lo, hi := start, end
		if lo >= len(openQuote) && text[lo-len(openQuote):lo] == openQuote {
			lo -= len(openQuote)
		}
		if hi+len(closeQuote) <= len(text) && text[hi:hi+len(closeQuote)] == closeQuote {
			hi += len(closeQuote)
		}
		b.WriteString(text[last:lo])
		b.WriteString(openQuote)
		b.WriteString(zhTitle)
		b.WriteString(closeQuote)
		last = hi
	}
	b.WriteString(text[last:])
	return b.String()
}
