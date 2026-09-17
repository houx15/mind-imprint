package api

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/liteworkspace"
)

// TestLiteClassSummaryCacheKeyBeijingDate is a pure test of the cache key
// function: no DB, no server, just the function. It exists because the key
// MUST use liteworkspace.BeijingOffset, never time.LoadLocation — a
// distroless runtime carries no tzdata and LoadLocation fails silently into
// UTC there, which would put the same class's morning requests into
// yesterday's bucket.
func TestLiteClassSummaryCacheKeyBeijingDate(t *testing.T) {
	classID := uuid.New()
	roster := []liteworkspace.Student{{ID: "s1", ActiveDaysThisWeek: 1}}

	// 2026-09-17 16:30 UTC is 2026-09-18 00:30 Beijing — a moment that is
	// "yesterday" in UTC and "today" in Beijing. The key must read Beijing.
	utc := time.Date(2026, 9, 17, 16, 30, 0, 0, time.UTC)
	key := liteClassSummaryCacheKey(classID, utc, roster)
	if want := classID.String() + "|2026-09-18|"; key[:len(want)] != want {
		t.Fatalf("key = %q, want prefix %q (Beijing date, not UTC date)", key, want)
	}

	// The other side of the same boundary: 2026-09-17 15:59 UTC is still
	// 2026-09-17 23:59 Beijing.
	justBefore := time.Date(2026, 9, 17, 15, 59, 0, 0, time.UTC)
	keyBefore := liteClassSummaryCacheKey(classID, justBefore, roster)
	if want := classID.String() + "|2026-09-17|"; keyBefore[:len(want)] != want {
		t.Fatalf("key = %q, want prefix %q", keyBefore, want)
	}

	// A key built directly from a Beijing wall-clock time.Time (as
	// time.FixedZone gives us — no tzdata needed) must agree with the UTC
	// instant it names.
	beijing := utc.In(liteworkspace.BeijingOffset)
	if got := liteClassSummaryCacheKey(classID, beijing, roster); got != key {
		t.Fatalf("key from a Beijing-zoned time.Time = %q, want %q", got, key)
	}
}

// TestLiteClassSummaryCacheKeyRosterFingerprint — two rosters that differ in
// any one student's activity fields must produce different keys (this is
// what "roster change → recompute" means at the key level); the same roster
// data must produce the same key regardless of row order.
func TestLiteClassSummaryCacheKeyRosterFingerprint(t *testing.T) {
	classID := uuid.New()
	now := time.Now()

	a := []liteworkspace.Student{
		{ID: "s1", ActiveDaysThisWeek: 1, OverdueAssignments: 0, WritingsDone: 2},
		{ID: "s2", ActiveDaysThisWeek: 0, OverdueAssignments: 1, WritingsDone: 0},
	}
	b := []liteworkspace.Student{
		{ID: "s1", ActiveDaysThisWeek: 1, OverdueAssignments: 0, WritingsDone: 3}, // WritingsDone changed
		{ID: "s2", ActiveDaysThisWeek: 0, OverdueAssignments: 1, WritingsDone: 0},
	}
	reordered := []liteworkspace.Student{a[1], a[0]}

	keyA := liteClassSummaryCacheKey(classID, now, a)
	keyB := liteClassSummaryCacheKey(classID, now, b)
	keyReordered := liteClassSummaryCacheKey(classID, now, reordered)

	if keyA == keyB {
		t.Fatalf("changing one student's WritingsDone did not change the key: %q", keyA)
	}
	if keyA != keyReordered {
		t.Fatalf("row order changed the key: %q vs %q", keyA, keyReordered)
	}

	// Setting a gender changes the pronouns the summary may use.
	gendered := []liteworkspace.Student{a[0], a[1]}
	gendered[0].Gender = liteworkspace.GenderFemale
	if liteClassSummaryCacheKey(classID, now, gendered) == keyA {
		t.Fatalf("setting a gender did not change the key: %q", keyA)
	}

	// A finished reading (Progress) refreshes the summary the same day.
	read := []liteworkspace.Student{a[0], a[1]}
	read[1].Progress = "r1/1 w0 p0/0 t3"
	if liteClassSummaryCacheKey(classID, now, read) == keyA {
		t.Fatalf("finishing a reading did not change the key: %q", keyA)
	}
}

// TestLiteClassSummaryStorePanicRecovers pins the panic path: a panic
// inside fn used to skip close(f.done) and delete(s.inflight, key), so every
// later request for that key blocked forever. It must instead: (1) hand the
// caller that started the flight an error, (2) release a concurrent waiter
// that joined the SAME flight with an error too, and (3) let a later call for
// the same key run fn again rather than staying stuck.
func TestLiteClassSummaryStorePanicRecovers(t *testing.T) {
	s := newLiteClassSummaryStore(4)
	const key = "k"
	// 🚨 The panicking flight must not finish before the waiter has JOINED it.
	// Closing `started` alone did not guarantee that: fn closed it and panicked
	// in the same breath, the flight was over before the goroutine got to
	// resolve, and the waiter started a flight of its own — 23 failures in 30
	// runs (2026-09-17). onJoin fires inside resolve's join branch, so fn
	// waits for it.
	joined := make(chan struct{})
	s.onJoin = func() { close(joined) }

	// A concurrent caller for the same key, launched only once the panicking
	// call is actually under way.
	started := make(chan struct{})
	waiterErr := make(chan error, 1)
	go func() {
		<-started
		_, _, err := s.resolve(key, func() (liteClassSummaryEntry, error) {
			t.Error("a concurrent waiter must join the in-flight call, not run fn itself")
			return liteClassSummaryEntry{}, nil
		})
		waiterErr <- err
	}()

	_, _, err := s.resolve(key, func() (liteClassSummaryEntry, error) {
		close(started)
		<-joined
		panic("boom")
	})
	if err == nil {
		t.Fatal("resolve returned a nil error for a panicking fn")
	}

	select {
	case werr := <-waiterErr:
		if werr == nil {
			t.Fatal("the waiter joined the panicking flight but got a nil error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the waiter never returned — the panic left the key stuck")
	}

	// The key must not be permanently in-flight, and a panic must never be
	// cached: a later call for the same key runs fn again.
	calls := 0
	entry, cached, err2 := s.resolve(key, func() (liteClassSummaryEntry, error) {
		calls++
		return liteClassSummaryEntry{Summary: "ok"}, nil
	})
	if err2 != nil || cached || calls != 1 || entry.Summary != "ok" {
		t.Fatalf("call after a panic: entry=%+v cached=%v err=%v calls=%d", entry, cached, err2, calls)
	}
}
