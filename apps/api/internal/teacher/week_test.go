package teacher_test

import (
	"testing"
	"time"

	"mindimprint/api/internal/teacher"
)

func TestWeekWindowStartsMondayUTC(t *testing.T) {
	// 2026-07-24 is a Friday, 15:30 in +08:00 → 07:30 UTC the same day.
	now := time.Date(2026, 7, 24, 15, 30, 0, 0, time.FixedZone("CST", 8*3600))
	start, end := teacher.WeekWindow(now)
	wantStart := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) {
		t.Fatalf("start = %v; want %v", start, wantStart)
	}
	if !end.Equal(wantStart.AddDate(0, 0, 7)) {
		t.Fatalf("end = %v; want start+7d", end)
	}
	if start.Location() != time.UTC || end.Location() != time.UTC {
		t.Fatal("window must be pinned to UTC")
	}
}

func TestWeekWindowMondayIsItsOwnStart(t *testing.T) {
	now := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	start, _ := teacher.WeekWindow(now)
	if !start.Equal(now) {
		t.Fatalf("start = %v; want the Monday itself", start)
	}
}

func TestWeekWindowSundayStaysInTheSameWeek(t *testing.T) {
	now := time.Date(2026, 7, 26, 23, 59, 0, 0, time.UTC)
	start, end := teacher.WeekWindow(now)
	if !start.Equal(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start = %v; want 2026-07-20", start)
	}
	if !now.Before(end) {
		t.Fatal("Sunday 23:59 must fall inside its own week window")
	}
}

func TestPrevWindowIsTheSameElapsedOffset(t *testing.T) {
	now := time.Date(2026, 7, 22, 9, 15, 0, 0, time.UTC) // Wednesday 09:15
	ps, pe := teacher.PrevWindow(now)
	if !ps.Equal(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("prev start = %v; want 2026-07-13", ps)
	}
	if !pe.Equal(time.Date(2026, 7, 15, 9, 15, 0, 0, time.UTC)) {
		t.Fatalf("prev end = %v; want the same elapsed offset (Wed 09:15)", pe)
	}
}

func TestWeekLabel(t *testing.T) {
	got := teacher.WeekLabel(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC))
	want := "第 30 周（7.20–7.26）"
	if got != want {
		t.Fatalf("WeekLabel = %q; want %q", got, want)
	}
}
