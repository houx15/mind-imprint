// Package liteweek holds the lite edition's calendar arithmetic: a student's
// day and week are Beijing days and weeks. The offset is fixed (+08:00)
// because the production image has no tzdata, and time.LoadLocation there
// silently falls back to UTC.
package liteweek

import "time"

// Beijing is UTC+8 with no daylight saving.
var Beijing = time.FixedZone("CST", 8*3600)

// Day returns midnight (Beijing) of the Beijing date t falls on.
func Day(t time.Time) time.Time {
	b := t.In(Beijing)
	return time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, Beijing)
}

// WeekStart returns Monday 00:00 (Beijing) of the week t falls in.
func WeekStart(t time.Time) time.Time {
	d := Day(t)
	offset := (int(d.Weekday()) + 6) % 7 // Monday → 0 … Sunday → 6
	return d.AddDate(0, 0, -offset)
}
