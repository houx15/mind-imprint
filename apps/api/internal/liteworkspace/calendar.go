package liteworkspace

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// calendar.go — deadlines a teacher gives by weekday. Real-user walk,
// 2026-09-17 (a Thursday): asked for 「周五晚上九点」, the model wrote 9月20日,
// a Sunday. The prompt carried today's date and nothing else, so the model
// computed the weekday itself; one run got it right, the next did not.

// CalendarDays is how many days from today the prompt lists.
const CalendarDays = 14

// Calendar lists today and the following days with their weekdays, one per
// line: 「- 9月18日 周五（2026-09-18）」. The prompt carries it so the model
// looks weekdays up instead of computing them.
func Calendar(today time.Time) string {
	lines := make([]string, 0, CalendarDays)
	for i := 0; i < CalendarDays; i++ {
		d := today.AddDate(0, 0, i)
		note := ""
		switch i {
		case 0:
			note = "，今天"
		case 1:
			note = "，明天"
		}
		lines = append(lines, fmt.Sprintf("- %d月%d日 %s%s（%s%s）", d.Month(), d.Day(), weekLabel(today, d), weekdayNames[d.Weekday()], d.Format("2006-01-02"), note))
	}
	return strings.Join(lines, "\n")
}

// weekLabel is 本周, 下周 or 下下周 for d, with weeks starting on Monday.
// Measured 2026-09-17 (a Thursday): 「下周三」 became 9月30日, the Wednesday
// after next, with a plain list of dates to read from.
func weekLabel(today, d time.Time) string {
	monday := func(t time.Time) time.Time {
		offset := (int(t.Weekday()) + 6) % 7
		y, m, day := t.AddDate(0, 0, -offset).Date()
		return time.Date(y, m, day, 0, 0, 0, 0, t.Location())
	}
	weeks := int(monday(d).Sub(monday(today)).Hours()/24+0.5) / 7
	switch weeks {
	case 0:
		return "本周"
	case 1:
		return "下周"
	case 2:
		return "下下周"
	}
	return ""
}

var weekdayMention = regexp.MustCompile(`(周|星期|礼拜)([一二三四五六日天])`)

var explicitDate = regexp.MustCompile(`\d{1,2}\s*月\s*\d{1,2}\s*[日号]|\d{4}-\d{2}-\d{2}|\d{1,2}\s*号`)

var weekdayByChar = map[string]time.Weekday{
	"一": time.Monday, "二": time.Tuesday, "三": time.Wednesday, "四": time.Thursday,
	"五": time.Friday, "六": time.Saturday, "日": time.Sunday, "天": time.Sunday,
}

// DueWeekdayMismatch reports, in Chinese, a deadline that falls on another
// weekday than the one the teacher named; "" when they agree or when the
// teacher gave no weekday. texts are her messages, oldest first: the last
// weekday she named counts, and a message with an explicit date (9月18日,
// 18号) after it means she chose the date herself.
func DueWeekdayMismatch(texts []string, dueInput string) string {
	due, err := time.ParseInLocation("2006-01-02T15:04", strings.TrimSpace(dueInput), BeijingOffset)
	if err != nil {
		return ""
	}
	want, named, word := time.Weekday(0), false, ""
	for _, t := range texts {
		if m := weekdayMention.FindAllStringSubmatch(t, -1); len(m) > 0 {
			last := m[len(m)-1]
			want, named, word = weekdayByChar[last[2]], true, last[0]
		}
		if named && explicitDate.MatchString(t) {
			return ""
		}
	}
	if !named || due.Weekday() == want {
		return ""
	}
	return fmt.Sprintf("老师说的是%s，但截止时间 %d月%d日 是%s。按提示里的日历找到%s那一天，用 set_fields 改正截止时间，再告诉老师",
		word, due.Month(), due.Day(), weekdayNames[due.Weekday()], word)
}
