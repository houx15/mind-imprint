package teacher

import (
	"fmt"
	"time"
)

// WeekWindow returns the half-open [Mon 00:00, next Mon 00:00) window enclosing
// now, pinned to UTC.
//
// Pinned to UTC (not now's/the server's local location) because the DB side
// buckets `event.created_at` via `AT TIME ZONE 'UTC')::date` — if this window
// were built in server-local time (e.g. Asia/Shanghai) while Postgres computes
// dates in UTC, a 7×24h window can straddle 8 distinct UTC calendar dates and
// activeDays could read 8.
//
// This is the ONLY week-window implementation. D1's roster (本周活跃) and D2's
// weekly report both call it, so the two adjacent screens cannot disagree.
func WeekWindow(now time.Time) (time.Time, time.Time) {
	now = now.UTC()
	y, m, d := now.Date()
	midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	offset := (int(now.Weekday()) + 6) % 7 // Monday=0
	start := midnight.AddDate(0, 0, -offset)
	return start, start.AddDate(0, 0, 7)
}

// PrevWindow returns last week's comparison window: last Monday 00:00 UTC
// through the SAME elapsed offset into that week that now sits at in this one.
// Week-to-date is compared against week-to-the-same-moment, so opening on a
// Wednesday does not show every metric down purely because three days have
// passed.
func PrevWindow(now time.Time) (time.Time, time.Time) {
	start, _ := WeekWindow(now)
	elapsed := now.UTC().Sub(start)
	prevStart := start.AddDate(0, 0, -7)
	return prevStart, prevStart.Add(elapsed)
}

// WeekLabel renders the binding design's 周报 header, e.g. 第 30 周（7.20–7.26）.
// The range names the full Monday–Sunday week even though the data is
// week-to-date; the DTO's asOf states the cut.
func WeekLabel(weekStart time.Time) string {
	weekStart = weekStart.UTC()
	_, week := weekStart.ISOWeek()
	end := weekStart.AddDate(0, 0, 6)
	return fmt.Sprintf("第 %d 周（%d.%d–%d.%d）", week,
		int(weekStart.Month()), weekStart.Day(), int(end.Month()), end.Day()) // U+2013 en-dash
}
