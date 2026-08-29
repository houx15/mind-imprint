package api

import (
	"net/http/httptest"
	"sort"
	"testing"
)

// 我的写作 is ordered by when each writing last MATTERED, not by when it was
// created — mirrors TestReadingRecency_* (readings_list_limit_test.go)
// exactly. The list query's own ORDER BY is `atom.created_at`, so a writing
// started in June and drafted all week would sit below one started
// yesterday and never touched — and once the list is CUT to the most recent
// N, that stops being cosmetic and starts dropping her live work off the end.
func TestWritingRecency_PrefersActivityOverCreation(t *testing.T) {
	stamp := func(s string) *string { return &s }

	rows := []writingDTO{
		{ID: "stale-but-new", LastActivityAt: "2026-08-20T00:00:00Z"},
		{ID: "drafted-all-week", LastActivityAt: "2026-08-27T00:00:00Z"},
		// A finished writing is placed by when she FINISHED it, not by the
		// last write into it — those are the same moment, but only the first
		// is the fact she is looking for in 已完成.
		{ID: "finished-today", LastActivityAt: "2026-01-01T00:00:00Z", FinishedAt: stamp("2026-08-28T00:00:00Z")},
	}
	sort.SliceStable(rows, func(i, j int) bool { return writingRecency(rows[i]) > writingRecency(rows[j]) })

	want := []string{"finished-today", "drafted-all-week", "stale-but-new"}
	for i, id := range want {
		if rows[i].ID != id {
			t.Fatalf("position %d = %q, want %q (full order: %v)", i, rows[i].ID, id, writingIDs(rows))
		}
	}
}

// An empty `finishedAt` string must NOT be treated as a finish stamp — it
// sorts before every real timestamp, which would bury the row.
func TestWritingRecency_IgnoresAnEmptyFinishStamp(t *testing.T) {
	empty := ""
	d := writingDTO{LastActivityAt: "2026-08-27T00:00:00Z", FinishedAt: &empty}
	if got := writingRecency(d); got != "2026-08-27T00:00:00Z" {
		t.Fatalf("writingRecency = %q, want the activity stamp", got)
	}
}

func TestWritingListLimit(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  int
	}{
		{"", writingListDefaultLimit},
		{"?limit=10", 10},
		{"?limit=1", 1},
		// Garbage, zero and negatives take the default rather than narrowing
		// her history to nothing: a malformed query string is our problem to
		// absorb, not hers to see as an empty shelf.
		{"?limit=abc", writingListDefaultLimit},
		{"?limit=0", writingListDefaultLimit},
		{"?limit=-5", writingListDefaultLimit},
		// And the ceiling holds, so `?limit=100000` cannot be used to ask the
		// API to serialize everything she has ever written.
		{"?limit=100000", writingListMaxLimit},
	} {
		t.Run(tc.query, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/v1/writings"+tc.query, nil)
			if got := writingListLimit(r); got != tc.want {
				t.Fatalf("writingListLimit(%q) = %d, want %d", tc.query, got, tc.want)
			}
		})
	}
}

func writingIDs(rows []writingDTO) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}
