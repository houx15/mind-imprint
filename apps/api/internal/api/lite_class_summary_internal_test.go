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
}
