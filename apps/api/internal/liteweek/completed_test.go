package liteweek

import (
	"errors"
	"testing"
	"time"
)

func TestLatestCompleted(t *testing.T) {
	// Wednesday 2026-09-16 10:00 Beijing → last week started Monday 2026-09-07.
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, Beijing)
	want := time.Date(2026, 9, 7, 0, 0, 0, 0, Beijing)
	if got := LatestCompleted(now); !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// Monday 00:00:30 Beijing: the week that just ended is the latest completed.
	now = time.Date(2026, 9, 14, 0, 0, 30, 0, Beijing)
	if got := LatestCompleted(now); !got.Equal(want) {
		t.Fatalf("monday: got %v want %v", got, want)
	}
}

// Sunday 23:59:59 Beijing is still inside the current week, so the latest
// completed week is the one before it.
func TestLatestCompletedSundayLastSecond(t *testing.T) {
	now := time.Date(2026, 9, 13, 23, 59, 59, 0, Beijing)
	want := time.Date(2026, 8, 31, 0, 0, 0, 0, Beijing)
	if got := LatestCompleted(now); !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// An RFC3339 value with the Beijing offset names the same Monday 00:00 as
// the date form.
func TestParseWeekStartRFC3339Beijing(t *testing.T) {
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, Beijing)
	got, err := ParseWeekStart("2026-08-31T00:00:00+08:00", now)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if want := time.Date(2026, 8, 31, 0, 0, 0, 0, Beijing); !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestParseWeekStart(t *testing.T) {
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, Beijing)
	if got, err := ParseWeekStart("", now); err != nil || !got.Equal(LatestCompleted(now)) {
		t.Fatalf("empty: %v %v", got, err)
	}
	if got, err := ParseWeekStart("2026-08-31", now); err != nil || got.Day() != 31 {
		t.Fatalf("date form: %v %v", got, err)
	}
	for _, bad := range []string{"2026-09-14", "2026-09-08", "garbage", "2026-09-07T00:00:00Z"} {
		// current week; a Tuesday; unparsable; UTC midnight is 08:00 Beijing, not a Beijing Monday 00:00
		if _, err := ParseWeekStart(bad, now); !errors.Is(err, ErrBadWeek) {
			t.Errorf("%s: want ErrBadWeek, got %v", bad, err)
		}
	}
}

func TestLabel(t *testing.T) {
	got := Label(time.Date(2026, 9, 7, 0, 0, 0, 0, Beijing))
	if got != "第 37 周（9.7–9.13）" {
		t.Fatalf("label = %q", got)
	}
}
