// Package liteweekly builds the teacher's weekly summary for one student:
// rule-derived cards (值得表扬 / 需要建议), the facts text and corpora the
// model prompt is built from, and CheckProse to validate model-written
// prose against those facts before it is shown to a teacher.
package liteweekly

import (
	"fmt"
	"strings"
)

// Item is one reading/writing/project item referenced by a student's week.
type Item struct {
	Kind  string // reading|writing|project
	Title string
}

// Moment is one quote captured from the student during the week, tied to
// the item it came from.
type Moment struct {
	Quote     string
	ItemTitle string
}

// StudentWeek is the raw shape of one student's week, computed by T3's
// loaders.
type StudentWeek struct {
	UserID, Name string

	ActiveDays     int
	Minutes        int // -1 = no buckets in or before this week
	Turns          int
	PrevActiveDays int

	Finished []Item

	AssignmentsDone    int // due in week, done on time
	AssignmentsLate    int // due in week, done late
	AssignmentsOverdue int // due in week, unfinished at week end

	Stalled []Item // unfinished, last activity > 7 days before week end

	NewKeywords []string
	Moments     []Moment
}

// Card is one rule card: a 值得表扬 (praise) or 需要建议 (watch) tag with the
// evidence that fired it.
type Card struct {
	Kind     string // "praise" | "watch"
	Code     string // tag code
	Label    string // 值得表扬 / 需要建议 tag label
	Evidence string
}

// Cards derives at most one watch card and one praise card from a
// student's week, per spec §6.2. Table order decides precedence within
// each kind; never_used suppresses any praise card.
func Cards(s StudentWeek) (watch *Card, praise *Card) {
	switch {
	case s.ActiveDays == 0:
		watch = &Card{
			Kind:     "watch",
			Code:     "never_used",
			Label:    "本周未使用",
			Evidence: fmt.Sprintf("本周 0 天有学习记录；上一周 %d 天。", s.PrevActiveDays),
		}
	case s.AssignmentsOverdue >= 1:
		watch = &Card{
			Kind:     "watch",
			Code:     "overdue",
			Label:    "作业逾期",
			Evidence: fmt.Sprintf("本周到期的作业中有 %d 份未完成。", s.AssignmentsOverdue),
		}
	case s.ActiveDays <= s.PrevActiveDays-2:
		watch = &Card{
			Kind:     "watch",
			Code:     "dropped_off",
			Label:    "活跃下降",
			Evidence: fmt.Sprintf("活跃天数 上一周 %d 天 → 本周 %d 天。", s.PrevActiveDays, s.ActiveDays),
		}
	case len(s.Stalled) >= 1:
		var evidence string
		if len(s.Stalled) > 1 {
			evidence = fmt.Sprintf("「%s」等 %d 项超过 7 天没有进展。", s.Stalled[0].Title, len(s.Stalled))
		} else {
			evidence = fmt.Sprintf("「%s」超过 7 天没有进展。", s.Stalled[0].Title)
		}
		watch = &Card{
			Kind:     "watch",
			Code:     "stalled",
			Label:    "进度停滞",
			Evidence: evidence,
		}
	}

	// never_used means there is nothing this week to praise.
	if watch != nil && watch.Code == "never_used" {
		return watch, nil
	}

	switch {
	case s.AssignmentsDone >= 1 && s.AssignmentsOverdue == 0:
		praise = &Card{
			Kind:     "praise",
			Code:     "finished_on_time",
			Label:    "按时完成",
			Evidence: fmt.Sprintf("本周到期的作业按时完成 %d 份。", s.AssignmentsDone),
		}
	case len(s.NewKeywords) >= 1:
		keywords := s.NewKeywords
		if len(keywords) > 3 {
			keywords = keywords[:3]
		}
		praise = &Card{
			Kind:     "praise",
			Code:     "new_interest",
			Label:    "新的兴趣",
			Evidence: fmt.Sprintf("兴趣树新增关键词：%s。", strings.Join(keywords, "、")),
		}
	case s.ActiveDays >= 3 && s.ActiveDays >= s.PrevActiveDays+2:
		praise = &Card{
			Kind:     "praise",
			Code:     "more_active",
			Label:    "更加投入",
			Evidence: fmt.Sprintf("活跃天数 上一周 %d 天 → 本周 %d 天。", s.PrevActiveDays, s.ActiveDays),
		}
	}

	return watch, praise
}

// FactsText renders every numeric field of s, plus weekLabel, finished and
// stalled titles, new keywords and moments, as plain Chinese sentences. It
// is both the allowed-digit set for CheckProse and, verbatim, part of T4's
// prompt.
func FactsText(s StudentWeek, weekLabel string) string {
	var b strings.Builder
	b.WriteString(weekLabel)
	fmt.Fprintf(&b, "；活跃 %d 天", s.ActiveDays)
	fmt.Fprintf(&b, "；上一周活跃 %d 天", s.PrevActiveDays)
	if s.Minutes == -1 {
		b.WriteString("；学习时长 无记录")
	} else {
		fmt.Fprintf(&b, "；学习 %d 分钟", s.Minutes)
	}
	fmt.Fprintf(&b, "；对话 %d 轮", s.Turns)
	fmt.Fprintf(&b, "；完成 %d 项", len(s.Finished))
	fmt.Fprintf(&b, "；按时完成作业 %d 份", s.AssignmentsDone)
	fmt.Fprintf(&b, "；逾期完成作业 %d 份", s.AssignmentsLate)
	fmt.Fprintf(&b, "；逾期作业 %d 份", s.AssignmentsOverdue)
	fmt.Fprintf(&b, "；停滞 %d 项", len(s.Stalled))

	for _, it := range s.Finished {
		fmt.Fprintf(&b, "；完成《%s》", it.Title)
	}
	for _, it := range s.Stalled {
		fmt.Fprintf(&b, "；停滞《%s》", it.Title)
	}
	for _, k := range s.NewKeywords {
		fmt.Fprintf(&b, "；新关键词 %s", k)
	}
	for _, m := range s.Moments {
		fmt.Fprintf(&b, "；「%s」（《%s》）", m.Quote, m.ItemTitle)
	}

	return b.String()
}

// Corpus returns the student's own words for the quote check: moment
// quotes and new keywords, one per line.
func Corpus(s StudentWeek) string {
	var lines []string
	for _, m := range s.Moments {
		lines = append(lines, m.Quote)
	}
	lines = append(lines, s.NewKeywords...)
	return strings.Join(lines, "\n")
}

// TitleCorpus returns finished and stalled item titles for the title
// check, one per line. Kept separate from Corpus because an item title can
// be the teacher's own assignment title, not the student's words.
func TitleCorpus(s StudentWeek) string {
	var lines []string
	for _, it := range s.Finished {
		lines = append(lines, it.Title)
	}
	for _, it := range s.Stalled {
		lines = append(lines, it.Title)
	}
	return strings.Join(lines, "\n")
}
