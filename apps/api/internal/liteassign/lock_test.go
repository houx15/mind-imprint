package liteassign

import (
	"testing"
	"time"
)

func TestLocked(t *testing.T) {
	due := time.Date(2026, 9, 20, 22, 0, 0, 0, time.UTC)
	returnDue := due.Add(72 * time.Hour)
	returnedAt := due.Add(time.Hour)
	at := func(t time.Time) *time.Time { return &t }

	cases := []struct {
		name string
		f    LockFacts
		now  time.Time
		want bool
	}{
		{"not homework, past due", LockFacts{Homework: false, HasVersion: true, DueAt: due}, due.Add(time.Hour), false},
		{"homework, no version, past due", LockFacts{Homework: true, HasVersion: false, DueAt: due}, due.Add(time.Hour), false},
		{"homework, version, before due", LockFacts{Homework: true, HasVersion: true, DueAt: due}, due.Add(-time.Hour), false},
		{"homework, version, exactly at due", LockFacts{Homework: true, HasVersion: true, DueAt: due}, due, false},
		{"homework, version, one second past due", LockFacts{Homework: true, HasVersion: true, DueAt: due}, due.Add(time.Second), true},
		{"returned, before return due", LockFacts{Homework: true, HasVersion: true, DueAt: due, ReturnedAt: at(returnedAt), ReturnDueAt: at(returnDue)}, returnDue.Add(-time.Hour), false},
		{"returned, past return due", LockFacts{Homework: true, HasVersion: true, DueAt: due, ReturnedAt: at(returnedAt), ReturnDueAt: at(returnDue)}, returnDue.Add(time.Second), true},
		{"return due without returned_at is ignored", LockFacts{Homework: true, HasVersion: true, DueAt: due, ReturnDueAt: at(returnDue)}, due.Add(time.Hour), true},
	}
	for _, c := range cases {
		if got := Locked(c.f, c.now); got != c.want {
			t.Errorf("%s: Locked = %v, want %v", c.name, got, c.want)
		}
		wantReason := ""
		if c.want {
			wantReason = LockReasonPastDue
		}
		if got := LockReason(c.f, c.now); got != wantReason {
			t.Errorf("%s: LockReason = %q, want %q", c.name, got, wantReason)
		}
	}
}

func TestEffectiveDue(t *testing.T) {
	due := time.Date(2026, 9, 20, 22, 0, 0, 0, time.UTC)
	returnDue := due.Add(48 * time.Hour)
	returnedAt := due
	if got := EffectiveDue(LockFacts{DueAt: due}); !got.Equal(due) {
		t.Fatalf("not returned: %v", got)
	}
	if got := EffectiveDue(LockFacts{DueAt: due, ReturnedAt: &returnedAt, ReturnDueAt: &returnDue}); !got.Equal(returnDue) {
		t.Fatalf("returned: %v", got)
	}
}
