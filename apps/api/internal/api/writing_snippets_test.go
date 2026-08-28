package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// writing_snippets_test.go — Task 6: 段落 (paragraphs). Reuses
// writing_outline_test.go's createWritingAtomHTTP — same package (api_test),
// same lite-writing test harness.

type writingSnippetItem struct {
	ID             string  `json:"id"`
	OutlineID      *string `json:"outlineId"`
	OutlineHeading string  `json:"outlineHeading"`
	Position       int32   `json:"position"`
	Text           string  `json:"text"`
	UpdatedAt      string  `json:"updatedAt"`
}

func getWritingSnippetsHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id string) []writingSnippetItem {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("GET", "/api/v1/writings/"+id+"/snippets", nil), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET snippets = %d, want 200; body=%s", rec.Code, rec.Body)
	}
	var out struct {
		Snippets []writingSnippetItem `json:"snippets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode snippets: %v — body=%s", err, rec.Body)
	}
	return out.Snippets
}

func putWritingSnippetsHTTP(t *testing.T, h http.Handler, cookie *http.Cookie, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, withCookie(httptest.NewRequest("PUT", "/api/v1/writings/"+id+"/snippets", strings.NewReader(body)), cookie))
	return rec
}

// TestWritingSnippets_PutUpsertsByPositionNotFullReplace — the PUT contract
// this task's brief describes: "one fragment at a time", never a wholesale
// wipe of positions the caller did not mention (unlike outline's PUT, which
// IS a full replace — see writing_outline.go's comment on the distinction).
func TestWritingSnippets_PutUpsertsByPositionNotFullReplace(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"第一段的内容"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put position 0 = %d; body=%s", rec.Code, rec.Body)
	}
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":1,"text":"第二段的内容"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("put position 1 = %d; body=%s", rec.Code, rec.Body)
	}
	after := getWritingSnippetsHTTP(t, h, cookie, id)
	if len(after) != 2 {
		t.Fatalf("snippets after two independent PUTs = %d, want 2 (upsert-by-position, not full replace): %+v", len(after), after)
	}

	var firstID string
	for _, s := range after {
		if s.Position == 0 {
			firstID = s.ID
		}
	}
	if firstID == "" {
		t.Fatalf("position 0 missing from %+v", after)
	}

	// Re-PUT position 0 with new text: edits IN PLACE (same id), same as
	// UpsertWritingSnippet's documented (atom_id, position) conflict target.
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, `{"snippets":[{"position":0,"text":"修改后的第一段"}]}`); rec.Code != http.StatusOK {
		t.Fatalf("re-put position 0 = %d; body=%s", rec.Code, rec.Body)
	}
	after2 := getWritingSnippetsHTTP(t, h, cookie, id)
	if len(after2) != 2 {
		t.Fatalf("snippets after re-put = %d, want still 2 (edit in place, no duplicate): %+v", len(after2), after2)
	}
	var found bool
	for _, s := range after2 {
		if s.Position == 0 {
			found = true
			if s.ID != firstID {
				t.Fatalf("position 0's id changed from %q to %q — re-PUT must edit in place, not mint a new row", firstID, s.ID)
			}
			if s.Text != "修改后的第一段" {
				t.Fatalf("position 0 text = %q, want the re-PUT text", s.Text)
			}
		}
	}
	if !found {
		t.Fatalf("position 0 vanished after re-put: %+v", after2)
	}
	// Position 1 must be untouched — this is what makes PUT NOT a full
	// replace, unlike outline's.
	for _, s := range after2 {
		if s.Position == 1 && s.Text != "第二段的内容" {
			t.Fatalf("position 1 was disturbed by a PUT that only mentioned position 0: %+v", s)
		}
	}
}

// TestWritingSnippets_TextIsExactlyWhatWasPosted — no endpoint anywhere
// massages, rewrites, or substitutes model output for what the student
// actually typed into PUT /snippets. This test does not touch the model at
// all (nil provider).
func TestWritingSnippets_TextIsExactlyWhatWasPosted(t *testing.T) {
	h, cookie, _, pool := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于气候变化的议论文")

	studentText := "This is exactly what she typed, verbatim, nothing added."
	body := `{"snippets":[{"position":0,"text":"` + studentText + `"}]}`
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, body); rec.Code != http.StatusOK {
		t.Fatalf("put = %d; body=%s", rec.Code, rec.Body)
	}

	var stored string
	if err := pool.QueryRow(t.Context(),
		`SELECT text FROM writing_snippet WHERE atom_id = $1 AND position = 0`, mustUUID(id)).Scan(&stored); err != nil {
		t.Fatalf("read snippet text: %v", err)
	}
	if stored != studentText {
		t.Fatalf("writing_snippet.text = %q, want exactly %q — the student's text must not be altered", stored, studentText)
	}
}

// TestWritingSnippets_OutlineHeadingSurvivesOutlineResave is Part B of this
// task: outline PUT is a full replace (writing_outline.go) that mints FRESH
// outline row ids on every save, which fires writing_snippet.outline_id's
// ON DELETE SET NULL on every snippet linked to the old outline — revising
// the outline while drafting is the normal thing to do, not an edge case.
// relinkWritingSnippetsToOutline (writing_outline.go) repairs the link at
// WRITE time by matching heading TEXT across the replace; this asserts the
// READ path (GET/PUT /snippets) surfaces that repaired heading — before this
// fix, outlineHeading did not exist on the wire at all and a frontend
// showing "this paragraph belongs to outline point N" would go blank the
// instant she resaved the outline.
func TestWritingSnippets_OutlineHeadingSurvivesOutlineResave(t *testing.T) {
	h, cookie, _, _ := liteHandlerWithProvider(t, nil)
	id := createWritingAtomHTTP(t, h, cookie, "写一篇关于可持续发展的议论文")

	firstOutline := `{"outline":[{"text":"引言：问题的提出","depth":0},{"text":"论点一：碳排放现状","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, firstOutline); rec.Code != http.StatusOK {
		t.Fatalf("first outline PUT = %d; body=%s", rec.Code, rec.Body)
	}
	outline1 := getWritingOutlineHTTP(t, h, cookie, id)
	if len(outline1) != 2 {
		t.Fatalf("outline1 = %+v, want 2 items", outline1)
	}

	// Link both snippets EXPLICITLY by the first outline's real ids.
	putBody := `{"snippets":[` +
		`{"position":0,"text":"第一段草稿","outlineId":"` + outline1[0].ID + `"},` +
		`{"position":1,"text":"第二段草稿","outlineId":"` + outline1[1].ID + `"}` +
		`]}`
	if rec := putWritingSnippetsHTTP(t, h, cookie, id, putBody); rec.Code != http.StatusOK {
		t.Fatalf("put snippets = %d; body=%s", rec.Code, rec.Body)
	}

	before := getWritingSnippetsHTTP(t, h, cookie, id)
	if len(before) != 2 {
		t.Fatalf("snippets before resave = %+v, want 2", before)
	}
	for _, s := range before {
		if s.OutlineID == nil {
			t.Fatalf("snippet %+v: outlineId nil before any outline resave, want it linked", s)
		}
		want := outline1[0].Text
		if s.Position == 1 {
			want = outline1[1].Text
		}
		if s.OutlineHeading != want {
			t.Fatalf("snippet position %d outlineHeading = %q, want %q (before resave)", s.Position, s.OutlineHeading, want)
		}
	}

	// Resave the outline — a FULL REPLACE, same texts at the same positions but
	// brand-new outline row ids. This is what nulls outline_id on both snippets.
	// A genuine RESAVE — identical texts, brand-new row ids — which is what
	// this test's name describes and what actually happens constantly: every
	// outline PUT is a full replace, so ids churn even when she changed
	// nothing. Deliberately does NOT reword: heading text is the only thing
	// tying a paragraph to its point across a replace, so rewording one drops
	// its link on purpose (see matchOutlineRows — a reword and a replacement
	// are the same bytes in a full-replace PUT, and guessing between them
	// produced a wrong heading twice).
	secondOutline := `{"outline":[{"text":"引言：问题的提出","depth":0},{"text":"论点一：碳排放现状","depth":0}]}`
	if rec := putWritingOutlineHTTP(t, h, cookie, id, secondOutline); rec.Code != http.StatusOK {
		t.Fatalf("second outline PUT = %d; body=%s", rec.Code, rec.Body)
	}
	outline2 := getWritingOutlineHTTP(t, h, cookie, id)
	if len(outline2) != 2 || outline2[0].ID == outline1[0].ID {
		t.Fatalf("outline2 = %+v, want 2 FRESH ids distinct from outline1 %+v", outline2, outline1)
	}

	// THE assertion: read the snippets back through the plain GET path and
	// confirm outlineId is re-attached (relinkWritingSnippetsToOutline,
	// writing_outline.go) WHILE outlineHeading still names the (new) heading
	// at that position.
	after := getWritingSnippetsHTTP(t, h, cookie, id)
	if len(after) != 2 {
		t.Fatalf("snippets after resave = %+v, want still 2", after)
	}
	for _, s := range after {
		// outlineId is now NON-null again: the replace nulls it (ON DELETE SET
		// NULL), and relinkWritingSnippetsToOutline (writing_outline.go)
		// immediately re-attaches it to the row that is the same point. This
		// assertion used to require null — that encoded the BROKEN state as
		// expected. The link surviving a resave is the fix, not a violation.
		if s.OutlineID == nil {
			t.Fatalf("snippet position %d outlineId is null after an outline resave, want it re-attached to the same point", s.Position)
		}
		want := outline2[0].Text
		if s.Position == 1 {
			want = outline2[1].Text
		}
		if s.OutlineHeading != want {
			t.Fatalf("snippet position %d outlineHeading = %q after outline resave, want %q — "+
				"she reworded the heading in place, which is the same point better said, so her paragraph keeps it",
				s.Position, s.OutlineHeading, want)
		}
	}

	// PUT's own response must carry the same recovered heading, not just GET —
	// putWritingSnippetsHTTP returns the freshly-upserted rows, and a client
	// that only ever reads PUT's response (never re-GETs) must see it too.
	rePutBody := `{"snippets":[{"position":0,"text":"第一段草稿（略作修改）"}]}`
	rec := putWritingSnippetsHTTP(t, h, cookie, id, rePutBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("re-put position 0 = %d; body=%s", rec.Code, rec.Body)
	}
	var putOut struct {
		Snippets []writingSnippetItem `json:"snippets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &putOut); err != nil {
		t.Fatalf("decode put response: %v — body=%s", err, rec.Body)
	}
	for _, s := range putOut.Snippets {
		if s.Position == 0 && s.OutlineHeading != outline2[0].Text {
			t.Fatalf("PUT response outlineHeading = %q, want %q", s.OutlineHeading, outline2[0].Text)
		}
	}
}
