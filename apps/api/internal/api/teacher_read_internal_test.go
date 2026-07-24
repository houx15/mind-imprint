package api

import (
	"testing"
	"time"
)

// TestWeekWindowPinnedToUTC guards the whole-branch review's FIX 2: weekWindow
// must anchor Monday-midnight in UTC regardless of the server process's local
// timezone, so it always agrees with the DB-side
// `(created_at AT TIME ZONE 'UTC')::date` bucketing in teacher.sql — otherwise
// a 7-day window can span 8 distinct UTC dates (e.g. an Asia/Shanghai server
// vs UTC-stored/bucketed events).
func TestWeekWindowPinnedToUTC(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	// 2026-01-05 02:00 in Shanghai (UTC+8) is 2026-01-04 18:00 UTC — a Sunday.
	// The local calendar date (Monday) and the UTC calendar date (Sunday)
	// disagree, which is exactly the scenario the fix must handle correctly.
	now := time.Date(2026, 1, 5, 2, 0, 0, 0, shanghai)

	start, end := weekWindow(now)

	if start.Location() != time.UTC || end.Location() != time.UTC {
		t.Fatalf("weekWindow must return UTC-located times, got start=%v end=%v", start.Location(), end.Location())
	}
	if !end.Equal(start.AddDate(0, 0, 7)) {
		t.Fatalf("window is not exactly 7 days: start=%v end=%v", start, end)
	}
	if start.Weekday() != time.Monday || start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 {
		t.Fatalf("start is not UTC Monday midnight: %v", start)
	}
	// The window must be anchored to now's UTC date (2026-01-04, a Sunday) —
	// i.e. the PRECEDING Monday in UTC — not now's Shanghai-local date
	// (2026-01-05, already a Monday), which would wrongly anchor a week later.
	wantStart := time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Fatalf("start = %v, want %v (UTC-anchored, not local-anchored)", start, wantStart)
	}
}
