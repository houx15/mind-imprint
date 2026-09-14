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
	fmt.Fprintf(&b, "日期 %s 至 %s（%s至%s），共 %d 天", f.RangeStart, f.RangeEnd, monthDay(f.RangeStart), monthDay(f.RangeEnd), f.Days)
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

// monthDay turns a YYYY-MM-DD date into M月D日. An unparseable date yields "".
func monthDay(date string) string {
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
