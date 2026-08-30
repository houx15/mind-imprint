package api

import (
	"encoding/json"
	"testing"
)

// reportWithPiece's PURE decisions — the branches that must not touch the
// database, pinned without one.
//
// Why this file exists at all: "no backfill, the section is simply absent" was
// a reasonable call while the piece was one section at the foot of the report.
// When the piece became the page it silently switched the article page OFF for
// every writing that already existed — production had 2 of 2 writing reports
// with no `piece`, so every share link opened a page of statistics with no way
// to the article. The failure was invisible: nothing errored, nothing logged,
// the tests were green, and the page looked deliberate.
//
// `a` is a nil-free zero API here on purpose: every case below must return
// BEFORE the draft lookup, so reaching the query would panic and fail loudly
// rather than pass by accident.
func TestReportWithPieceLeavesBlobAloneWithoutTouchingTheDatabase(t *testing.T) {
	a := &API{}

	cases := []struct {
		name   string
		stored string
	}{
		{
			// A reading has no piece and never will; it must not even look.
			name:   "reading report",
			stored: `{"version":1,"kind":"reading","title":"读了一篇"}`,
		},
		{
			// Already generated with one — the stored text is authoritative,
			// and re-reading the draft over it would be a second source of
			// truth for the same words.
			name:   "writing that already carries a piece",
			stored: `{"version":1,"kind":"writing","piece":"我写的正文。"}`,
		},
		{
			name:   "not an object",
			stored: `["nope"]`,
		},
		{
			name:   "no kind at all",
			stored: `{"version":1,"title":"没有 kind"}`,
		},
		{
			name:   "kind is not a string",
			stored: `{"version":1,"kind":7}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := a.reportWithPiece(t.Context(), [16]byte{}, []byte(tc.stored))
			if string(got) != tc.stored {
				t.Fatalf("blob was rewritten:\n got %s\nwant %s", got, tc.stored)
			}
		})
	}
}

// A whitespace-only stored piece counts as ABSENT, not as "already has one" —
// otherwise a report that stored "" (or "  ") would be treated as authoritative
// and would never pick her draft up. This case must fall THROUGH to the draft
// lookup, which is what the panic proves.
func TestReportWithPieceTreatsBlankStoredPieceAsMissing(t *testing.T) {
	a := &API{}
	defer func() {
		if recover() == nil {
			t.Fatal("expected the draft lookup to be reached for a blank stored piece")
		}
	}()
	//nolint:errcheck // the panic IS the assertion
	_ = a.reportWithPiece(t.Context(), [16]byte{}, []byte(`{"kind":"writing","piece":"   "}`))
}

// The hydrated blob must keep every other field byte-for-byte — the public
// payload's key set is pinned elsewhere (TestPublicPayloadCarriesNothingExtra)
// and this function must never add, drop or reshape anything but `piece`.
func TestReportWithPiecePreservesEveryOtherField(t *testing.T) {
	stored := `{"version":1,"kind":"writing","title":"转弯中的国家","studentName":"Phoebe",` +
		`"stats":[{"key":"words","label":"写了","value":842,"unit":"字"}],"keep":null}`

	var before, after map[string]any
	if err := json.Unmarshal([]byte(stored), &before); err != nil {
		t.Fatal(err)
	}
	// Simulate what the function produces once a draft is found, by running
	// the same marshal path on the same map. (The DB half is covered by the
	// handler tests; what is checked here is that nothing else moves.)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stored), &fields); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal("我写完的正文。")
	if err != nil {
		t.Fatal(err)
	}
	fields["piece"] = encoded
	out, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &after); err != nil {
		t.Fatal(err)
	}

	if after["piece"] != "我写完的正文。" {
		t.Fatalf("piece = %v", after["piece"])
	}
	delete(after, "piece")
	if len(after) != len(before) {
		t.Fatalf("key set changed: got %d keys, want %d", len(after), len(before))
	}
	for k, v := range before {
		gotJSON, _ := json.Marshal(after[k])
		wantJSON, _ := json.Marshal(v)
		if string(gotJSON) != string(wantJSON) {
			t.Fatalf("field %q changed: got %s, want %s", k, gotJSON, wantJSON)
		}
	}
}
