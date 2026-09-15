// Package liteparent holds the pure parts of a lite parent report: the facts
// a teacher's report is written from, date-range parsing, the facts text and
// corpora the model prompt and prose check read, and which sections a report
// shows. Loading the facts and calling the model live in other packages.
package liteparent

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"mindimprint/api/internal/liteweek"
)

// SectionKeys is the fixed order of a report's sections.
var SectionKeys = []string{"overview", "reading", "writing", "projects", "interests", "next"}

// SectionLabels is the heading shown for each section key.
var SectionLabels = map[string]string{
	"overview":  "总体概述",
	"reading":   "阅读",
	"writing":   "写作",
	"projects":  "项目",
	"interests": "兴趣",
	"next":      "下一步建议",
}

// Item is one reading, writing or project finished in the range.
type Item struct {
	Kind       string `json:"kind"`
	Title      string `json:"title"`
	FinishedAt string `json:"finishedAt"` // YYYY-MM-DD Beijing
}

// Moment is one quote of the student's own words, tied to the item it came
// from.
type Moment struct {
	Quote     string `json:"quote"`
	ItemTitle string `json:"itemTitle"`
}

// Keyword is one word that first appeared on the student's interest tree in
// the range.
type Keyword struct {
	Text       string `json:"text"`
	Field      string `json:"field"`
	FieldLabel string `json:"fieldLabel"`
}

// Facts is the snapshot a parent report is written from. It is frozen into
// lite_parent_report.facts when the report is generated. It carries counts,
// finished work titles, her quotes and keywords; never chat text and never
// text the teacher wrote.
type Facts struct {
	StudentName       string    `json:"studentName"`
	ClassName         string    `json:"className"`
	TeacherName       string    `json:"teacherName"`
	RangeStart        string    `json:"rangeStart"`
	RangeEnd          string    `json:"rangeEnd"`
	Days              int       `json:"days"`
	ActiveDays        int       `json:"activeDays"`
	Minutes           int       `json:"minutes"` // -1 when no buckets exist in range
	Turns             int       `json:"turns"`
	Readings          []Item    `json:"readings"`
	Writings          []Item    `json:"writings"`
	Projects          []Item    `json:"projects"`
	Moments           []Moment  `json:"moments"`
	AssignmentsTotal  int       `json:"assignmentsTotal"`
	AssignmentsOnTime int       `json:"assignmentsOnTime"`
	AssignmentsLate   int       `json:"assignmentsLate"`
	AssignmentsMissed int       `json:"assignmentsMissed"`
	Keywords          []Keyword `json:"keywords"`
}

// Hidden is what a teacher has hidden from a report, addressed by text: a
// moment by its exact quote, a keyword by its exact text. It is stored in
// lite_parent_report.hidden. Moments and Keywords are never nil once
// normalised.
type Hidden struct {
	Moments  []string `json:"moments"`
	Keywords []string `json:"keywords"`
}

// Normalize returns h with both lists non-nil and duplicates removed, keeping
// the first occurrence's order.
func (h Hidden) Normalize() Hidden {
	return Hidden{Moments: dedupe(h.Moments), Keywords: dedupe(h.Keywords)}
}

func dedupe(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// UnknownHidden returns the first entry of h that is not in f: a moment entry
// that equals no moment quote, or a keyword entry that equals no keyword text.
// ok is false when every entry is in f.
func UnknownHidden(f Facts, h Hidden) (entry string, ok bool) {
	quotes := make(map[string]bool, len(f.Moments))
	for _, m := range f.Moments {
		quotes[m.Quote] = true
	}
	for _, q := range h.Moments {
		if !quotes[q] {
			return q, true
		}
	}
	words := make(map[string]bool, len(f.Keywords))
	for _, k := range f.Keywords {
		words[k.Text] = true
	}
	for _, w := range h.Keywords {
		if !words[w] {
			return w, true
		}
	}
	return "", false
}

// VisibleFacts returns a copy of f without the hidden moments (matched by
// exact quote) and keywords (matched by exact text). An entry that matches
// nothing is ignored. Every other field, counts included, is unchanged. The
// returned Moments and Keywords are new slices, never nil.
func VisibleFacts(f Facts, h Hidden) Facts {
	hiddenQuotes := make(map[string]bool, len(h.Moments))
	for _, q := range h.Moments {
		hiddenQuotes[q] = true
	}
	hiddenWords := make(map[string]bool, len(h.Keywords))
	for _, w := range h.Keywords {
		hiddenWords[w] = true
	}
	out := f
	out.Moments = make([]Moment, 0, len(f.Moments))
	for _, m := range f.Moments {
		if !hiddenQuotes[m.Quote] {
			out.Moments = append(out.Moments, m)
		}
	}
	out.Keywords = make([]Keyword, 0, len(f.Keywords))
	for _, k := range f.Keywords {
		if !hiddenWords[k.Text] {
			out.Keywords = append(out.Keywords, k)
		}
	}
	return out
}

// HiddenMentions reports where the body still mentions a hidden item. It
// looks at each section in sections (the visible sections,
// SectionsWithFacts(VisibleFacts(f, h))) and lists every hidden moment quote
// and hidden keyword text, of at least 2 runes and present in f, that the
// section's text contains. A section appears only with at least one mention;
// its list has no duplicates and follows the hidden lists' order (moments,
// then keywords). The map is never nil. The export is built on the client, so
// this is how the editor learns a hidden 金句 is still quoted in the text.
func HiddenMentions(body map[string]string, sections []string, f Facts, h Hidden) map[string][]string {
	out := map[string][]string{}
	inFacts := make(map[string]bool, len(f.Moments)+len(f.Keywords))
	for _, m := range f.Moments {
		inFacts[m.Quote] = true
	}
	for _, k := range f.Keywords {
		inFacts[k.Text] = true
	}
	var texts []string
	for _, list := range [][]string{h.Moments, h.Keywords} {
		for _, s := range list {
			if inFacts[s] && utf8.RuneCountInString(s) >= 2 {
				texts = append(texts, s)
			}
		}
	}
	for _, sec := range sections {
		text := body[sec]
		if text == "" {
			continue
		}
		var found []string
		seen := map[string]bool{}
		for _, s := range texts {
			if !seen[s] && strings.Contains(text, s) {
				seen[s] = true
				found = append(found, s)
			}
		}
		if len(found) > 0 {
			out[sec] = found
		}
	}
	return out
}

// ErrBadRange is returned for a date range that cannot be reported on.
var ErrBadRange = errors.New("liteparent: invalid date range")

// MaxRangeDays is the longest range, counted inclusive of both dates.
const MaxRangeDays = 366

// DefaultRangeDays is the length of DefaultRange.
const DefaultRangeDays = 28

const dateLayout = "2006-01-02"

// ParseRange parses two Beijing calendar dates (YYYY-MM-DD) into midnight
// (Beijing) of each date. The range includes both dates. It rejects a bad
// format, an end before the start, an end after today (Beijing) and a range
// longer than MaxRangeDays, all with ErrBadRange.
func ParseRange(start, end string, now time.Time) (s, e time.Time, err error) {
	s, err = time.ParseInLocation(dateLayout, start, liteweek.Beijing)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: start %q", ErrBadRange, start)
	}
	e, err = time.ParseInLocation(dateLayout, end, liteweek.Beijing)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: end %q", ErrBadRange, end)
	}
	if e.Before(s) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: end %s is before start %s", ErrBadRange, end, start)
	}
	if e.After(liteweek.Day(now)) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: end %s is in the future", ErrBadRange, end)
	}
	if n := RangeDays(s, e); n > MaxRangeDays {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: %d days, over the %d-day limit", ErrBadRange, n, MaxRangeDays)
	}
	return s, e, nil
}

// RangeDays counts the calendar days from s to e, both included. s and e are
// Beijing midnights as returned by ParseRange.
func RangeDays(s, e time.Time) int {
	return int(e.Sub(s).Hours()/24) + 1
}

// DefaultRange returns the DefaultRangeDays days ending yesterday (Beijing),
// as YYYY-MM-DD dates.
func DefaultRange(now time.Time) (start, end string) {
	e := liteweek.Day(now).AddDate(0, 0, -1)
	s := e.AddDate(0, 0, -(DefaultRangeDays - 1))
	return s.Format(dateLayout), e.Format(dateLayout)
}

// SectionsWithFacts lists the sections a report shows, in SectionKeys order.
// overview and next are always present; the others only when the facts have
// something for them.
func SectionsWithFacts(f Facts) []string {
	out := []string{"overview"}
	if len(f.Readings) > 0 {
		out = append(out, "reading")
	}
	if len(f.Writings) > 0 {
		out = append(out, "writing")
	}
	if len(f.Projects) > 0 {
		out = append(out, "projects")
	}
	if len(f.Keywords) > 0 {
		out = append(out, "interests")
	}
	return append(out, "next")
}

// FactsText renders every numeric field of f, the finished titles, keywords
// and moments as plain Chinese sentences. It is both the allowed-digit set
// for the prose check and, verbatim, part of the model prompt, so it writes
// no number that is not a fact: no keyword count (the keyword list is
// capped) and no per-item finish date. The range dates are written twice
// (2026-08-17 and 8月17日) so either spelling in the prose matches a digit
// run here. Minutes == -1 is written as 无记录, never as a number.
func FactsText(f Facts) string {
	var b strings.Builder
	fmt.Fprintf(&b, "日期 %s 至 %s（%s至%s），共 %d 天", f.RangeStart, f.RangeEnd, MonthDay(f.RangeStart), MonthDay(f.RangeEnd), f.Days)
	if f.StudentName != "" {
		fmt.Fprintf(&b, "；学生 %s", f.StudentName)
	}
	if f.ClassName != "" {
		fmt.Fprintf(&b, "；班级 %s", f.ClassName)
	}
	fmt.Fprintf(&b, "；活跃 %d 天", f.ActiveDays)
	if f.Minutes == -1 {
		b.WriteString("；学习时长 无记录")
	} else {
		fmt.Fprintf(&b, "；学习 %d 分钟", f.Minutes)
	}
	fmt.Fprintf(&b, "；对话 %d 轮", f.Turns)
	fmt.Fprintf(&b, "；完成阅读 %d 篇", len(f.Readings))
	fmt.Fprintf(&b, "；完成写作 %d 篇", len(f.Writings))
	fmt.Fprintf(&b, "；完成项目 %d 个", len(f.Projects))
	fmt.Fprintf(&b, "；到期作业 %d 份", f.AssignmentsTotal)
	fmt.Fprintf(&b, "；按时完成作业 %d 份", f.AssignmentsOnTime)
	fmt.Fprintf(&b, "；逾期完成作业 %d 份", f.AssignmentsLate)
	fmt.Fprintf(&b, "；未完成作业 %d 份", f.AssignmentsMissed)

	// Keywords are listed but not counted: the query keeps only the top 12,
	// so len(f.Keywords) is not her real count. Item finish dates are not
	// written either: every date here becomes an allowed digit run.
	for _, group := range []struct {
		label string
		items []Item
	}{{"完成阅读", f.Readings}, {"完成写作", f.Writings}, {"完成项目", f.Projects}} {
		for _, it := range group.items {
			fmt.Fprintf(&b, "；%s《%s》", group.label, it.Title)
		}
	}
	for _, k := range f.Keywords {
		if k.FieldLabel != "" {
			fmt.Fprintf(&b, "；新关键词 %s（%s）", k.Text, k.FieldLabel)
		} else {
			fmt.Fprintf(&b, "；新关键词 %s", k.Text)
		}
	}
	for _, m := range f.Moments {
		fmt.Fprintf(&b, "；「%s」（《%s》）", m.Quote, m.ItemTitle)
	}
	return b.String()
}

// DigitFactsText is the allowed-digit text for the prose check: FactsText
// with every 「…」 and 《…》 span removed and the class name replaced with a
// newline. The check already removes verified quotes and titles from the
// prose, so digits inside them never need to be allowed; allowing them would
// let a count taken from a title (《第7课》) or a class name (高一（3）班) pass
// as a fact. Each removal leaves a newline, so the text on either side cannot
// join into a new digit run. FactsText itself, which the model reads, is
// unchanged.
func DigitFactsText(f Facts) string {
	s := FactsText(f)
	if f.ClassName != "" {
		s = strings.ReplaceAll(s, f.ClassName, "\n")
	}
	return removeBracketSpans(s)
}

// removeBracketSpans replaces every 「…」 and 《…》 span, brackets included,
// with a newline. An opening mark with no close is left as it is.
func removeBracketSpans(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		var closeCh rune
		switch runes[i] {
		case '「':
			closeCh = '」'
		case '《':
			closeCh = '》'
		default:
			b.WriteRune(runes[i])
			continue
		}
		j := i + 1
		for j < len(runes) && runes[j] != closeCh {
			j++
		}
		if j >= len(runes) {
			b.WriteRune(runes[i])
			continue
		}
		b.WriteRune('\n')
		i = j
	}
	return b.String()
}

// DropMomentsNaming returns the moments whose quote names none of names. A
// name shorter than 2 runes is ignored: one character matches too much
// ordinary text. A 金句 is shown on the parent page and poster as it is, and
// the prose check deliberately allows a classmate's name inside a verified
// quote, so a quote that names a classmate must not enter the facts at all.
func DropMomentsNaming(moments []Moment, names []string) []Moment {
	out := make([]Moment, 0, len(moments))
	for _, m := range moments {
		if !namesAny(m.Quote, names) {
			out = append(out, m)
		}
	}
	return out
}

func namesAny(s string, names []string) bool {
	for _, n := range names {
		if utf8.RuneCountInString(n) >= 2 && strings.Contains(s, n) {
			return true
		}
	}
	return false
}

// MonthDay turns a YYYY-MM-DD date into M月D日. An unparseable date yields "".
func MonthDay(date string) string {
	t, err := time.ParseInLocation(dateLayout, date, liteweek.Beijing)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d月%d日", int(t.Month()), t.Day())
}

// Corpus returns the student's own words for the quote check: moment quotes
// and keyword texts, one per line. Titles are not her words (a title can be
// the teacher's assignment title) and live in TitleCorpus.
func Corpus(f Facts) string {
	var lines []string
	for _, m := range f.Moments {
		lines = append(lines, m.Quote)
	}
	for _, k := range f.Keywords {
		lines = append(lines, k.Text)
	}
	return strings.Join(lines, "\n")
}

// TitleCorpus returns every reading, writing and project title for the title
// check, one per line.
func TitleCorpus(f Facts) string {
	var lines []string
	for _, items := range [][]Item{f.Readings, f.Writings, f.Projects} {
		for _, it := range items {
			lines = append(lines, it.Title)
		}
	}
	return strings.Join(lines, "\n")
}
