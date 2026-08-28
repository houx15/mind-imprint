package api

import (
	"net/http/httptest"
	"sort"
	"testing"
)

// 我的阅读 is ordered by when each reading last MATTERED, not by when it was
// created. The list query's own ORDER BY is `atom.created_at`, so a reading
// opened in June and read all week would sit below one opened yesterday and
// never touched — and once the list is CUT to the most recent N, that stops
// being a cosmetic ordering问题 and starts dropping her live work off the end.
func TestReadingRecency_PrefersActivityOverCreation(t *testing.T) {
	stamp := func(s string) *string { return &s }

	rows := []readingDTO{
		{ID: "stale-but-new", LastActivityAt: "2026-08-20T00:00:00Z"},
		{ID: "read-all-week", LastActivityAt: "2026-08-27T00:00:00Z"},
		// A finished reading is placed by when she FINISHED it, not by the
		// last write into it — those are the same moment, but only the first
		// is the fact she is looking for in 已完成.
		{ID: "finished-today", LastActivityAt: "2026-01-01T00:00:00Z", FinishedAt: stamp("2026-08-28T00:00:00Z")},
	}
	sort.SliceStable(rows, func(i, j int) bool { return readingRecency(rows[i]) > readingRecency(rows[j]) })

	want := []string{"finished-today", "read-all-week", "stale-but-new"}
	for i, id := range want {
		if rows[i].ID != id {
			t.Fatalf("position %d = %q, want %q (full order: %v)", i, rows[i].ID, id, ids(rows))
		}
	}
}

// An empty `finishedAt` string must NOT be treated as a finish stamp — it
// sorts before every real timestamp, which would bury the row.
func TestReadingRecency_IgnoresAnEmptyFinishStamp(t *testing.T) {
	empty := ""
	d := readingDTO{LastActivityAt: "2026-08-27T00:00:00Z", FinishedAt: &empty}
	if got := readingRecency(d); got != "2026-08-27T00:00:00Z" {
		t.Fatalf("readingRecency = %q, want the activity stamp", got)
	}
}

func TestReadingListLimit(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  int
	}{
		{"", readingListDefaultLimit},
		{"?limit=10", 10},
		{"?limit=1", 1},
		// Garbage, zero and negatives take the default rather than narrowing
		// her history to nothing: a malformed query string is our problem to
		// absorb, not hers to see as an empty shelf.
		{"?limit=abc", readingListDefaultLimit},
		{"?limit=0", readingListDefaultLimit},
		{"?limit=-5", readingListDefaultLimit},
		// And the ceiling holds, so `?limit=100000` cannot be used to ask the
		// API to serialize everything she has ever read.
		{"?limit=100000", readingListMaxLimit},
	} {
		t.Run(tc.query, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/v1/readings"+tc.query, nil)
			if got := readingListLimit(r); got != tc.want {
				t.Fatalf("readingListLimit(%q) = %d, want %d", tc.query, got, tc.want)
			}
		})
	}
}

func ids(rows []readingDTO) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}
