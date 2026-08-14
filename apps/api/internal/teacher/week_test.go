package teacher

import (
	"testing"
	"time"
)

func TestWeekWindowStartsMondayUTC(t *testing.T) {
	// 2026-07-24 is a Friday, 15:30 in +08:00 → 07:30 UTC the same day.
	now := time.Date(2026, 7, 24, 15, 30, 0, 0, time.FixedZone("CST", 8*3600))
	start, end := WeekWindow(now)
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
	start, _ := WeekWindow(now)
	if !start.Equal(now) {
		t.Fatalf("start = %v; want the Monday itself", start)
	}
}

func TestWeekWindowSundayStaysInTheSameWeek(t *testing.T) {
	now := time.Date(2026, 7, 26, 23, 59, 0, 0, time.UTC)
	start, end := WeekWindow(now)
	if !start.Equal(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start = %v; want 2026-07-20", start)
	}
	if !now.Before(end) {
		t.Fatal("Sunday 23:59 must fall inside its own week window")
	}
}

func TestPrevWindowIsTheSameElapsedOffset(t *testing.T) {
	now := time.Date(2026, 7, 22, 9, 15, 0, 0, time.UTC) // Wednesday 09:15
	ps, pe := PrevWindow(now)
	if !ps.Equal(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("prev start = %v; want 2026-07-13", ps)
	}
	if !pe.Equal(time.Date(2026, 7, 15, 9, 15, 0, 0, time.UTC)) {
		t.Fatalf("prev end = %v; want the same elapsed offset (Wed 09:15)", pe)
	}
}

func TestWeekLabel(t *testing.T) {
	got := WeekLabel(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC))
	want := "第 30 周（7.20–7.26）"
	if got != want {
		t.Fatalf("WeekLabel = %q; want %q", got, want)
	}
}

func TestLastCompletedWeekStart(t *testing.T) {
	// Wed 2026-08-12 UTC → current week Mon 2026-08-10 → last completed Mon 2026-08-03.
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	got := LastCompletedWeekStart(now)
	want := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("LastCompletedWeekStart = %v, want %v", got, want)
	}
}

func TestCompletedWeekWindows(t *testing.T) {
	ws := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	start, end, ps, pe := CompletedWeekWindows(ws)
	if !start.Equal(ws) || !end.Equal(ws.AddDate(0, 0, 7)) {
		t.Fatalf("window = [%v,%v)", start, end)
	}
	if !ps.Equal(ws.AddDate(0, 0, -7)) || !pe.Equal(ws) {
		t.Fatalf("prev = [%v,%v)", ps, pe)
	}
}

func TestValidateCompletedWeekStart(t *testing.T) {
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	if err := ValidateCompletedWeekStart(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), now); err != nil {
		t.Fatalf("valid last-completed week rejected: %v", err)
	}
	// Current (in-progress) week is not viewable.
	if err := ValidateCompletedWeekStart(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), now); err == nil {
		t.Fatal("current week accepted")
	}
	// Not a Monday.
	if err := ValidateCompletedWeekStart(time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC), now); err == nil {
		t.Fatal("non-Monday accepted")
	}
}

func TestIsLatestCompletedWeek(t *testing.T) {
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	if !IsLatestCompletedWeek(time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC), now) {
		t.Fatal("last completed week should be latest")
	}
	if IsLatestCompletedWeek(time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC), now) {
		t.Fatal("older week should not be latest")
	}
}
