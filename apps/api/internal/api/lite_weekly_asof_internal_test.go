package api

import (
	"testing"
	"time"
)

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
