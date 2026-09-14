package liteweek

import (
	"testing"
	"time"
)

func TestDayUsesBeijingNotUTC(t *testing.T) {
	// 2026-09-13 16:30 UTC = 2026-09-14 00:30 Beijing.
	got := Day(time.Date(2026, 9, 13, 16, 30, 0, 0, time.UTC))
	if got.Year() != 2026 || got.Month() != 9 || got.Day() != 14 {
		t.Fatalf("Day = %v, want 2026-09-14", got)
	}
	if got.Hour() != 0 || got.Minute() != 0 {
		t.Fatalf("Day not at midnight: %v", got)
	}
}

func TestDayBoundaryOneMinuteApart(t *testing.T) {
	before := Day(time.Date(2026, 9, 14, 23, 59, 30, 0, Beijing))
	after := Day(time.Date(2026, 9, 15, 0, 0, 30, 0, Beijing))
	if before.Equal(after) {
		t.Fatalf("23:59:30 and 00:00:30 Beijing landed on the same day %v", before)
	}
}

func TestWeekStartIsMondayBeijing(t *testing.T) {
	// Sunday 2026-09-20 23:59 Beijing belongs to the week starting Monday 2026-09-14.
	got := WeekStart(time.Date(2026, 9, 20, 23, 59, 0, 0, Beijing))
	want := time.Date(2026, 9, 14, 0, 0, 0, 0, Beijing)
	if !got.Equal(want) {
		t.Fatalf("WeekStart = %v, want %v", got, want)
	}
	// Monday 00:00 is its own week's start.
	if m := WeekStart(want); !m.Equal(want) {
		t.Fatalf("WeekStart(Monday 00:00) = %v, want %v", m, want)
	}
}
