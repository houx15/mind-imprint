package liteweek

import (
	"errors"
	"fmt"
	"time"
)

var ErrBadWeek = errors.New("liteweek: not a completed Beijing week start")

// LatestCompleted is the Monday 00:00 (Beijing) of the last full week before now.
func LatestCompleted(now time.Time) time.Time {
	return WeekStart(now).AddDate(0, 0, -7)
}

// IsCompleted reports whether the week starting at weekStart has ended by now.
func IsCompleted(weekStart, now time.Time) bool {
	return !weekStart.AddDate(0, 0, 7).After(now)
}

// ParseWeekStart reads a ?weekStart value. Empty means the latest completed week.
func ParseWeekStart(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return LatestCompleted(now), nil
	}
	var t time.Time
	var err error
	if len(s) == len("2006-01-02") {
		t, err = time.ParseInLocation("2006-01-02", s, Beijing)
	} else {
		t, err = time.Parse(time.RFC3339, s)
	}
	if err != nil {
		return time.Time{}, ErrBadWeek
	}
	t = t.In(Beijing)
	if !t.Equal(WeekStart(t)) || !IsCompleted(t, now) {
		return time.Time{}, ErrBadWeek
	}
	return t, nil
}

// Label names a week: 第 37 周（9.7–9.13）.
func Label(weekStart time.Time) string {
	ws := weekStart.In(Beijing)
	_, wk := ws.ISOWeek()
	end := ws.AddDate(0, 0, 6)
	return fmt.Sprintf("第 %d 周（%d.%d–%d.%d）", wk, int(ws.Month()), ws.Day(), int(end.Month()), end.Day())
}
