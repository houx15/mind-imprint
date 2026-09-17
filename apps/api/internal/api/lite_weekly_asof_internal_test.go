package api

import (
	"testing"
	"time"
)

func TestLiteWeekAssignmentOutcome(t *testing.T) {
	end := time.Date(2026, 9, 20, 16, 0, 0, 0, time.UTC) // Monday 00:00 Beijing
	thursday := time.Date(2026, 9, 17, 14, 0, 0, 0, time.UTC)
	friday := time.Date(2026, 9, 18, 13, 0, 0, 0, time.UTC)
	wed := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)
	early := thursday.Add(-time.Hour)
	for _, tc := range []struct {
		name     string
		started  bool
		finished *time.Time
		due, now time.Time
		want     string
	}{
		{"finished before a deadline still ahead", true, &early, friday, thursday, ""},
		{"not started, deadline still ahead", false, nil, friday, thursday, ""},
		{"finished before a deadline that passed", true, &early, wed.Add(48 * time.Hour), end.Add(time.Hour), "done"},
		{"deadline passed, not finished", true, nil, wed, thursday, "overdue"},
		{"finished after the deadline, in the week", true, &early, wed, thursday, "done_late"},
		{"finished after the week ended", true, ptrTime(end.Add(time.Hour)), wed, end.Add(48 * time.Hour), "overdue"},
	} {
		if got := liteWeekAssignmentOutcome(tc.started, tc.finished, tc.due, end, tc.now); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestLiteWeekAsOf(t *testing.T) {
	end := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	thursday := end.Add(-3 * 24 * time.Hour)
	if got := liteWeekAsOf(end, thursday); !got.Equal(thursday) {
		t.Fatalf("running week: %v, want now %v", got, thursday)
	}
	later := end.Add(48 * time.Hour)
	if got := liteWeekAsOf(end, later); !got.Equal(end) {
		t.Fatalf("finished week: %v, want its end %v", got, end)
	}
}
