package api

import (
	"encoding/json"
	"testing"
)

// The read-time hydration of a stored writing report.
//
// Why this exists: a report is generated ONCE and stored as a JSON blob, then
// re-served verbatim forever. "No backfill, the section is simply absent" was
// a fine call while `piece` was one section at the foot of the page. When the
// piece BECAME the page, that decision silently switched the article page off
// for every writing that already existed — production had 2 of 2 writing
// reports with no `piece`, so every share link opened a page of statistics
// with no way to the article at all. Nothing errored, nothing logged, the
// tests were green, and the page looked deliberate.
//
// The two rules below are ASYMMETRIC on purpose, which is the thing most
// likely to get "tidied" into one and quietly broken.

// piece: only when the blob has none. A report generated after the field
// shipped is authoritative — reading the draft over it would make two sources
// of truth for the same words.
func TestMergeLiveWritingFieldsFillsPieceOnlyWhenMissing(t *testing.T) {
	cases := []struct {
		name      string
		stored    string
		draftBody string
		want      string // expected piece afterwards
	}{
		{
			name:      "absent — filled from the draft",
			stored:    `{"kind":"writing","title":"t"}`,
			draftBody: "我写完的正文。",
			want:      "我写完的正文。",
		},
		{
			// A stored "" (or "   ") counts as ABSENT — treating it as present
			// is what would let a blank field pin itself forever.
			name:      "blank — treated as absent",
			stored:    `{"kind":"writing","title":"t","piece":"   "}`,
			draftBody: "我写完的正文。",
			want:      "我写完的正文。",
		},
		{
			name:      "already present — the blob wins",
			stored:    `{"kind":"writing","title":"t","piece":"报告里存着的正文。"}`,
			draftBody: "草稿里现在的正文。",
			want:      "报告里存着的正文。",
		},
		{
			name:      "no draft to draw on — left absent, not blanked",
			stored:    `{"kind":"writing","title":"t","piece":"报告里存着的正文。"}`,
			draftBody: "   ",
			want:      "报告里存着的正文。",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := decode(t, tc.stored)
			mergeLiveWritingFields(fields, tc.draftBody, "", "t")
			if got := str(t, fields["piece"]); got != tc.want {
				t.Fatalf("piece = %q, want %q", got, tc.want)
			}
		})
	}
}

// title: ALWAYS from the live row. She can still rename a finished writing,
// and 给这篇起个名字 only asks at 完成这篇 — so a piece finished before that
// flow shipped still carries her raw 「我想写：…」 sentence in its blob, where a
// rename would never reach it.
func TestMergeLiveWritingFieldsAlwaysTakesTheLiveTitle(t *testing.T) {
	fields := decode(t, `{"kind":"writing","title":"我想写一篇论证文，说说学校该不该允许学生课间用手机——我自己观察到…"}`)

	changed := mergeLiveWritingFields(fields, "", "", "课间十分钟")

	if !changed {
		t.Fatal("a renamed title must report a change")
	}
	if got := str(t, fields["title"]); got != "课间十分钟" {
		t.Fatalf("title = %q", got)
	}
}

// …but an empty live title never blanks a stored one: a row we could not read
// (or a writing with no title at all) must not erase the report's headline.
func TestMergeLiveWritingFieldsNeverBlanksTheTitle(t *testing.T) {
	fields := decode(t, `{"kind":"writing","title":"转弯中的国家"}`)

	if mergeLiveWritingFields(fields, "", "", "   ") {
		t.Fatal("an empty live title must not count as a change")
	}
	if got := str(t, fields["title"]); got != "转弯中的国家" {
		t.Fatalf("title = %q", got)
	}
}

// An unchanged title is not a change — otherwise every read of every report
// would re-marshal the whole blob for nothing.
func TestMergeLiveWritingFieldsIsANoOpWhenNothingDiffers(t *testing.T) {
	fields := decode(t, `{"kind":"writing","title":"转弯中的国家","piece":"正文。"}`)

	if mergeLiveWritingFields(fields, "正文。", "", "转弯中的国家") {
		t.Fatal("identical live values must not report a change")
	}
}

// Nothing but `piece` and `title` may move. The public payload's key set is
// pinned elsewhere (TestPublicPayloadCarriesNothingExtra) and this must never
// add, drop or reshape another field.
func TestMergeLiveWritingFieldsTouchesNothingElse(t *testing.T) {
	stored := `{"version":1,"kind":"writing","title":"旧标题","studentName":"Phoebe",` +
		`"stats":[{"key":"words","label":"写了","value":842,"unit":"字"}],"keep":null,"gains":["一"]}`
	before := decode(t, stored)
	after := decode(t, stored)

	mergeLiveWritingFields(after, "正文。", "", "新标题")

	if len(after) != len(before)+1 { // +1 for the added piece
		t.Fatalf("key count = %d, want %d", len(after), len(before)+1)
	}
	for k, v := range before {
		if k == "title" || k == "piece" {
			continue
		}
		if string(after[k]) != string(v) {
			t.Fatalf("field %q changed: got %s, want %s", k, after[k], v)
		}
	}
}

// The kind gate, on the real entry point: a reading has no piece and no
// writing row, and must return byte-identical without any lookup at all. `a`
// is a zero API on purpose — reaching the database would panic and fail
// loudly rather than pass by accident.
func TestReportWithPieceIgnoresAnythingThatIsNotAWriting(t *testing.T) {
	a := &API{}
	for _, stored := range []string{
		`{"version":1,"kind":"reading","title":"读了一篇"}`,
		`["not an object"]`,
		`{"version":1,"title":"没有 kind"}`,
		`{"version":1,"kind":7}`,
	} {
		got := a.reportWithPiece(t.Context(), [16]byte{}, []byte(stored))
		if string(got) != stored {
			t.Fatalf("blob was rewritten:\n got %s\nwant %s", got, stored)
		}
	}
}

// The title beside the piece is the submitted version's; a rename made while
// revising only reaches the report after 完成这篇. No version (or a blank
// version title) falls back to the live title.
func TestWritingPieceTitle(t *testing.T) {
	cases := []struct {
		name, live, version string
		hasVersion          bool
		want                string
	}{
		{"version exists — its title wins over an unsubmitted rename", "修改中的标题", "雨", true, "雨"},
		{"no version — live title", "雨", "", false, "雨"},
		{"blank version title — live title", "雨", "  ", true, "雨"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := writingPieceTitle(tc.live, tc.version, tc.hasVersion); got != tc.want {
				t.Fatalf("writingPieceTitle = %q, want %q", got, tc.want)
			}
		})
	}
}

func decode(t *testing.T, s string) map[string]json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func str(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("not a string: %s", raw)
	}
	return s
}

// Once versions exist (0153), the piece is the latest submitted version:
// she can edit after 完成, and the report must show what she submitted, not
// what the report stored at generation time or her unsubmitted draft.
func TestMergeLiveWritingFieldsLatestVersionWins(t *testing.T) {
	cases := []struct {
		name        string
		stored      string
		draftBody   string
		versionBody string
		want        string
		wantChanged bool
	}{
		{"version replaces the stored piece", `{"kind":"writing","title":"t","piece":"第一版。"}`, "还没提交的修改。", "第二版。", "第二版。", true},
		{"version fills a missing piece", `{"kind":"writing","title":"t"}`, "草稿。", "第一版。", "第一版。", true},
		{"same text is not a change", `{"kind":"writing","title":"t","piece":"第二版。"}`, "", "第二版。", "第二版。", false},
		{"blank version falls back to the old rule", `{"kind":"writing","title":"t","piece":"报告里存着的正文。"}`, "草稿。", "   ", "报告里存着的正文。", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := decode(t, tc.stored)
			changed := mergeLiveWritingFields(fields, tc.draftBody, tc.versionBody, "t")
			if got := str(t, fields["piece"]); got != tc.want {
				t.Fatalf("piece = %q, want %q", got, tc.want)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tc.wantChanged)
			}
		})
	}
}
